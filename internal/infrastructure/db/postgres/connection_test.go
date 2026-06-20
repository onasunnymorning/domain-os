package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	dbUser       = "postgres"
	dbPass       = "unittest"
	dbHost       = "127.0.0.1"
	dbPortString = "5432"
	dbPort       = 5432
	dbName       = "dos_unittests"
	sslmode      = "require"
)

func setupTestDB() *gorm.DB {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", dbUser, dbPass, dbHost, dbPortString, dbName, sslmode)
	gormDB, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		// If the database does not exist, create it and retry
		if strings.Contains(err.Error(), "3D000") {
			log.Println("Database does not exist. Creating...")
			createTestDB()
			gormDB, err := gorm.Open(postgres.Open(dsn))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Connected to Database %s", dbName)
			return gormDB
		}
		// Otherwise, log the error and exit
		log.Fatal(err)
	}
	log.Printf("Connected to Database %s", dbName)
	return gormDB
}

func createTestDB() {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=require", dbHost, dbPort, dbUser, dbPass)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// createDatabaseCommand := fmt.Sprintf("SELECT 'CREATE DATABASE %s' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '%s')", os.Getenv("DB_NAME"), os.Getenv("DB_NAME"))
	createDatabaseCommand := fmt.Sprintf("CREATE DATABASE %s", dbName)
	_, err = db.Exec(createDatabaseCommand)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Database created: %s", dbName)
}

func getTestDB() *gorm.DB {
	return testDB
}

var testDB *gorm.DB

func TestMain(m *testing.M) {
	// Setup database connection
	gormDB := setupTestDB()
	err := AutoMigrate(gormDB)
	if err != nil {
		log.Fatalf("failed to migrated DB, error:%s", err)
	}
	log.Printf("Migrated Database %s", dbName)
	testDB = gormDB
	// Run tests
	code := m.Run()
	// Close database connection
	os.Exit(code)
}
