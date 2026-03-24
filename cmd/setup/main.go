package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/LorenzoDalBo/dns-filter/internal/store"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable"
	}

	ctx := context.Background()
	db, err := store.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	id, err := db.CreateAdminUser(ctx, "admin", "admin123", 0)
	if err != nil {
		log.Fatalf("Failed to create admin: %v", err)
	}

	fmt.Printf("Admin user created with ID %d (password: admin123, role: admin)\n", id)
}
