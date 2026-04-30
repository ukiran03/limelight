package data

import (
	"context"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Permissions []string

func (p Permissions) Include(code string) bool {
	return slices.Contains(p, code)
}

type PermissionModel struct {
	DB      *pgxpool.Pool
	Timeout time.Duration
}

func (m PermissionModel) GetAllForUser(ctx context.Context, userID int64) (Permissions, error) {
	query := `SELECT permissions.code
              FROM permissions
              INNER JOIN users_permissions
                    ON users_permissions.permission_id = permissions.id
              INNER JOIN users ON users_permissions.user_id = users.id
              WHERE users.id = $1`

	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()

	rows, err := m.DB.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var permissions Permissions

	for rows.Next() {
		var permission string
		err := rows.Scan(&permission)
		if err != nil {
			return nil, err
		}
		permissions = append(permissions, permission)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return permissions, nil
}
