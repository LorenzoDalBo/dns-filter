package store

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type AdminUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
	Active   bool   `json:"active"`
}

// AuthenticateAdmin validates credentials and returns the user (RF09.1).
func (s *Store) AuthenticateAdmin(ctx context.Context, username, password string) (*AdminUser, error) {
	var user AdminUser
	var hash string

	err := s.pool.QueryRow(ctx, `
		SELECT id, username, password, role, active
		FROM admin_users WHERE username = $1 AND active = true
	`, username).Scan(&user.ID, &user.Username, &hash, &user.Role, &user.Active)
	if err != nil {
		return nil, fmt.Errorf("store: auth: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("store: invalid password")
	}

	return &user, nil
}

// CreateAdminUser creates a new dashboard user with hashed password (RF09.5).
func (s *Store) CreateAdminUser(ctx context.Context, username, password string, role int) (int, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, fmt.Errorf("store: hash password: %w", err)
	}

	var id int
	err = s.pool.QueryRow(ctx, `
		INSERT INTO admin_users (username, password, role)
		VALUES ($1, $2, $3) RETURNING id
	`, username, string(hash), role).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store: create user: %w", err)
	}

	return id, nil
}

// ListAdminUsers returns all dashboard users (without passwords).
func (s *Store) ListAdminUsers(ctx context.Context) ([]AdminUser, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, username, role, active FROM admin_users ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("store: list users: %w", err)
	}
	defer rows.Close()

	var users []AdminUser
	for rows.Next() {
		var u AdminUser
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Active); err != nil {
			return nil, fmt.Errorf("store: scan user: %w", err)
		}
		users = append(users, u)
	}

	return users, nil
}
