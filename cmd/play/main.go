package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	pg "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	dbUser       = "postgres"
	dbPass       = "unittest"
	dbHost       = "127.0.0.1"
	dbPortString = "5432"
	dbPort       = 5432
	dbName       = "dos_unittests"
)

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

func setupTestDB() *gorm.DB {
	gormDB, err := gorm.Open(pg.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPortString + "/" + dbName))
	if err != nil {
		errMsg := err.Error()
		// If the database does not exist, create it and retry
		if errMsg == fmt.Sprintf("failed to connect to `host=%s user=%s database=%s`: server error (FATAL: database \"%s\" does not exist (SQLSTATE 3D000))", dbHost, dbUser, dbName, dbName) {
			log.Println("Database does not exist. Creating...")
			createTestDB()
			gormDB, err := gorm.Open(pg.Open("postgres://" + dbUser + ":" + dbPass + "@" + dbHost + ":" + dbPortString + "/" + dbName))
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

func main() {

	gormDB := setupTestDB()

	// Migrate
	err := postgres.AutoMigrate(gormDB)
	if err != nil {
		log.Println(err)
	}

	rarRepo := postgres.NewGormRegistrarRepository(gormDB)
	tldRepo := postgres.NewGormTLDRepo(gormDB)

	// Create a registrar
	// rar, err := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199, [2]*entities.RegistrarPostalInfo{
	// 	{
	// 		Type: "int",
	// 		Address: &entities.Address{
	// 			Street1:     "123 Main St",
	// 			City:        "Anytown",
	// 			CountryCode: "PE",
	// 		},
	// 	},
	// })
	// if err != nil {
	// 	log.Println(err)
	// }

	// createdRar, err := rarRepo.Create(context.Background(), rar)
	// if err != nil {
	// 	log.Println(err)
	// }

	// Create a TLD
	// tld, err := entities.NewTLD("apex")
	// if err != nil {
	// 	log.Println(err)
	// }

	// err = tldRepo.Create(context.Background(), tld)
	// if err != nil {
	// 	log.Println(err)
	// }

	// Create an accreditation

	// Get the TLD
	tld, err := tldRepo.GetByName(context.Background(), "apex")
	if err != nil {
		log.Println(err)
	}

	// Get the Registrar
	rar, err := rarRepo.GetByClID(context.Background(), "199-myrar", false)
	if err != nil {
		log.Println(err)
	}

	accRepo := postgres.NewAccreditationRepository(gormDB)
	err = accRepo.CreateAccreditation(context.Background(), tld.Name.String(), rar.ClID.String())
	if err != nil {
		log.Println(err)
	}

	// Get the Accreditation by TLD
	rars, err := accRepo.ListTLDRegistrars(context.Background(), 10, "", tld.Name.String())
	if err != nil {
		log.Println(err)
	}

	log.Printf("TLD Registrars: %s", rars[0].ClID)

	// Get the Accreditation by Registrar
	tlds, err := accRepo.ListRegistrarTLDs(context.Background(), 10, "", rar.ClID.String())
	if err != nil {
		log.Println(err)
	}

	log.Printf("Registrar TLDs: %s", tlds[0].Name)

	// Delete the accreditation
	err = accRepo.DeleteAccreditation(context.Background(), tld.Name.String(), rar.ClID.String())
	if err != nil {
		log.Println(err)
	}

	// Get the Accreditation by Registrar
	tlds, err = accRepo.ListRegistrarTLDs(context.Background(), 10, "", rar.ClID.String())
	if err != nil {
		log.Println(err)
	}

	log.Printf("Registrar TLDs: %v", tlds)

}
