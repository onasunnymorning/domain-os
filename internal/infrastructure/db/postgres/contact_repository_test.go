package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type ContactSuite struct {
	suite.Suite
	db      *gorm.DB
	rarClid string
}

func TestContactSuite(t *testing.T) {
	suite.Run(t, new(ContactSuite))
}

func (s *ContactSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, _ := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199, getValidRegistrarPostalInfoArr())
	repo := NewGormRegistrarRepository(s.db)
	createdRar, _ := repo.Create(context.Background(), rar)
	s.rarClid = createdRar.ClID.String()
}

func (s *ContactSuite) TearDownSuite() {
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
}

func (s *ContactSuite) TestCreateContact() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)
}

func (s *ContactSuite) TestCreateContact_MissingFK() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", "missingFK")
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().Error(err)
	s.Require().Nil(createdContact)
}

func (s *ContactSuite) TestCreateContact_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)

	// Create a duplicate
	createdContact, err = repo.CreateContact(context.Background(), contact)
	s.Require().Error(err)
	s.Require().Nil(createdContact)
}

func (s *ContactSuite) TestReadContact() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)

	readContact, err := repo.GetContactByID(context.Background(), createdContact.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(readContact)
	s.Require().Equal(createdContact.ID, readContact.ID)
	s.Require().Equal(createdContact.ClID, readContact.ClID)
	s.Require().Equal(createdContact.Email, readContact.Email)
	s.Require().Equal(createdContact.RoID, readContact.RoID)
	s.Require().Equal(createdContact.AuthInfo, readContact.AuthInfo)
}

func (s *ContactSuite) TestUpdateContact() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)

	createdContact.Email = "new@mail.com"

	updatedContact, err := repo.UpdateContact(context.Background(), createdContact)
	s.Require().NoError(err)
	s.Require().NotNil(updatedContact)
	s.Require().Equal("new@mail.com", updatedContact.Email)

}

func (s *ContactSuite) TestDeleteContact() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	createdContact, err := repo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)

	err = repo.DeleteContactByID(context.Background(), createdContact.ID.String())
	s.Require().NoError(err)

	n, err := repo.GetContactByID(context.Background(), createdContact.ID.String())
	s.Require().Nil(n)
	s.Require().Error(err)

	err = repo.DeleteContactByID(context.Background(), createdContact.ID.String())
	s.Require().NoError(err)

	_, err = repo.GetContactByID(context.Background(), createdContact.ID.String())
	s.Require().Error(err)

}
