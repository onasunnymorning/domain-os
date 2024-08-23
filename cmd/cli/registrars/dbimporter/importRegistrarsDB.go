package main

// This script generates slices of 100 Registrar objects and writes them to database
// it measures the time the insert takes as well as to create the objects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"gorm.io/gorm"
)

const (
	chunkSize = 1000
	total     = 100000
	baseURL   = "http://192.168.64.6:8080"
)

func main() {
	gormDB, err := postgres.NewConnection(
		postgres.Config{
			User:        "postgres",
			Pass:        "postgres",
			Host:        "192.168.1.198",
			Port:        "5432",
			DBName:      "dos_seed",
			SSLmode:     "require",
			AutoMigrate: true,
		},
	)
	if err != nil {
		log.Fatalln(err)
	}
	start := time.Now()

	err = CreatePostGresRegistrarsGORM(gormDB, total, chunkSize)
	if err != nil {
		log.Fatal(err)
	}

	end := time.Now()
	duration := end.Sub(start)
	log.Printf("Time to write %d postgres.Registrar objects to the database in chunks of %d: %v\n", total, chunkSize, duration)

	// Now do the same through the BULK API endpoint
	start = time.Now()

	err = CreateRegistrarsThroughAPI(total, chunkSize)
	if err != nil {
		log.Fatal(err)
	}

	end = time.Now()
	duration = end.Sub(start)
	log.Printf("Time to write %d Registrar objects in through the API in chunks of %d: %v\n", total, chunkSize, duration)

}

// YieldPostGresRegistrars generates a slice of Registrar objects
func YieldPostGresRegistrars(n int) []postgres.Registrar {
	// Create n Registrar objects
	registrars := make([]postgres.Registrar, chunkSize)
	for i := 0; i < chunkSize; i++ {
		clid := CreateRandomClID()
		registrars[i] = postgres.Registrar{
			ClID:        clid,
			Name:        clid,
			GurID:       i,
			NickName:    clid,
			Email:       strconv.Itoa(i) + "@example.com",
			Voice:       "+51.12334" + strconv.Itoa(i),
			RdapBaseUrl: "https://rdap.example.com",
			Whois43:     "whois.example.com",
		}
	}
	return registrars
}

// YieldCreateRegistrarCommands
func YieldCreateRegistrarCommands(n int) []*commands.CreateRegistrarCommand {
	// Create n objects
	cmds := make([]*commands.CreateRegistrarCommand, n)
	for i := 0; i < chunkSize; i++ {
		clid := CreateRandomClID()
		a, _ := entities.NewAddress("Vichayitos", "PE")
		pi, _ := entities.NewRegistrarPostalInfo(entities.RegistrarPostalInfoTypeINT, a)
		cmds[i] = &commands.CreateRegistrarCommand{
			ClID:  clid,
			Name:  clid,
			GurID: i,
			Email: strconv.Itoa(i) + "@example.com",
			Voice: "+51.12334" + strconv.Itoa(i),
			PostalInfo: [2]*entities.RegistrarPostalInfo{
				pi,
			},
			RdapBaseURL: "https://rdap.example.com",
		}
	}
	return cmds
}

// Create pseudo random string of 16 characters
func CreateRandomClID() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// CreatePostGresRegistrarsGORM
func CreatePostGresRegistrarsGORM(gormDB *gorm.DB, total, chunkSize int) error {
	for i := 0; i < total/chunkSize; i++ {
		rars := YieldPostGresRegistrars(chunkSize)
		// Write the Registrar objects to the database - IN BULK
		err := gormDB.Create(&rars).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// createRegistrarsAPI creates registrars in BULK throug one API command
func CreateRegistrarsThroughAPI(total, chuckSize int) error {
	for i := 0; i < total/chunkSize; i++ {
		cmds := YieldCreateRegistrarCommands(chuckSize)
		URL := baseURL + "/registrars-bulk"
		postBody, err := json.Marshal(cmds)
		if err != nil {
			return fmt.Errorf("error marshaling command: %v", err)
		}

		resp, err := http.Post(URL, "application/json", bytes.NewBuffer(postBody))
		if err != nil {
			log.Fatalln("[ERR] error send create command to API")
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %v", err)
		}

		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("error creating registrars in bulk through API: %s", string(body))
		}
	}
	return nil
}
