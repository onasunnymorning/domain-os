package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestAdminDomainUpdateOverPostgres: the admin API's update writes only
// over the version the caller read, so a copy loaded before a renew cannot
// undo it.
func TestAdminDomainUpdateOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx := context.Background()
	scope := entities.RegistrarClID(f.rar1)
	name := "admin." + f.tld
	_, err := f.domains.Create(ctx, scope, commands.EPPCreateDomainCommand{
		Name: name, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}}, AuthInfo: "Tr4nsf3r-C0de-Xq9#Lm2!",
	})
	require.NoError(t, err)

	// What the admin UI loads, and later sends back with one field changed.
	loaded, err := f.registry.GetDomainByName(ctx, name, false)
	require.NoError(t, err)
	edit := func(d *entities.Domain) *commands.UpdateDomainCommand {
		return &commands.UpdateDomainCommand{
			RegistrantID: d.RegistrantID.String(), TechID: d.TechID.String(), ClID: d.ClID.String(),
			CrRr: d.CrRr.String(), UpRr: d.UpRr.String(), ExpiryDate: d.ExpiryDate,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, Status: d.Status, RGPStatus: d.RGPStatus,
			GrandFathering: entities.DomainGrandFathering{GFAmount: 100, GFCurrency: "USD", GFExpiryCondition: "transfer"},
		}
	}

	t.Run("a copy loaded before a renew is refused, and the renew kept", func(t *testing.T) {
		_, err := f.domains.Renew(ctx, scope, commands.EPPRenewDomainCommand{Name: name, CurExpDate: loaded.ExpiryDate.UTC().Format("2006-01-02")})
		require.NoError(t, err)
		_, err = f.registry.UpdateDomain(ctx, name, edit(loaded))
		require.ErrorIs(t, err, entities.ErrDomainChanged)
		stored, err := f.registry.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.Equal(t, loaded.ExpiryDate.AddDate(1, 0, 0).UTC(), stored.ExpiryDate.UTC())
	})
	t.Run("the current copy is written, with a new version", func(t *testing.T) {
		current, err := f.registry.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		updated, err := f.registry.UpdateDomain(ctx, name, edit(current))
		require.NoError(t, err)
		assert.True(t, updated.UpdatedAt.After(current.UpdatedAt))
		stored, err := f.registry.GetDomainByName(ctx, name, false)
		require.NoError(t, err)
		assert.Equal(t, uint64(100), stored.GrandFathering.GFAmount)
		assert.WithinDuration(t, time.Now(), stored.UpdatedAt, time.Minute, "the server stamps the version, not the caller")
	})
}
