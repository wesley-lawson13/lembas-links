package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/wesley-lawson13/lembas-links/config"
)

func NewPool(cfg *config.Config) *sql.DB {

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to open connection: %v\n", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// connection retry ping
	for i := range 10 {

		// connection succeeded
		if err := db.Ping(); err == nil {
			log.Println("Successfully connected to Postgres.")
			return db
		} else {
			log.Printf("Attempt %d/10 failed: %v\n", i+1, err)
		}
		time.Sleep(2 * time.Second)
	}

	log.Fatal("Could not connect to database.")
	return nil
}
