package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type RegistrarSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestRegistrarSuite(t *testing.T) {
	suite.Run(t, new(RegistrarSuite))
}

func (s *RegistrarSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)
}

func (s *RegistrarSuite) TestCreateRegistrar() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)
}

func (s *RegistrarSuite) TestCreateRegistrar_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)

	// Create a duplicate
	createdRegistrar, err = repo.Create(context.Background(), registrar)
	require.Error(s.T(), err)
	require.Nil(s.T(), createdRegistrar)
}

func (s *RegistrarSuite) TestReadRegistrar() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)

	readRegistrar, err := repo.GetByClID(context.Background(), registrar.ClID.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readRegistrar)
	require.Equal(s.T(), createdRegistrar, readRegistrar)

	readRegistrar, err = repo.GetByGurID(context.Background(), registrar.GurID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readRegistrar)
	require.Equal(s.T(), createdRegistrar, readRegistrar)
}

func (s *RegistrarSuite) TestUpdateRegistrar() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)

	createdRegistrar.Name = "Updated Registrar Name"
	updatedRegistrar, err := repo.Update(context.Background(), createdRegistrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), updatedRegistrar)
	require.Equal(s.T(), "Updated Registrar Name", updatedRegistrar.Name)
}

func (s *RegistrarSuite) TestDeleteRegistrar() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)

	err = repo.Delete(context.Background(), createdRegistrar.ClID.String())
	require.NoError(s.T(), err)

	_, err = repo.GetByClID(context.Background(), createdRegistrar.ClID.String())
	require.Error(s.T(), err)

	err = repo.Delete(context.Background(), createdRegistrar.ClID.String())
	require.NoError(s.T(), err)

	_, err = repo.GetByClID(context.Background(), createdRegistrar.ClID.String())
	require.Error(s.T(), err)
}

func (s *RegistrarSuite) TestListRegistrars() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar1, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar1, err := repo.Create(context.Background(), registrar1)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar1)

	registrar2, _ := entities.NewRegistrar("my-registrar-id2", "GoBro Inc.",
		"contact@gobro.com", 12346, getValidRegistrarPostalInfoArr())
	createdRegistrar2, err := repo.Create(context.Background(), registrar2)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar2)

	registrars, err := repo.List(context.Background(), 2, "")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrars)
	require.Len(s.T(), registrars, 2)
}

func getValidRegistrarPostalInfo(t string) *entities.RegistrarPostalInfo {
	a, err := entities.NewAddress("BA", "AR")
	if err != nil {
		panic(err)
	}
	p, err := entities.NewRegistrarPostalInfo(t, a)
	if err != nil {
		panic(err)
	}
	return p
}
func getValidRegistrarPostalInfoArr() [2]*entities.RegistrarPostalInfo {
	return [2]*entities.RegistrarPostalInfo{
		getValidRegistrarPostalInfo("loc"),
		getValidRegistrarPostalInfo("int"),
	}
}
