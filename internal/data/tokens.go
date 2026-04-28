package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"ukiran.com/limelight/internal/validator"
)

const ScopeActivation = "activation"

type Token struct {
	Plaintext string
	Hash      []byte
	UserID    int64
	Expiry    time.Time
	Scope     string
}

func generateToken(userID int64, ttl time.Duration, scope string) *Token {
	token := &Token{
		Plaintext: rand.Text(),
		UserID:    userID,
		Expiry:    time.Now().Add(ttl),
		Scope:     scope,
	}

	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token
}

func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

type TokenModel struct {
	DB      *pgxpool.Pool
	Timeout time.Duration
}

// New method is a shortcut which creates a new Token struct and then inserts
// the data in the tokens table.
func (m TokenModel) New(
	ctx context.Context, userID int64, ttl time.Duration, scope string) (
	*Token, error,
) {
	token := generateToken(userID, ttl, scope)

	err := m.Insert(context.Background(), token)
	return token, err
}

// Insert adds the data for a specific token to the tokens table.
func (m TokenModel) Insert(ctx context.Context, token *Token) error {
	query := `INSERT INTO tokens (hash, user_id, expiry, scope)
              VALUES ($1, $2, $3, $4)`

	args := []any{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, args...)
	return err
}

// DeleteAllForUser deletes all tokens for a specific user and scope
func (m TokenModel) DeleteAllForUser(
	ctx context.Context, scope string, userID int64,
) error {
	query := `DELETE FROM tokens WHERE scope = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	_, err := m.DB.Exec(ctx, query, scope, userID)
	return err
}
