package data

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Movies MovieModel
	Tokens TokenModel
	Users  UserModel
}

func NewModels(db *pgxpool.Pool) Models {
	return Models{
		Movies: MovieModel{
			DB:      db,
			Timeout: 3 * time.Second,
		},
		Tokens: TokenModel{
			DB:      db,
			Timeout: 3 * time.Second,
		},
		Users: UserModel{
			DB:      db,
			Timeout: 3 * time.Second,
		},
	}
}
