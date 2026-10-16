package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestEPPContactCommandsOverPostgres runs the contact commands on the
// Postgres repository, as cmd/epp wires them: create, a partial update
// written over the contact as read, the link count behind linked and
// delete, and the events, whose authorization information is redacted.
func TestEPPContactCommandsOverPostgres(t *testing.T) {
	f := newEPPFixture(t)
	ctx := appcontext.WithUserID(context.Background(), "epp:"+f.rar1)
	repo := postgres.NewContactRepository(f.db, "EPPTEST")
	svc := NewEPPContactService(EPPContactServiceDeps{Contacts: repo, Roids: f.roids, Events: f.events})
	scope := entities.RegistrarClID(f.rar1)
	str := func(s string) *string { return &s }
	id := "e" + f.rar1

	created, err := svc.Create(ctx, scope, commands.EPPCreateContactCommand{
		ID: id,
		PostalInfo: []commands.EPPPostalInfo{{Type: "int", Name: str("Ada Lovelace"), Org: str("Engines Ltd."),
			Addr: &commands.EPPContactAddress{Streets: []string{"12 St James's Square"}, City: "London", CC: "gb"}}},
		Voice: &commands.EPPPhone{Number: "+44.2071234567"},
		Email: "ada@example.com", AuthInfo: "Contact-Auth-1!",
	})
	require.NoError(t, err)
	t.Run("created as given, with a minted ROID", func(t *testing.T) {
		assert.Regexp(t, `^\d+_CONT-EPPTEST$`, created.RoID.String())
		stored, err := repo.GetContactByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "Engines Ltd.", stored.PostalInfo[0].Org.String(), "a trailing dot is kept")
		assert.Equal(t, "GB", string(stored.PostalInfo[0].Address.CountryCode))
		assert.Equal(t, entities.ClIDType(f.rar1), stored.CrRr)
	})
	t.Run("a partial update keeps the rest", func(t *testing.T) {
		_, err := svc.Update(ctx, scope, commands.EPPUpdateContactCommand{
			ID: id, AddStatuses: []string{"clientDeleteProhibited"},
			PostalInfo: []commands.EPPPostalInfo{{Type: "int", Name: str("Ada King")}},
		})
		require.NoError(t, err)
		stored, err := repo.GetContactByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "Ada King", stored.PostalInfo[0].Name.String())
		assert.Equal(t, "London", stored.PostalInfo[0].Address.City.String())
		assert.True(t, stored.Status.ClientDeleteProhibited)
		assert.Equal(t, entities.ClIDType(f.rar1), stored.UpRr)
	})
	t.Run("a write over a contact changed since it was read is refused", func(t *testing.T) {
		read, err := repo.GetContactByID(ctx, id)
		require.NoError(t, err)
		_, err = svc.Update(ctx, scope, commands.EPPUpdateContactCommand{ID: id, Email: str("ada@example.net")})
		require.NoError(t, err)
		read.Email = "stale@example.com"
		_, err = repo.UpdateContactIfUnchanged(ctx, read)
		require.ErrorIs(t, err, entities.ErrContactChanged)
		stored, err := repo.GetContactByID(ctx, id)
		require.NoError(t, err)
		assert.Equal(t, "ada@example.net", stored.Email)
	})

	t.Run("a contact a domain uses is linked, and is not deleted", func(t *testing.T) {
		_, err := svc.Update(ctx, scope, commands.EPPUpdateContactCommand{ID: id, RemoveStatuses: []string{"clientDeleteProhibited"}})
		require.NoError(t, err)
		_, err = f.domains.Create(ctx, scope, commands.EPPCreateDomainCommand{
			Name: "contacts." + f.tld, Period: 1, PeriodUnit: "y", RegistrantID: id,
			Contacts: []commands.EPPDomainContact{{Type: "tech", ID: id}}, AuthInfo: "Domain-Auth-1!",
		})
		require.NoError(t, err)
		info, err := svc.Info(ctx, scope, id)
		require.NoError(t, err)
		assert.True(t, info.Status.Linked)
		require.ErrorIs(t, svc.Delete(ctx, scope, id), entities.ErrEPPContactLinked)
	})
	t.Run("an unused contact is deleted, and not written back", func(t *testing.T) {
		other := "f" + f.rar1
		_, err := svc.Create(ctx, scope, commands.EPPCreateContactCommand{
			ID: other, PostalInfo: []commands.EPPPostalInfo{{Type: "int", Name: str("Bob"), Addr: &commands.EPPContactAddress{City: "Paris", CC: "FR"}}},
			Email: "bob@example.com", AuthInfo: "Contact-Auth-1!",
		})
		require.NoError(t, err)
		read, err := repo.GetContactByID(ctx, other)
		require.NoError(t, err)
		require.NoError(t, svc.Delete(ctx, scope, other))
		_, err = repo.UpdateContactIfUnchanged(ctx, read)
		require.ErrorIs(t, err, entities.ErrContactNotFound)
		_, err = repo.GetContactByID(ctx, other)
		require.ErrorIs(t, err, entities.ErrContactNotFound)
	})
	t.Run("its events name the login and redact the authorization information", func(t *testing.T) {
		var rows []struct{ Type, Command, AfterState string }
		require.NoError(t, f.db.Table("domain_events").
			Select("type, coalesce(command::text, '') AS command, coalesce(after_state::text, '') AS after_state").
			Where("subject = ? AND actor = ?", id, "epp:"+f.rar1).Scan(&rows).Error)
		var types []string
		for _, r := range rows {
			types = append(types, r.Type)
			assert.NotContains(t, r.Command+r.AfterState, "Contact-Auth-1!")
		}
		assert.Contains(t, types, "contact.created")
		assert.Contains(t, types, "contact.updated")
	})
}
