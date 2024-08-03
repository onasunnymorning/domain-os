package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"     // Standard postgres driver (in case we need to create the database)
	"gorm.io/driver/postgres" // Gorm postgres driver
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&IANARegistrar{},
		&Spec5Label{},
		&RegistryOperator{},
		&TLD{},
		&Phase{},
		&Price{},
		&Fee{},
		&NNDN{},
		&Registrar{},
		&Contact{},
		&Host{},
		&HostAddress{},
		&Domain{},
		&PremiumList{},
		&PremiumLabel{},
		&FX{},
		&DNSRecord{},
	)
	if err != nil {
		return err
	}

	return nil
}

func CreateDB(dbUser, dbPass, dbHost, dbName, dbPort string) error {
	// Connect to the server
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=require", dbHost, dbPort, dbUser, dbPass)
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

type Config struct {
	User        string
	Pass        string
	Host        string
	Port        string
	DBName      string
	SSLmode     string
	AutoMigrate bool
}

func NewConnection(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLmode)
	gormDB, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, fmt.Sprintf("database \"%s\" does not exist", cfg.DBName)) {
			log.Printf("Database '%s' does not exist. Attempting to create it...", cfg.DBName)
			if err := CreateDB(cfg.User, cfg.Pass, cfg.Host, cfg.DBName, cfg.Port); err != nil {
				log.Println(err)
				return nil, fmt.Errorf("failed to create database: %w", err)
			}
			// Retry the connection after creating the database
			gormDB, err = gorm.Open(postgres.Open(dsn))
			if err != nil {
				// If there is still an issue establishing the connection, return the error
				return nil, fmt.Errorf("failed to connect to database: %w, after creating it", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
	}

	if cfg.AutoMigrate {
		log.Println("Auto migrating database")
		if err = AutoMigrate(gormDB); err != nil {
			return gormDB, fmt.Errorf("failed to migrate database: %w", err)
		}
	} else {
		log.Println("Skipping auto migration")
	}

	return gormDB, nil
}
