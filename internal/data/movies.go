package data

import (
	"context"
	"time"

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

type MovieModel interface {
	Insert(ctx context.Context, movie *Movie) error
	Get(ctx context.Context, id int64) (*Movie, error)
	Update(ctx context.Context, movie *Movie) error
	Delete(ctx context.Context, id int64) error
	GetAll(
		ctx context.Context, title string,
		genres []string, filters Filters,
	) ([]*Movie, Metadata, error)
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
