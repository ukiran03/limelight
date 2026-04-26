package data

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int64
	CreateAt  time.Time
	Name      string
	Email     string
	Password  password
	Activated bool
	Version   int
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
		&user.ID, &user.CreateAt, &user.Version,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		switch {
		case errors.As(err, pgErr):
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
		&user.ID, &user.CreateAt, &user.Name, &user.Email,
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
		case errors.As(err, pgErr):
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
