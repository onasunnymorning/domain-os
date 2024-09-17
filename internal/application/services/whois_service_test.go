// services/whois_service_test.go

package services

import (
	"context"
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetDomainWhois(t *testing.T) {
	// Create mock domain and registrar repositories
	mockDomRepo := new(repositories.MockDomainRepository)
	mockRarRepo := new(repositories.MockRegistrarRepository)

	// Create a WhoisService with the mock repositories
	service := NewWhoisService(mockDomRepo, mockRarRepo)

	// Define test data
	domainName := "example.com"
	mockDomain := &entities.Domain{
		Name: entities.DomainName(domainName),
		ClID: entities.ClIDType("testClID"),
	}
	mockRegistrar := &entities.Registrar{
		Name: "Test Registrar",
	}

	// Set up expectations for the mock domain repository
	mockDomRepo.On("GetDomainByName", mock.Anything, domainName, true).Return(mockDomain, nil)

	// Set up expectations for the mock registrar repository
	mockRarRepo.On("GetByClID", mock.Anything, mockDomain.ClID.String(), false).Return(mockRegistrar, nil)

	// Call the method being tested
	result, err := service.GetDomainWhois(context.TODO(), domainName)

	// Assert that there were no errors
	assert.NoError(t, err)

	// Assert that the result matches expected values
	assert.Equal(t, domainName, result.DomainName)
	assert.Equal(t, "Test Registrar", result.Registrar)

	// Verify that the expectations were met
	mockDomRepo.AssertExpectations(t)
	mockRarRepo.AssertExpectations(t)
}

func TestGetDomainWhois_RegistrarRepoError(t *testing.T) {
	// Create mock domain and registrar repositories
	mockDomRepo := new(repositories.MockDomainRepository)
	mockRarRepo := new(repositories.MockRegistrarRepository)

	// Create the service with the mock repositories
	service := NewWhoisService(mockDomRepo, mockRarRepo)

	// Define test data
	domainName := "example.com"
	mockDomain := &entities.Domain{
		Name: entities.DomainName(domainName),
		ClID: entities.ClIDType("testClID"),
	}
	mockError := errors.New("registrar not found")

	// Set up the domain repo to return a valid domain
	mockDomRepo.On("GetDomainByName", mock.Anything, domainName, true).Return(mockDomain, nil)

	// Set up the registrar repo to return an error
	mockRarRepo.On("GetByClID", mock.Anything, mockDomain.ClID.String(), false).Return((*entities.Registrar)(nil), mockError)

	// Call the method
	result, err := service.GetDomainWhois(context.TODO(), domainName)

	// Assert that the error is returned
	assert.Nil(t, result)
	assert.EqualError(t, err, "registrar not found")

	// Verify that the expectations were met
	mockDomRepo.AssertExpectations(t)
	mockRarRepo.AssertExpectations(t)
}

func TestGetDomainWhois_DomainRepoError(t *testing.T) {
	// Create mock domain and registrar repositories
	mockDomRepo := new(repositories.MockDomainRepository)
	mockRarRepo := new(repositories.MockRegistrarRepository)

	// Create the service with the mock repositories
	service := NewWhoisService(mockDomRepo, mockRarRepo)

	// Define test data
	domainName := "example.com"
	mockError := errors.New("domain not found")

	// Set up the domain repo to return an error
	mockDomRepo.On("GetDomainByName", mock.Anything, domainName, true).Return((*entities.Domain)(nil), mockError)

	// Call the method
	result, err := service.GetDomainWhois(context.TODO(), domainName)

	// Assert that the error is returned
	assert.Nil(t, result)
	assert.EqualError(t, err, "domain not found")

	// Verify that the expectations were met
	mockDomRepo.AssertExpectations(t)
	mockRarRepo.AssertExpectations(t)
}
