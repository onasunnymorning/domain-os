package postgres

import (
	"reflect"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestPremiumLabel_ToEntity(t *testing.T) {
	pl := &PremiumLabel{
		ID:                 1,
		Label:              "testLabel",
		PremiumListName:    "testList",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
		Currency:           "USD",
		Class:              "testClass",
	}

	expected := &entities.PremiumLabel{
		ID:                 1,
		Label:              entities.Label("testLabel"),
		PremiumListName:    "testList",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
		Currency:           "USD",
		Class:              "testClass",
	}

	entity := pl.ToEntity()
	if !reflect.DeepEqual(entity, expected) {
		t.Errorf("ToEntity() = %+v, expected %+v", entity, expected)
	}
}

func TestFromEntity(t *testing.T) {
	domainEntity := &entities.PremiumLabel{
		ID:                 2,
		Label:              entities.Label("anotherLabel"),
		PremiumListName:    "anotherList",
		RegistrationAmount: 500,
		RenewalAmount:      600,
		TransferAmount:     700,
		RestoreAmount:      800,
		Currency:           "EUR",
		Class:              "anotherClass",
	}

	expected := &PremiumLabel{
		ID:                 2,
		Label:              "anotherLabel",
		PremiumListName:    "anotherList",
		RegistrationAmount: 500,
		RenewalAmount:      600,
		TransferAmount:     700,
		RestoreAmount:      800,
		Currency:           "EUR",
		Class:              "anotherClass",
	}

	pl := FromEntity(domainEntity)
	if !reflect.DeepEqual(pl, expected) {
		t.Errorf("FromEntity() = %+v, expected %+v", pl, expected)
	}
}
