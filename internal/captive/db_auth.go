package captive

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// DBAuthenticator validates credentials against PostgreSQL (RF06.3).
type DBAuthenticator struct {
	pool *pgxpool.Pool
}

func NewDBAuthenticator(pool *pgxpool.Pool) *DBAuthenticator {
	return &DBAuthenticator{pool: pool}
}

func (d *DBAuthenticator) Authenticate(username, password string) (*UserInfo, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var hash string
	var userID, groupID int

	err := d.pool.QueryRow(ctx, `
		SELECT au.id, au.password, COALESCE(ir.group_id, 1) as group_id
		FROM admin_users au
		LEFT JOIN ip_ranges ir ON ir.id = 1
		WHERE au.username = $1 AND au.active = true
	`, username).Scan(&userID, &hash, &groupID)
	if err != nil {
		fmt.Printf("Captive DB auth: user '%s' not found: %v\n", username, err)
		return nil, false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, false
	}

	return &UserInfo{UserID: userID, GroupID: groupID}, true
}
