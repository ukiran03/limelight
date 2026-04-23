package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ukiran.com/limelight/internal/validator"
)

type Movie struct {
	ID        int64     `json:"id"               db:"id"`
	CreatedAt time.Time `json:"-"                db:"created_at"`
	Title     string    `json:"title"            db:"title"`
	Year      int32     `json:"year,omitzero"    db:"year"`
	Runtime   Runtime   `json:"runtime,omitzero" db:"runtime"`
	Genres    []string  `json:"genres,omitzero"  db:"genres"`
	Version   int32     `json:"version"          db:"version"`
}

type MovieModel struct {
	DB *pgxpool.Pool
}

func (m MovieModel) Insert(ctx context.Context, movie *Movie) error {
	query := `
		INSERT INTO movies (title, year, runtime, genres)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`

	args := []any{
		movie.Title, movie.Year,
		movie.Runtime, movie.Genres,
	}

	err := m.DB.QueryRow(ctx, query, args...).Scan(
		&movie.ID, &movie.CreatedAt, &movie.Version,
	)

	return err
}

func (m MovieModel) Get(ctx context.Context, id int64) (*Movie, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	query := `SELECT ID, created_at, title, YEAR, runtime,
              genres, version  FROM movies WHERE ID = $1`

	var movie Movie
	err := m.DB.QueryRow(ctx, query, id).Scan(
		&movie.ID, &movie.CreatedAt, &movie.Title,
		&movie.Year, &movie.Runtime, &movie.Genres,
		&movie.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &movie, nil
}

func (m MovieModel) Update(ctx context.Context, movie *Movie) error {
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

	err := m.DB.QueryRow(ctx, query, args...).Scan(&movie.Version)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m MovieModel) Delete(ctx context.Context, id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}

	query := `DELETE FROM movies WHERE id = $1`
	result, err := m.DB.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func ValidateMovie(v *validator.Validator, movie *Movie) {
	v.Check(movie.Title != "", "title", "must be provided")
	v.Check(len(movie.Title) <= 500, "title",
		"must not be more than 500 bytes long")
	v.Check(movie.Year != 0, "year", "must be provided")
	v.Check(movie.Year >= 1888, "year", "must be greater than 1888")
	v.Check(movie.Year <= int32(time.Now().Year()), "year",
		"must not be in the future")
	v.Check(movie.Runtime != 0, "runtime", "must be provided")
	v.Check(movie.Runtime > 0, "runtime", "must be a positive integer")
	v.Check(movie.Genres != nil, "genres", "must be provided")
	v.Check(len(movie.Genres) >= 1, "genres", "must contain at least 1 genre")
	v.Check(len(movie.Genres) <= 5, "genres",
		"must not contain more than 5 genres")
	v.Check(validator.Unique(movie.Genres), "genres",
		"must not contain duplicate values")
}
