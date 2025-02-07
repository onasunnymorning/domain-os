package interfaces

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

type RoidService interface {
	GenerateRoid(objectType string) (entities.RoidType, error)
	ListNode() int64
}

// MockRoidService is the mock implementation of the RoidService
type MockRoidService struct {
	GenerateRoidFunc func(objectType string) (entities.RoidType, error)
	ListNodeFunc     func() int64
}

// GenerateRoid generates a new Roid
func (m *MockRoidService) GenerateRoid(objectType string) (entities.RoidType, error) {
	return m.GenerateRoidFunc(objectType)
}

// ListNode returns the Node ID of the IDGenerator so we can register it and ensure its uniqueness
func (m *MockRoidService) ListNode() int64 {
	return m.ListNodeFunc()
}

// NewMockRoidService creates a new instance of MockRoidService
func NewMockRoidService() *MockRoidService {
	return &MockRoidService{}
}
