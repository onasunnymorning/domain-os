package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/tj/assert"
)

func TestRegistryOperator_TableName(t *testing.T) {
	ro := RegistryOperator{}
	assert.Equal(t, "registry_operators", ro.TableName())
}

func TestRegistryOperator_ToEntity(t *testing.T) {
	ro := RegistryOperator{
		RyID:      "ry-id",
		Name:      "name",
		URL:       "http://example.com",
		Email:     "me@online.com",
		Voice:     "+123456",
		Fax:       "+123456",
		CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	e := ro.ToEntity()

	assert.Equal(t, ro.RyID, e.RyID.String())
	assert.Equal(t, ro.Name, e.Name)
	assert.Equal(t, ro.URL, e.URL.String())
	assert.Equal(t, ro.Email, e.Email)
	assert.Equal(t, ro.Voice, e.Voice.String())
	assert.Equal(t, ro.Fax, e.Fax.String())
	assert.Equal(t, ro.CreatedAt, e.CreatedAt)
	assert.Equal(t, ro.UpdatedAt, e.UpdatedAt)
}

func TestRegistryOperator_FromEntity(t *testing.T) {
	e := entities.RegistryOperator{}
	ro := RegistryOperator{
		RyID:      "ry-id",
		Name:      "name",
		URL:       "http://example.com",
		Email:     "me@online.com",
		Voice:     "+123456",
		Fax:       "+123456",
		CreatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		PremiumLists: []*PremiumList{
			{
				Name: "premium-list1",
			},
			{
				Name: "premium-list2",
			},
		},
		TLDs: []*TLD{
			{
				Name: "tld1",
			},
			{
				Name: "tld2",
			},
		},
	}

	ro.FromEntity(&e)

	assert.Equal(t, e.RyID.String(), ro.RyID)
	assert.Equal(t, e.Name, ro.Name)
	assert.Equal(t, e.URL.String(), ro.URL)
	assert.Equal(t, e.Email, ro.Email)
	assert.Equal(t, e.Voice.String(), ro.Voice)
	assert.Equal(t, e.Fax.String(), ro.Fax)
	assert.Equal(t, e.CreatedAt, ro.CreatedAt)
	assert.Equal(t, e.UpdatedAt, ro.UpdatedAt)
	assert.Equal(t, len(e.PremiumLists), len(ro.PremiumLists))
	assert.Equal(t, len(e.TLDs), len(ro.TLDs))

}
