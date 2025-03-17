package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

func (s *ContactSuite) TestBulkCreateContacts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact1, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	contact2, err := entities.NewContact("contactID2", "1235_CONT-APEX", "jane@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	contacts := []*entities.Contact{contact1, contact2}

	err = repo.BulkCreate(context.Background(), contacts)
	s.Require().NoError(err)

	createdContact1, err := repo.GetContactByID(context.Background(), contact1.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(createdContact1)

	createdContact2, err := repo.GetContactByID(context.Background(), contact2.ID.String())
	s.Require().NoError(err)
	s.Require().NotNil(createdContact2)
}

func (s *ContactSuite) TestBulkCreateContacts_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	contact1, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jon@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	contact2, err := entities.NewContact("contactID1", "1234_CONT-APEX", "jane@doe.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)

	contacts := []*entities.Contact{contact1, contact2}

	err = repo.BulkCreate(context.Background(), contacts)
	s.Require().Error(err)
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

func (s *ContactSuite) TestListContacts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	a, err := entities.NewAddress("El Cuyo", "MX")
	s.Require().NoError(err)
	pi, _ := entities.NewContactPostalInfo("int", "my pi", a)
	s.Require().NoError(err)

	contact1, err := entities.NewContact("clid1", "1234_CONT-APEX", "mail@me.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)
	err = contact1.AddPostalInfo(pi)
	s.Require().NoError(err)

	createdContact1, err := repo.CreateContact(context.Background(), contact1)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact1)

	contact2, err := entities.NewContact("clid2", "1235_CONT-APEX", "mail@me.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)
	err = contact2.AddPostalInfo(pi)
	s.Require().NoError(err)
	createdContact2, err := repo.CreateContact(context.Background(), contact2)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact2)

	contacts, _, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 2)

	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize:   25,
		PageCursor: "1234_CONT-APEX",
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 1)

	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize:   25,
		PageCursor: "1234_HOST-APEX",
	})
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)
	s.Require().Nil(contacts)

	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize:   25,
		PageCursor: "abc_CONT-APEX",
	})
	s.Require().Error(err)
	s.Require().Nil(contacts)
}

func (s *ContactSuite) TestFilterContacts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	a, err := entities.NewAddress("El Cuyo", "MX")
	s.Require().NoError(err)
	pi, _ := entities.NewContactPostalInfo("int", "my pi", a)
	s.Require().NoError(err)

	contact1, err := entities.NewContact("clid1", "1234_CONT-APEX", "clid1@me.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)
	err = contact1.AddPostalInfo(pi)
	s.Require().NoError(err)

	createdContact1, err := repo.CreateContact(context.Background(), contact1)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact1)

	contact2, err := entities.NewContact("clid2", "1235_CONT-APEX", "clid2@me.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)
	err = contact2.AddPostalInfo(pi)
	s.Require().NoError(err)
	createdContact2, err := repo.CreateContact(context.Background(), contact2)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact2)

	contact3, err := entities.NewContact("clid3", "1236_CONT-APEX", "clid3@me.com", "str0NGP@ZZw0rd", s.rarClid)
	s.Require().NoError(err)
	err = contact3.AddPostalInfo(pi)
	s.Require().NoError(err)
	createdContact3, err := repo.CreateContact(context.Background(), contact3)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact3)

	// Test RoidGreaterThan
	contacts, _, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidGreaterThan: "1234_CONT-APEX",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 2)

	// Test RoidLessThan + RoidGreaterThan
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidGreaterThan: "1234_CONT-APEX",
			RoidLessThan:    "1236_CONT-APEX",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 1)

	// Test IdLike
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			IdLike: "clid",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 3)

	// Test EmailLike 1
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			EmailLike: "clid1",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 1)

	// Test EmailLike 3
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			EmailLike: "me.com",
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 3)

	// Test ClidEquals
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			ClidEquals: s.rarClid,
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 3)

	// ClidEquals + Pagination
	contacts, cursor, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
		Filter: queries.ListContactsFilter{
			ClidEquals: s.rarClid,
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 2)
	s.Require().NotEqual("", cursor)

	// ClidEquals + Pagination + Cursor
	contacts, newCursor, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize:   2,
		PageCursor: cursor,
		Filter: queries.ListContactsFilter{
			ClidEquals: s.rarClid,
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(contacts)
	s.Require().Len(contacts, 1)
	s.Require().Equal("", newCursor)
}

func (s *ContactSuite) TestFilterContactsRoidGTError() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	// invalid roid
	contacts, _, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidGreaterThan: "1234_DOM-APEX",
		},
	})
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)
	s.Require().Nil(contacts)

	// invalid roid int64
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidGreaterThan: "abcd_DOM-APEX",
		},
	})
	s.Require().Error(err)
	s.Require().Nil(contacts)
}

func (s *ContactSuite) TestFilterContactsRoidLTError() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewContactRepository(tx)

	// invalid roid
	contacts, _, err := repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidLessThan: "1234_DOM-APEX",
		},
	})
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)
	s.Require().Nil(contacts)

	// invalid roid int64
	contacts, _, err = repo.ListContacts(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListContactsFilter{
			RoidLessThan: "abcd_DOM-APEX",
		},
	})
	s.Require().Error(err)
	s.Require().Nil(contacts)
}
