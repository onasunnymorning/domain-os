package tests

import (
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"gorm.io/gorm"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

func TestController(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Integration Tests Suite")
}

const (
	// make sure the following values are set to match your environment
	dbUser = "postgres"
	dbPass = "unittest"
	dbHost = "127.0.0.1"
	dbPort = "5432"
	dbName = "regos4_integration_tests"
)

func getTestDB() (*gorm.DB, error) {
	return postgres.NewConnection(
		postgres.Config{
			User:   dbUser,
			Pass:   dbPass,
			Host:   dbHost,
			Port:   dbPort,
			DBName: dbName,
		},
	)
}
