package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	cfg := struct {
		User    string
		Pass    string
		Host    string
		Port    string
		DBName  string
		SSLmode string
	}{
		User:    "postgres",
		Pass:    "unittest",
		Host:    "localhost",
		Port:    "5432",
		DBName:  "playdb",
		SSLmode: "require",
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLmode)
	_, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		log.Printf("failed to open the database: %v", err)
		log.Println("Creating the database...")

		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=require", cfg.Host, cfg.Port, cfg.User, cfg.Pass)

		db, err := sql.Open("postgres", dsn)
		if err != nil {
			log.Fatalf("failed to connect to server: %v", err)

		}
		defer db.Close()

		createDatabaseCommand := fmt.Sprintf("CREATE DATABASE %s", cfg.DBName)
		_, err = db.Exec(createDatabaseCommand)
		if err != nil {
			log.Fatalf("failed to create the database: %v", err)
		}

		fmt.Println("Successfully created to the database!")

	}

	fmt.Println("Successfully connected to the database!")
}
