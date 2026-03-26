package store

import (
	"embed"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// AutoMigrate runs all pending database migrations automatically (RNF07.4).
// Called at startup before any other database operations.
func AutoMigrate(databaseURL string) error {
	// Create a sub-filesystem pointing to the migrations directory
	subFS, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: sub fs: %w", err)
	}

	source, err := iofs.New(subFS, ".")
	if err != nil {
		return fmt.Errorf("migrate: create source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	defer m.Close()

	err = m.Up()
	if err == migrate.ErrNoChange {
		fmt.Println("Migrations: banco já está atualizado")
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate: up: %w", err)
	}

	version, dirty, _ := m.Version()
	fmt.Printf("Migrations: aplicadas com sucesso (versão %d, dirty=%v)\n", version, dirty)
	return nil
}
