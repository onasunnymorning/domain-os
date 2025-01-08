package postgres

import (
	"context"
	"fmt"
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

func (s *RegistrarSuite) TestIsRegistrarAccreditedForTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	clid := "test-registrar"

	ctx := context.Background()
	repo := NewGormRegistrarRepository(tx)

	// Create a test registrar
	registrar, err := entities.NewRegistrar(clid, "Test Inc.", "test@inc.com", 9999, getValidRegistrarPostalInfoArr())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrar)

	created, err := repo.Create(ctx, registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), created)
	fmt.Println(created)

	// Create a test TLD
	err = tx.Exec("INSERT INTO tlds (name) VALUES (?)", "com").Error
	require.NoError(s.T(), err)

	// Manually insert some record that the IsRegistrarAccreditedForTLD method expects to find.
	// Adjust the table/columns below to match your actual accreditation schema.
	err = tx.Exec("INSERT INTO accreditations (registrar_cl_id, tld_name) VALUES (?, ?)",
		clid, "com").Error
	require.NoError(s.T(), err)

	// This should return true and no error
	accredited, err := repo.IsRegistrarAccreditedForTLD(ctx, "com", clid)
	require.NoError(s.T(), err)
	require.True(s.T(), accredited)

	// This should return false
	accredited, err = repo.IsRegistrarAccreditedForTLD(ctx, "net", clid)
	require.NoError(s.T(), err)
	require.False(s.T(), accredited)

	// This should return false
	accredited, err = repo.IsRegistrarAccreditedForTLD(ctx, "NOT NULL", clid)
	require.NoError(s.T(), err)
	require.False(s.T(), accredited)
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

	// Check the created registrar
	readRegistrar, err := repo.GetByClID(context.Background(), registrar.ClID.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readRegistrar)
	require.Equal(s.T(), createdRegistrar, readRegistrar)

	// Delete the registrar
	err = repo.Delete(context.Background(), registrar.ClID.String())
	require.NoError(s.T(), err)

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

	// Try and Create a duplicate
	_, err = repo.Create(context.Background(), registrar)
	require.Error(s.T(), err)

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

	readRegistrar, err := repo.GetByClID(context.Background(), registrar.ClID.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readRegistrar)
	require.Equal(s.T(), createdRegistrar, readRegistrar)

	readRegistrar, err = repo.GetByGurID(context.Background(), registrar.GurID)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readRegistrar)
	require.Equal(s.T(), createdRegistrar, readRegistrar)

	// Error record not found
	readRegistrar, err = repo.GetByGurID(context.Background(), 1234556657)
	require.ErrorIs(s.T(), err, entities.ErrRegistrarNotFound)
	require.Nil(s.T(), readRegistrar)
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

	// Delete the registrar
	err = repo.Delete(context.Background(), createdRegistrar.ClID.String())
	require.NoError(s.T(), err)
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

	_, err = repo.GetByClID(context.Background(), createdRegistrar.ClID.String(), false)
	require.Error(s.T(), err)

	err = repo.Delete(context.Background(), createdRegistrar.ClID.String())
	require.NoError(s.T(), err)

	_, err = repo.GetByClID(context.Background(), createdRegistrar.ClID.String(), false)
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

	// Delete one registrar
	err = repo.Delete(context.Background(), createdRegistrar1.ClID.String())
	require.NoError(s.T(), err)

	registrars, err = repo.List(context.Background(), 2, "")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrars)
	require.GreaterOrEqual(s.T(), len(registrars), 1)

	// Delete the other registrar
	err = repo.Delete(context.Background(), createdRegistrar2.ClID.String())
	require.NoError(s.T(), err)

	registrars, err = repo.List(context.Background(), 2, "")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrars)

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

func (s *RegistrarSuite) TestCountRegistrars() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	registrar, _ := entities.NewRegistrar("my-registrar-id", "Gomamma Inc.",
		"contact@gomamma.com", 12345, getValidRegistrarPostalInfoArr())
	createdRegistrar, err := repo.Create(context.Background(), registrar)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar)

	count, err := repo.Count(context.Background())
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), count, int64(1)) // Other tests might create a regsitrar as part of their setup

	registrar2, _ := entities.NewRegistrar("my-registrar-id2", "GoBro Inc.",
		"contact@gobro.com", 12346, getValidRegistrarPostalInfoArr())
	createdRegistrar2, err := repo.Create(context.Background(), registrar2)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdRegistrar2)

	count, err = repo.Count(context.Background())
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), count, int64(2))

	// Delete one registrar
	err = repo.Delete(context.Background(), createdRegistrar.ClID.String())
	require.NoError(s.T(), err)

	count, err = repo.Count(context.Background())
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), count, int64(1))

	// Delete the other registrar
	err = repo.Delete(context.Background(), createdRegistrar2.ClID.String())
	require.NoError(s.T(), err)

	count, err = repo.Count(context.Background())
	require.NoError(s.T(), err)
	require.GreaterOrEqual(s.T(), count, int64(0))

}
