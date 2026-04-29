package data

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64     `json:"id"         db:"id"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	Name      string    `json:"name"       db:"name"`
	Email     string    `json:"email"      db:"email"`
	Password  password  `json:"-"          db:"password"`
	Activated bool      `json:"activated"  db:"activated"`
	Version   int       `json:"-"          db:"version"`
}

type password struct {
	plaintext *string
	hash      []byte
}

func (p *password) Set(plaintextPwd string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPwd), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plaintextPwd
	p.hash = hash
	return nil
}

func (p *password) Matches(plaintextPwd string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPwd))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}
	return true, nil
}

// https://www.postgresql.org/docs/current/errcodes-appendix.html
const pgErrCodeUniqueViolation = "23505"

var ErrDuplicateEmail = errors.New("duplicate email")

type UserModel struct {
	DB      *pgxpool.Pool
	Timeout time.Duration
}

func (m UserModel) Insert(ctx context.Context, user *User) error {
	query := `INSERT INTO users (name, email, password_hash, activated)
              VALUES ($1, $2, $3, $4)
              RETURNING id, created_at, version`

	args := []any{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.CreatedAt, &user.Version,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.As(err, &pgErr):
			if pgErr.Code == pgErrCodeUniqueViolation {
				return ErrDuplicateEmail
			}
		default:
			return err
		}
	}
	return nil
}

func (m UserModel) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT id, created_at, NAME, email,
                    password_hash, activated, version
             FROM users
             WHERE email = $1`
	var user User

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.CreatedAt, &user.Name, &user.Email,
		&user.Password.hash, &user.Activated, &user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

func (m UserModel) Update(ctx context.Context, user *User) error {
	query := `UPDATE users
              SET name = $1, email = $2, password_hash = $3,
                  activated = $4, version = version + 1,
              WHERE id = $5 AND version = $6
              RETURNING version`
	args := []any{
		user.Name, user.Email, user.Password.hash,
		user.Activated, user.ID, user.Version,
	}

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.As(err, &pgErr):
			if pgErr.Code == pgErrCodeUniqueViolation {
				return ErrDuplicateEmail
			}
		case errors.Is(err, pgx.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}
	return nil
}

func (m UserModel) GetForToken(
	ctx context.Context, tokenScope, tokenPlaintext string,
) (*User, error) {
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	query := `SELECT users.id, users.created_at, users.name,
                     users.email, users.password_hash, users.activated,
                     users.version
              FROM users
              INNER JOIN tokens ON users.id = tokens.user_id
              WHERE tokens.hash = $1
                    AND tokens.scope = $2
                    AND tokens.expiry > $3`

	// we pass the current time as the value to check against the token expiry.
	args := []any{tokenHash[:], tokenScope, time.Now()}
	var user User

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	err := m.DB.QueryRow(ctx, query, args...).Scan(
		&user.ID, &user.CreatedAt, &user.Name, &user.Email,
		&user.Password.hash, &user.Activated, &user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}
