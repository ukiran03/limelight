package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	nfTTL = 5 * time.Minute // shorter TTL than c.TTL for non-existent records
	nfVal = "NF"            // not found sentinel
)

type CachedMovieModel struct {
	Store      MovieModel // interface
	Redis      *redis.Client
	TTL        time.Duration // redis data lifetime
	cacheQueue chan cacheJob
	wg         sync.WaitGroup
	logger     *slog.Logger
}

func NewCachedMovieModel(
	movieModel MovieModel,
	rdb *redis.Client,
	logger *slog.Logger,
) *CachedMovieModel {
	c := &CachedMovieModel{
		Store:      movieModel, // store movie model, a concrete type
		Redis:      rdb,
		TTL:        RedisDataTTL,
		cacheQueue: make(chan cacheJob, 100),
		logger:     logger,
	}
	return c
}

type cacheJob struct {
	key  string
	data []byte
}

// start at startup
func (c *CachedMovieModel) StartCacheWorkers(workers int) {
	for range workers {
		c.wg.Add(1)
		go func() {
			// LIFO order. wg.Done() should be declared LAST so it runs LAST.
			defer c.wg.Done()

			// Panic recovery should always wrap the worker's core logic safely
			defer func() {
				if pv := recover(); pv != nil {
					c.logger.Info("redis worker panic recovered", "pv", pv)
				}
			}()

			for job := range c.cacheQueue {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				c.Redis.Set(ctx, job.key, job.data, c.TTL)
				cancel()
			}
		}()
	}
}

// StopCacheWorkers closes the queue channel and waits for workers to drain it.
func (c *CachedMovieModel) StopCacheWorkers() {
	close(c.cacheQueue)
	c.wg.Wait()
}

func (c *CachedMovieModel) Get(ctx context.Context, id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	cacheKey := fmt.Sprintf("movie:%d", id)

	// Get from redis
	val, err := c.Redis.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit!, check if it's nor explicit "Not Found" sentinel
		if val == nfVal {
			return nil, ErrRecordNotFound
		}
		var movie Movie
		if err := json.Unmarshal([]byte(val), &movie); err == nil {
			return &movie, nil
		}
		c.logger.Error("failed to unmarshal cached movie", "error", err)
	} else if !errors.Is(err, redis.Nil) {
		// Log real Redis infrastructure errors, but let the code fall through
		// to DB
		c.logger.Error("critical redis error", "error", err)
	}

	// Cache Miss / Failure -> Fallback to DB
	movie, err := c.Store.Get(ctx, id)
	if err != nil {
		// If the record genuinely doesn't exist, cache the sentinel to protect the DB
		if errors.Is(err, ErrRecordNotFound) {
			_ = c.Redis.Set(ctx, cacheKey, nfVal, nfTTL).Err()
		}
		return nil, err
	}

	// Queue the background cache write for successful records
	if movie != nil {
		if jsonData, err := json.Marshal(movie); err == nil {
			select {
			case c.cacheQueue <- cacheJob{key: cacheKey, data: jsonData}:
			default:
				c.logger.Warn("cache queue full, dropping update", "key", cacheKey)
			}
		}
	}

	return movie, nil
}

func (c *CachedMovieModel) Insert(ctx context.Context, movie *Movie) error {
	return c.Store.Insert(ctx, movie)
}

func (c *CachedMovieModel) Update(ctx context.Context, movie *Movie) error {
	if err := c.Store.Update(ctx, movie); err != nil {
		return err
	}
	// proceed only if database transaction succeeds.
	cacheKey := fmt.Sprintf("movie:%d", movie.ID)
	return c.Redis.Del(ctx, cacheKey).Err()
}

func (c *CachedMovieModel) Delete(ctx context.Context, id int64) error {
	if err := c.Store.Delete(ctx, id); err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("movie:%d", id)
	return c.Redis.Del(ctx, cacheKey).Err()
}

// [29-05-2026] TODO: skipping the GetAll() cache for now, as its to fragile
func (c *CachedMovieModel) GetAll(
	ctx context.Context, title string, genres []string, filters Filters,
) ([]*Movie, Metadata, error) {
	return c.Store.GetAll(ctx, title, genres, filters)
}

// NOTE: look into golang.org/x/sync/singleflight to ensure only one DB
// call happens per ID, in CachedMovieModel.Get()
