package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type DomainSuite struct {
	suite.Suite
	db      *gorm.DB
	rarClid string
}

func TestDomainSuite(t *testing.T) {
	suite.Run(t, new(DomainSuite))
}

func (s *DomainSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, _ := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199)
	repo := NewGormRegistrarRepository(s.db)
	createdRar, _ := repo.Create(context.Background(), rar)
	s.rarClid = createdRar.ClID.String()
}

func (s *DomainSuite) TearDownSuite() {
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
}
