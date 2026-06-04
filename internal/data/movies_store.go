package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StoreMovieModel struct {
	DB     Querier
	TTL    time.Duration
	logger *slog.Logger // just in case
}

func NewStoreMovieModel(db *pgxpool.Pool, logger *slog.Logger) *StoreMovieModel {
	return &StoreMovieModel{
		DB:     db,
		TTL:    PgxReqCtxTTL,
		logger: logger,
	}
}

func (s *StoreMovieModel) Insert(ctx context.Context, movie *Movie) error {
	query := `
		INSERT INTO movies (title, year, runtime, genres)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`

	args := []any{
		movie.Title, movie.Year,
		movie.Runtime, movie.Genres,
	}

	ctx, cancel := context.WithTimeout(ctx, s.TTL)
	defer cancel()

	err := s.DB.QueryRow(ctx, query, args...).Scan(
		&movie.ID, &movie.CreatedAt, &movie.Version,
	)

	return err
}

func (s *StoreMovieModel) Get(ctx context.Context, id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT ID, created_at, title, YEAR, runtime,
              genres, version  FROM movies WHERE ID = $1`

	var movie Movie

	ctx, cancel := context.WithTimeout(ctx, s.TTL)
	defer cancel()

	err := s.DB.QueryRow(ctx, query, id).Scan(
		&movie.ID, &movie.CreatedAt, &movie.Title,
		&movie.Year, &movie.Runtime, &movie.Genres,
		&movie.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &movie, nil
}

// [24-04-2026] TODO: impl errors.Is(err, context.DeadlineExceeded) in the
// error handling.

func (s *StoreMovieModel) Update(ctx context.Context, movie *Movie) error {
	query := `UPDATE movies
              SET title = $1, year = $2, runtime = $3,
                  genres = $4, version = version + 1
              WHERE id = $5 AND version = $6
              RETURNING version`

	args := []any{
		movie.Title, movie.Year,
		movie.Runtime, movie.Genres,
		movie.ID, movie.Version,
	}

	ctx, cancel := context.WithTimeout(ctx, s.TTL)
	defer cancel()

	err := s.DB.QueryRow(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (s *StoreMovieModel) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `DELETE FROM movies WHERE id = $1`

	ctx, cancel := context.WithTimeout(ctx, s.TTL)
	defer cancel()

	result, err := s.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (s *StoreMovieModel) GetAll(
	ctx context.Context, title string, genres []string, filters Filters,
) ([]*Movie, Metadata, error) {
	query := fmt.Sprintf(`
        SELECT count(*) OVER(), id, created_at, title, year,
               runtime, genres, version
        FROM movies
        WHERE (to_tsvector('simple', title)
               @@ plainto_tsquery('simple', $1) OR $1 = '')
        AND (genres @> $2 OR $2 = '{}')
        ORDER BY %s %s, id ASC
        LIMIT $3 OFFSET $4`,
		filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(ctx, s.TTL)
	defer cancel()

	args := []any{title, genres, filters.limit(), filters.offset()}

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	totalRecords := 0
	movies := []*Movie{}

	for rows.Next() {
		var movie Movie
		err := rows.Scan(
			&totalRecords, &movie.ID, &movie.CreatedAt, &movie.Title,
			&movie.Year, &movie.Runtime, &movie.Genres, &movie.Version,
		)
		if err != nil {
			return nil, Metadata{}, err
		}
		movies = append(movies, &movie)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(
		totalRecords, filters.Page, filters.PageSize)

	return movies, metadata, nil
}
