package migrate

import (
	"database/sql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"log"
	"os"
)

func RunMigrations(pool *sql.DB) {
	driver, err := postgres.WithInstance(pool, &postgres.Config{})
	if err != nil {
		log.Fatalf("Failed to create migrate driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file:///db/migrations",
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatalf("Failed to create migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	log.Println("Migrations ran successfully")
}

func SeedQuotesIfEmpty(pool *sql.DB) {
	var count int
	if err := pool.QueryRow("SELECT COUNT(*) FROM quotes").Scan(&count); err != nil {
		log.Printf("Seed check failed: %v", err)
		return
	}
	if count > 0 {
		log.Println("Quotes table already seeded, skipping")
		return
	}

	sqlBytes, err := os.ReadFile("/db/seeds/quotes.sql")
	if err != nil {
		log.Printf("Failed to read quotes seed file: %v", err)
		return
	}
	if _, err := pool.Exec(string(sqlBytes)); err != nil {
		log.Printf("Failed to seed quotes: %v", err)
		return
	}
	log.Println("Quotes table seeded successfully")
}
