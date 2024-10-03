// services/whois_service_test.go

package services

import (
	"context"
	"errors"
	"testing"
	"time"

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
		RoID:       entities.RoidType("1234567890_DOM-APEX"),
		Name:       entities.DomainName(domainName),
		ClID:       entities.ClIDType("testClID"),
		CreatedAt:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiryDate: time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		Status: entities.DomainStatus{
			OK: true,
		},
		Hosts: []*entities.Host{
			{
				Name: "ns1.example.com",
			},
			{
				Name: "ns2.example.com",
			},
		},
	}
	mockRegistrar := &entities.Registrar{
		Name: "Test Registrar",
		WhoisInfo: entities.WhoisInfo{
			Name: "whois.example.com",
		},
		URL:   "http://example.com",
		GurID: 2222,
		Email: "me@registrar.com",
		Voice: "+1.5555555555",
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
	assert.Equal(t, mockDomain.RoID.String(), result.RegistryDomainID)
	assert.Equal(t, mockRegistrar.WhoisInfo.Name.String(), result.RegistrarWhoisServer)
	assert.Equal(t, mockRegistrar.URL.String(), result.RegistrarURL)
	assert.Equal(t, mockDomain.UpdatedAt, result.UpdatedDate)
	assert.Equal(t, mockDomain.CreatedAt, result.CreationDate)
	assert.Equal(t, mockDomain.ExpiryDate, result.RegistryExpiryDate)
	assert.Equal(t, mockRegistrar.Name, result.Registrar)
	assert.Equal(t, "2222", result.RegistrarIANAID)
	assert.Equal(t, "me@registrar.com", result.RegistrarAbuseContactEmail)
	assert.Equal(t, "+1.5555555555", result.RegistrarAbuseContactPhone)
	assert.Equal(t, []string{"ok"}, result.DomainStatus)
	assert.Equal(t, []string{"ns1.example.com", "ns2.example.com"}, result.NameServers)
	assert.Equal(t, "unsigned", result.DNSSEC)
	assert.Equal(t, "URL of the ICANN Whois Inaccuracy Complaint Form: https://www.icann.org/wicf/", result.ICANNComplaintURL)
	assert.Equal(t, time.Now().Format(time.RFC3339), result.LastWhoisUpdate.Format(time.RFC3339))

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
