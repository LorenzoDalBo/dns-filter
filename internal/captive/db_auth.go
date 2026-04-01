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
	var userID int

	err := d.pool.QueryRow(ctx, `
		SELECT id, password FROM admin_users
		WHERE username = $1 AND active = true
	`, username).Scan(&userID, &hash)
	if err != nil {
		fmt.Printf("Captive DB auth: user '%s' not found: %v\n", username, err)
		return nil, false
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, false
	}

	// GroupID will be set by the captive portal from the IP range
	return &UserInfo{UserID: userID, GroupID: 0}, true
}
