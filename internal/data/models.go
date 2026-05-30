package data

import (
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

var (
	RedisDataTTL = 24 * time.Hour  // redis data lifetime
	PgxReqCtxTTL = 3 * time.Second // pgx query request context lifetime
)

type Models struct {
	Movies      MovieModel // interface
	Permissions PermissionModel
	Tokens      TokenModel
	Users       UserModel
}

func NewModels(movieModel MovieModel, db *pgxpool.Pool, rdb *redis.Client) Models {
	return Models{
		Movies: movieModel,
		Permissions: PermissionModel{
			DB:      db,
			Timeout: PgxReqCtxTTL,
		},
		Tokens: TokenModel{
			DB:      db,
			Timeout: PgxReqCtxTTL,
		},
		Users: UserModel{
			DB:      db,
			Timeout: PgxReqCtxTTL,
		},
	}
}
