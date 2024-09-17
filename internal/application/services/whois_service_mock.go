package services

import (
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/mock"
)

// MockWhoisService is a mock implementation of the WhoisService
type MockWhoisService struct {
	mock.Mock
}

func (m *MockWhoisService) GetDomainWhois(ctx *gin.Context, domainName string) (entities.WhoisResponse, error) {
	args := m.Called(ctx, domainName)
	return args.Get(0).(entities.WhoisResponse), args.Error(1)
}
