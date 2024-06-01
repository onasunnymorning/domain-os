package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	cfg := struct {
		User   string
		Pass   string
		Host   string
		Port   string
		DBName string
	}{
		User:   "postgres",
		Pass:   "VrZVfF949JL$qV",
		Host:   "free-tier-db.aws.apexdomains.net",
		Port:   "5432",
		DBName: "dos-dev01",
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.DBName)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open the database: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatalf("failed to connect to the database: %v", err)
	}

	fmt.Println("Successfully connected to the database!")
}
