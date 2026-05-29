package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	nfTTL = 5 * time.Minute // shorter TTL than c.TTL for non-existent records
	nfVal = "NF"            // not found sentinel
)

type CachedMovieModel struct {
	M          MovieModel
	Redis      *redis.Client
	TTL        time.Duration // redis data lifetime
	cacheQueue chan cacheJob
	wg         sync.WaitGroup
}

func NewCachedMovieModel(
	ctx context.Context,
	db *pgxpool.Pool, rdb *redis.Client,
) *CachedMovieModel {
	c := &CachedMovieModel{
		M: MovieModel{
			DB:      db,
			Timeout: PgxReqCtxTTL,
		},
		Redis:      rdb,
		TTL:        RedisDataTTL,
		cacheQueue: make(chan cacheJob, 100),
	}

	c.StartCacheWorkers(10)

	go func() {
		<-ctx.Done() // Block until main context is cancelled
		c.StopCacheWorkers()
	}()

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
			defer c.wg.Done()
			for job := range c.cacheQueue {
				// with reasonable timeout for redis writes
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				c.Redis.Set(ctx, job.key, job.data, c.TTL)
				cancel()
			}
		}()
	}
}

// For main.go shutdown logic:
func (c *CachedMovieModel) StopCacheWorkers() {
	close(c.cacheQueue)
	c.wg.Wait()
}

func (c *CachedMovieModel) Get(ctx context.Context, id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	cacheKey := fmt.Sprintf("movie:%d", id)

	val, err := c.Redis.Get(ctx, cacheKey).Bytes()
	// cache hit!
	if err == nil {
		// Check if it's the "Not Found" sentinel, preventing Cache Penetration
		if string(val) == nfVal { //
			return nil, ErrRecordNotFound
		}

		var movie Movie
		if err := json.Unmarshal(val, &movie); err == nil {
			return &movie, nil
		}
	}

	// cache miss!, fallback to DB
	movie, err := c.M.Get(ctx, id) // which returns (*Movie, error)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			// Cache the "Not Found" result for 5 minutes, using a shorter TTL
			// than c.TTL for non-existent records.
			c.Redis.Set(ctx, cacheKey, nfVal, nfTTL)
			return nil, err
		}
		return nil, err
	}

	if movie != nil {
		// Worker Pool: Save to redis cache
		if jsonData, err := json.Marshal(movie); err == nil {
			select {
			case c.cacheQueue <- cacheJob{key: cacheKey, data: jsonData}:
				// job sent successfully
			default:
				// Queue is full!, drop the cache update
			}
		}
	}

	return movie, nil
}

func (c *CachedMovieModel) Insert(ctx context.Context, movie *Movie) error {
	return c.M.Insert(ctx, movie)
}

func (c *CachedMovieModel) Update(ctx context.Context, movie *Movie) error {
	if err := c.M.Update(ctx, movie); err != nil {
		return err
	}
	// proceed only if database transaction succeeds.
	cacheKey := fmt.Sprintf("movie:%d", movie.ID)
	return c.Redis.Del(ctx, cacheKey).Err()
}

func (c *CachedMovieModel) Delete(ctx context.Context, id int64) error {
	if err := c.M.Delete(ctx, id); err != nil {
		return err
	}
	cacheKey := fmt.Sprintf("movie:%d", id)
	return c.Redis.Del(ctx, cacheKey).Err()
}

// [29-05-2026] TODO: skipping the GetAll() cache for now, as its to fragile
func (c *CachedMovieModel) GetAll(
	ctx context.Context, title string, genres []string, filters Filters,
) ([]*Movie, Metadata, error) {
	return c.M.GetAll(ctx, title, genres, filters)
}

// NOTE: look into golang.org/x/sync/singleflight to ensure only one DB
// call happens per ID, in CachedMovieModel.Get()
