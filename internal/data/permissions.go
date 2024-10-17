package data

import (
	"context"
	"database/sql"
	"github.com/lib/pq"
	"time"
)

// 定义一个权限切片来保存获取到的权限
type Permissions []string

// 检查权限切片中是否包含某个具体的权限
func (p Permissions) Include(code string) bool {
	for i := range p {
		if code == p[i] {
			return true
		}
	}
	return false
}

type PermissionModel struct {
	DB *sql.DB
}

// 通过某个具体的userID得到其所有权限
func (m PermissionModel) GetAllForUser(userID int64) (Permissions, error) {
	query := `
			SELECT permissions.code
			FROM permissions
			INNER JOIN users_permissions ON users_permission.permission_id=permissions.id
			INNER JOIN users ON users_permissions.user_id = users.id
			WHERE users.id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
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

// 为某个具体userID添加指定的权限
func (m PermissionModel) AddForUser(userID int64, codes ...string) error {
	query := `
			INSERT INTO users_permissions
			SELECT $1, permissions.id FROM permissions WHERE permissions.code = ANY($2)`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, userID, pq.Array(codes))
	return err
}
