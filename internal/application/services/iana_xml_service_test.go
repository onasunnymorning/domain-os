package services

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

type MockIANARepository struct {
	Registrars []*entities.IANARegistrar
}

func NewMockIANARepository() repositories.IANARepository {
	return &MockIANARepository{
		Registrars: []*entities.IANARegistrar{
			{
				GurID:  9995,
				Name:   "Reserved for Pre-Delegation Testing transactions #1 reporting",
				Status: entities.IANARegistrarStatus("Reserved"),
			},
			{
				GurID:  9996,
				Name:   "Reserved for Pre-Delegation Testing transactions #2 reporting",
				Status: entities.IANARegistrarStatus("Reserved"),
			},
			{
				GurID:   10007,
				Name:    "Domain The Net Technologies Ltd.",
				Status:  entities.IANARegistrarStatus("Accredited"),
				RdapURL: "https://rdap.domainthenet.com/",
			},
		},
	}
}

func (repo *MockIANARepository) ListIANARegistrars() ([]*entities.IANARegistrar, error) {
	return repo.Registrars, nil
}

func TestIANAXMLService_ListIANARegistrars(t *testing.T) {
	repo := NewMockIANARepository()
	svc := NewIANAXMLService(repo)

	ianaRegistrars, err := svc.ListIANARegistrars()
	if err != nil {
		t.Error(err)
	}

	if len(ianaRegistrars) != 3 {
		t.Errorf("Expected 3 IANARegistrars, got %d", len(ianaRegistrars))
	}
}
