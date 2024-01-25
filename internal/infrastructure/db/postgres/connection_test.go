package postgres

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

const (
	dbUser       = "postgres"
	dbPass       = "unittest"
	dbHost       = "127.0.0.1"
	dbPortString = "5432"
	dbPort       = 5432
	dbName       = "regos4_unittests"
)

func setupTestDB() *gorm.DB {
	gormDB, err := gorm.Open(postgres.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPortString + "/" + dbName))
	if err != nil {
		errMsg := err.Error()
		// If the database does not exist, create it and retry
		if errMsg == fmt.Sprintf("failed to connect to `host=%s user=%s database=%s`: server error (FATAL: database \"%s\" does not exist (SQLSTATE 3D000))", dbHost, dbUser, dbName, dbName) {
			log.Println("Database does not exist. Creating...")
			createTestDB()
			gormDB, err := gorm.Open(postgres.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPortString + "/" + dbName))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf("Connected to Database %s", dbName)
			return gormDB
		} else {
			// Otherwise, log the error and exit
			log.Fatal(err)
		}
	}
	log.Printf("Connected to Database %s", dbName)
	return gormDB
}

func createTestDB() {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=disable", dbHost, dbPort, dbUser, dbPass)
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
	gormDB := setupTestDB()
	err := AutoMigrate(gormDB)
	if err != nil {
		log.Fatalf("failed to migrated DB, error:%s", err)
	}
	log.Printf("Migrated Database %s", dbName)
	return gormDB
}
