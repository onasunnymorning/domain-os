package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
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

	registrars, _, err := repo.List(context.Background(), queries.ListItemsQuery{PageSize: 2})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrars)
	require.Len(s.T(), registrars, 2)

	// Delete one registrar
	err = repo.Delete(context.Background(), createdRegistrar1.ClID.String())
	require.NoError(s.T(), err)

	registrars, _, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 2})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), registrars)
	require.GreaterOrEqual(s.T(), len(registrars), 1)

	// Delete the other registrar
	err = repo.Delete(context.Background(), createdRegistrar2.ClID.String())
	require.NoError(s.T(), err)

	registrars, _, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 2})
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

func (s *RegistrarSuite) TestListRegistrarsFilteringAndSorting() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormRegistrarRepository(tx)

	// Create test TLDs
	tx.Exec("INSERT INTO tlds (name) VALUES (?)", "com")
	tx.Exec("INSERT INTO tlds (name) VALUES (?)", "net")

	// Create 3 test registrars
	reg1, _ := entities.NewRegistrar("reg-id-1", "Registrar One", "one@test.com", 10001, getValidRegistrarPostalInfoArr())
	_, err := repo.Create(context.Background(), reg1)
	require.NoError(s.T(), err)

	reg2, _ := entities.NewRegistrar("reg-id-2", "Registrar Two", "two@test.com", 10002, getValidRegistrarPostalInfoArr())
	_, err = repo.Create(context.Background(), reg2)
	require.NoError(s.T(), err)

	reg3, _ := entities.NewRegistrar("reg-id-3", "Registrar Three", "three@test.com", 10003, getValidRegistrarPostalInfoArr())
	_, err = repo.Create(context.Background(), reg3)
	require.NoError(s.T(), err)

	// Add TLD accreditations
	// reg1: com
	tx.Exec("INSERT INTO accreditations (registrar_cl_id, tld_name) VALUES (?, ?)", "reg-id-1", "com")
	// reg2: net
	tx.Exec("INSERT INTO accreditations (registrar_cl_id, tld_name) VALUES (?, ?)", "reg-id-2", "net")
	// reg3: com, net
	tx.Exec("INSERT INTO accreditations (registrar_cl_id, tld_name) VALUES (?, ?)", "reg-id-3", "com")
	tx.Exec("INSERT INTO accreditations (registrar_cl_id, tld_name) VALUES (?, ?)", "reg-id-3", "net")

	// Add domains
	// reg1: 1 domain
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1001, "domain1.com", "reg-id-1", "com", time.Now(), "auth")
	// reg2: 3 domains
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1002, "domain2.net", "reg-id-2", "net", time.Now(), "auth")
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1003, "domain3.net", "reg-id-2", "net", time.Now(), "auth")
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1004, "domain4.net", "reg-id-2", "net", time.Now(), "auth")
	// reg3: 2 domains
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1005, "domain5.com", "reg-id-3", "com", time.Now(), "auth")
	tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		1006, "domain6.net", "reg-id-3", "net", time.Now(), "auth")

	// Test 1: List all and verify counts
	res, _, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
	})
	require.NoError(s.T(), err)
	// Find our test registrars in the results (there might be others from global migrations)
	var found1, found2, found3 *entities.RegistrarListItem
	for _, item := range res {
		if item.ClID.String() == "reg-id-1" {
			found1 = item
		} else if item.ClID.String() == "reg-id-2" {
			found2 = item
		} else if item.ClID.String() == "reg-id-3" {
			found3 = item
		}
	}
	require.NotNil(s.T(), found1)
	require.NotNil(s.T(), found2)
	require.NotNil(s.T(), found3)

	require.Equal(s.T(), 1, found1.DomainCount)
	require.Equal(s.T(), 3, found2.DomainCount)
	require.Equal(s.T(), 2, found3.DomainCount)

	require.Equal(s.T(), 1, found1.TLDCount)
	require.Contains(s.T(), found1.TLDList, "com")

	require.Equal(s.T(), 2, found3.TLDCount)
	require.Contains(s.T(), found3.TLDList, "com")
	require.Contains(s.T(), found3.TLDList, "net")

	// Test 2: Filter by TLD = net
	resFilter, _, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListRegistrarsFilter{
			TLD: "net",
		},
	})
	require.NoError(s.T(), err)
	// reg1 should NOT be in the results; reg2 and reg3 should be
	hasReg1 := false
	hasReg2 := false
	hasReg3 := false
	for _, item := range resFilter {
		if item.ClID.String() == "reg-id-1" {
			hasReg1 = true
		} else if item.ClID.String() == "reg-id-2" {
			hasReg2 = true
		} else if item.ClID.String() == "reg-id-3" {
			hasReg3 = true
		}
	}
	require.False(s.T(), hasReg1)
	require.True(s.T(), hasReg2)
	require.True(s.T(), hasReg3)

	// Test 3: Sort by domain_count DESC
	resSortDesc, _, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListRegistrarsFilter{
			SortBy:    "domain_count",
			SortOrder: "desc",
		},
	})
	require.NoError(s.T(), err)
	// We want to make sure reg2 (3 domains) comes before reg3 (2 domains) which comes before reg1 (1 domain)
	var orderIdx1, orderIdx2, orderIdx3 int = -1, -1, -1
	idx := 0
	for _, item := range resSortDesc {
		// Only check our test registrars (ignore others that might have 0 domains)
		if item.ClID.String() == "reg-id-1" {
			orderIdx1 = idx
			idx++
		} else if item.ClID.String() == "reg-id-2" {
			orderIdx2 = idx
			idx++
		} else if item.ClID.String() == "reg-id-3" {
			orderIdx3 = idx
			idx++
		}
	}
	require.True(s.T(), orderIdx2 < orderIdx3, "reg2 (3) should be before reg3 (2)")
	require.True(s.T(), orderIdx3 < orderIdx1, "reg3 (2) should be before reg1 (1)")
}
