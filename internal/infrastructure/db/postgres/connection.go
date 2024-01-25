package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"

	_ "github.com/lib/pq"     // Standard postgres driver (in case we need to create the database)
	"gorm.io/driver/postgres" // Gorm postgres driver
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&TLD{},
		&NNDN{},
	)
	if err != nil {
		return err
	}

	return nil
}

func CreateDB(dbUser, dbPass, dbHost, dbName, dbPort string) error {
	port, _ := strconv.Atoi(dbPort)
	// Connect to the server
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=disable", dbHost, port, dbUser, dbPass)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create the database
	createDatabaseCommand := fmt.Sprintf("CREATE DATABASE %s", dbName)
	_, err = db.Exec(createDatabaseCommand)
	if err != nil {
		return err
	}

	return nil
}

func NewConnection() (*gorm.DB, error) {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	gormDB, err := gorm.Open(postgres.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName))
	if err != nil {
		errMsg := err.Error()
		// If the database does not exist, create it and retry
		if errMsg == fmt.Sprintf("failed to connect to `host=%s user=%s database=%s`: server error (FATAL: database \"%s\" does not exist (SQLSTATE 3D000))", dbHost, dbUser, dbName, dbName) {
			log.Printf("Database '%s' does not exist. Attempting to create it...", dbName)
			err = CreateDB(dbUser, dbPass, dbHost, dbName, dbPort)
			if err != nil {
				return nil, err
			}
			// If the create works, retry the connection
			gormDB, err := gorm.Open(postgres.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPort + "/" + dbName))
			if err != nil {
				// If there is still an issue establishing the connection, return the error
				return nil, err
			}
			return gormDB, nil
		} else {
			// If the error is not that the database does not exist, return the error
			return nil, err
		}
	}
	err = AutoMigrate(gormDB)
	if err != nil {
		return gormDB, err
	}

	return gormDB, nil
}
