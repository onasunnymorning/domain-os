package services

import (
	"context"
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockContactRepository is a mock implementation of the ContactRepository interface
type MockContactRepository struct {
	mock.Mock
}

func (m *MockContactRepository) CreateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Contact), args.Error(1)
}

func (m *MockContactRepository) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Contact), args.Error(1)
}

func (m *MockContactRepository) UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	args := m.Called(ctx, c)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entities.Contact), args.Error(1)
}

func (m *MockContactRepository) DeleteContactByID(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContactRepository) ListContacts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Contact, string, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).([]*entities.Contact), args.String(1), args.Error(2)
}

func (m *MockContactRepository) BulkCreate(ctx context.Context, contacts []*entities.Contact) error {
	args := m.Called(ctx, contacts)
	return args.Error(0)
}

func TestContactService_CreateContact(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	t.Run("Valid contact without RoID", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		contact, err := entities.NewContact("contact123", "123_CONT-APEX", "test@example.com", "sTr0N5p@zzWqRD", "client123")
		assert.NoError(t, err)

		mockRepo.On("CreateContact", mock.Anything, mock.AnythingOfType("*entities.Contact")).
			Return(contact, nil)

		cmd := &commands.CreateContactCommand{
			ID:       "contact123",
			Email:    "test@example.com",
			AuthInfo: "sTr0N5p@zzWqRD",
			ClID:     "client123",
			PostalInfo: [2]*entities.ContactPostalInfo{
				{
					Type: entities.PostalInfoEnumTypeLOC,
					Name: "John Doe",
					Address: &entities.Address{
						City:        entities.PostalLineType("Anytown"),
						CountryCode: entities.CCType("US"),
					},
				},
			},
		}

		result, err := service.CreateContact(context.Background(), cmd)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "contact123", result.ID.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("Valid contact with RoID", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		contact, err := entities.NewContact("contact456", "123_CONT-APEX", "test2@example.com", "sTr0N5p@zzWqRD", "client456")
		assert.NoError(t, err)

		mockRepo.On("CreateContact", mock.Anything, mock.AnythingOfType("*entities.Contact")).
			Return(contact, nil)

		cmd := &commands.CreateContactCommand{
			ID:       "contact456",
			RoID:     "123_CONT-APEX",
			Email:    "test2@example.com",
			AuthInfo: "sTr0N5p@zzWqRD",
			ClID:     "client456",
			PostalInfo: [2]*entities.ContactPostalInfo{
				{
					Type: entities.PostalInfoEnumTypeLOC,
					Name: "Jane Smith",
					Address: &entities.Address{
						City:        entities.PostalLineType("Somewhere"),
						CountryCode: entities.CCType("US"),
					},
				},
			},
		}

		result, err := service.CreateContact(context.Background(), cmd)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "contact456", result.ID.String())
		assert.Equal(t, "123_CONT-APEX", result.RoID.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("Repository error", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("CreateContact", mock.Anything, mock.AnythingOfType("*entities.Contact")).
			Return(nil, errors.New("database error"))

		cmd := &commands.CreateContactCommand{
			ID:       "contact789",
			Email:    "test3@example.com",
			AuthInfo: "sTr0N5p@zzWqRD",
			ClID:     "client789",
			PostalInfo: [2]*entities.ContactPostalInfo{
				{
					Type: entities.PostalInfoEnumTypeLOC,
					Name: "Bob Johnson",
					Address: &entities.Address{
						City:        entities.PostalLineType("Elsewhere"),
						CountryCode: entities.CCType("US"),
					},
				},
			},
		}

		result, err := service.CreateContact(context.Background(), cmd)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database error")
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Invalid contact - empty ID", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		cmd := &commands.CreateContactCommand{
			ID:       "",
			Email:    "test@example.com",
			AuthInfo: "sTr0N5p@zzWqRD",
			ClID:     "client123",
		}

		result, err := service.CreateContact(context.Background(), cmd)

		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestContactService_GetContactByID(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	t.Run("Contact found", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		contact, err := entities.NewContact("contact123", "123_CONT-APEX", "test@example.com", "sTr0N5p@zzWqRD", "client123")
		assert.NoError(t, err)
		mockRepo.On("GetContactByID", mock.Anything, "contact123").
			Return(contact, nil)

		result, err := service.GetContactByID(context.Background(), "contact123")

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "contact123", result.ID.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("Contact not found", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("GetContactByID", mock.Anything, "nonexistent").
			Return(nil, errors.New("contact not found"))

		result, err := service.GetContactByID(context.Background(), "nonexistent")

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestContactService_UpdateContact(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	contact, err := entities.NewContact("contact123", "123_CONT-APEX", "test@example.com", "sTr0N5p@zzWqRD", "client123")
	assert.NoError(t, err)

	t.Run("Successful update", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("UpdateContact", mock.Anything, contact).
			Return(contact, nil)

		result, err := service.UpdateContact(context.Background(), contact)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Update error", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("UpdateContact", mock.Anything, contact).
			Return(nil, errors.New("update failed"))

		result, err := service.UpdateContact(context.Background(), contact)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestContactService_DeleteContactByID(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	t.Run("Successful deletion", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("DeleteContactByID", mock.Anything, "contact123").
			Return(nil)

		err := service.DeleteContactByID(context.Background(), "contact123")

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Deletion error", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("DeleteContactByID", mock.Anything, "contact456").
			Return(errors.New("deletion failed"))

		err := service.DeleteContactByID(context.Background(), "contact456")

		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestContactService_ListContacts(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	t.Run("List with results", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		contact1, err := entities.NewContact("contact1", "123_CONT-APEX", "test1@example.com", "sTr0N5p@zzWqRD", "client1")
		assert.NoError(t, err)
		contact2, err := entities.NewContact("contact2", "124_CONT-APEX", "test2@example.com", "sTr0N5p@zzWqRD", "client2")
		assert.NoError(t, err)
		contacts := []*entities.Contact{contact1, contact2}

		mockRepo.On("ListContacts", mock.Anything, mock.AnythingOfType("queries.ListItemsQuery")).
			Return(contacts, "", nil)

		params := queries.ListItemsQuery{PageSize: 10}
		result, cursor, err := service.ListContacts(context.Background(), params)

		assert.NoError(t, err)
		assert.Equal(t, 2, len(result))
		assert.Equal(t, "", cursor)
		mockRepo.AssertExpectations(t)
	})

	t.Run("List error", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("ListContacts", mock.Anything, mock.AnythingOfType("queries.ListItemsQuery")).
			Return(nil, "", errors.New("list failed"))

		params := queries.ListItemsQuery{PageSize: 10}
		result, _, err := service.ListContacts(context.Background(), params)

		assert.Error(t, err)
		assert.Nil(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestContactService_BulkCreate(t *testing.T) {
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	assert.NoError(t, err)
	roidService := NewRoidService(idgen)

	t.Run("Successful bulk create", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("BulkCreate", mock.Anything, mock.MatchedBy(func(contacts []*entities.Contact) bool {
			return len(contacts) == 2
		})).Return(nil)

		cmds := []*commands.CreateContactCommand{
			{
				ID:       "contact1",
				Email:    "test1@example.com",
				AuthInfo: "sTr0N5p@zzWqRD",
				ClID:     "client1",
				PostalInfo: [2]*entities.ContactPostalInfo{
					{
						Type: entities.PostalInfoEnumTypeLOC,
						Name: "Person 1",
						Address: &entities.Address{
							City:        entities.PostalLineType("City1"),
							CountryCode: entities.CCType("US"),
						},
					},
				},
			},
			{
				ID:       "contact2",
				Email:    "test2@example.com",
				AuthInfo: "sTr0N5p@zzWqRD",
				ClID:     "client2",
				PostalInfo: [2]*entities.ContactPostalInfo{
					{
						Type: entities.PostalInfoEnumTypeLOC,
						Name: "Person 2",
						Address: &entities.Address{
							City:        entities.PostalLineType("City2"),
							CountryCode: entities.CCType("US"),
						},
					},
				},
			},
		}

		err := service.BulkCreate(context.Background(), cmds)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Bulk create with repository error", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		mockRepo.On("BulkCreate", mock.Anything, mock.AnythingOfType("[]*entities.Contact")).
			Return(errors.New("bulk insert failed"))

		cmds := []*commands.CreateContactCommand{
			{
				ID:       "contact3",
				Email:    "test3@example.com",
				AuthInfo: "sTr0N5p@zzWqRD",
				ClID:     "client3",
				PostalInfo: [2]*entities.ContactPostalInfo{
					{
						Type: entities.PostalInfoEnumTypeLOC,
						Name: "Person 3",
						Address: &entities.Address{
							City:        entities.PostalLineType("City3"),
							CountryCode: entities.CCType("US"),
						},
					},
				},
			},
		}

		err := service.BulkCreate(context.Background(), cmds)

		assert.Error(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Bulk create with invalid contact", func(t *testing.T) {
		mockRepo := new(MockContactRepository)
		service := NewContactService(mockRepo, *roidService)

		cmds := []*commands.CreateContactCommand{
			{
				ID:       "",
				Email:    "test@example.com",
				AuthInfo: "sTr0N5p@zzWqRD",
				ClID:     "client",
			},
		}

		err := service.BulkCreate(context.Background(), cmds)

		assert.Error(t, err)
	})
}
