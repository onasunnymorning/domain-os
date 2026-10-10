package activities

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// syncEventCapture records the events the registrar service publishes.
type syncEventCapture struct{ events []entities.DomainEvent }

func (c *syncEventCapture) Publish(_ context.Context, events ...entities.DomainEvent) error {
	c.events = append(c.events, events...)
	return nil
}

// existingRegistrarItems reads what the sync's GetRegistrarListItems step sees:
// every stored registrar, as a list item.
func existingRegistrarItems(t *testing.T, db *gorm.DB) []entities.RegistrarListItem {
	t.Helper()
	var rows []struct {
		ClID, Name, Status, IANAStatus string
		GurID                          int
	}
	require.NoError(t, db.Table("registrars").
		Select("cl_id AS cl_id, name, gur_id, status, iana_status").Scan(&rows).Error)
	items := make([]entities.RegistrarListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, entities.RegistrarListItem{
			ClID:       entities.ClIDType(r.ClID),
			Name:       r.Name,
			GurID:      r.GurID,
			Status:     entities.RegistrarStatus(r.Status),
			IANAStatus: entities.IANARegistrarStatus(r.IANAStatus),
		})
	}
	return items
}

// TestRegistrarSync_NameCollisionDoesNotReplayEveryRun is the regression test
// for the daily "bulk created 3 registrars" event.
//
// An IANA registrar whose name is already held by a registrar with a different
// ClID used to be planned as a create on every run. The insert was silently
// dropped by ON CONFLICT DO NOTHING on the unique name, yet the service still
// announced it as created. The registrar never appeared, so the same create and
// the same event came back the next day.
//
// It runs the real diff, service and repository against Postgres, twice, the way
// the daily schedule does.
func TestRegistrarSync_NameCollisionDoesNotReplayEveryRun(t *testing.T) {
	ctx := context.Background()
	db := testDB(t)
	tx := db.Begin()
	defer tx.Rollback()

	repo := postgres.NewGormRegistrarRepository(tx)
	pub := &syncEventCapture{}
	svc := services.NewRegistrarService(repo, pub)

	holder := entities.IANARegistrar{GurID: 9701, Name: "Rgsn Holder", Status: entities.IANARegistrarStatusAccredited}
	// Same name as the holder, different gurid and therefore different ClID.
	collider := entities.IANARegistrar{GurID: 9702, Name: "Rgsn Holder", Status: entities.IANARegistrarStatusAccredited}
	fresh := entities.IANARegistrar{GurID: 9703, Name: "Rgsn Fresh", Status: entities.IANARegistrarStatusAccredited}

	// The holder is already in the registry.
	holderCmd, err := commands.CreateCreateRegistrarCommandFromIANARegistrar(holder)
	require.NoError(t, err)
	_, err = svc.BulkCreate(ctx, []*commands.CreateRegistrarCommand{holderCmd})
	require.NoError(t, err)
	pub.events = nil

	iana := []entities.IANARegistrar{holder, collider, fresh}

	runSync := func() (DiffPlanResult, []string) {
		t.Helper()
		plan, err := DiffAndPlanRegistrars(ctx, "corr", iana, existingRegistrarItems(t, tx))
		require.NoError(t, err)
		cmds := make([]*commands.CreateRegistrarCommand, 0, len(plan.Creates))
		for i := range plan.Creates {
			cmds = append(cmds, &plan.Creates[i])
		}
		created, err := svc.BulkCreate(ctx, cmds)
		require.NoError(t, err)
		return plan, created
	}

	// Run 1: both new registrars are created, the collider under a suffixed name.
	plan, created := runSync()
	require.Len(t, plan.Creates, 2)
	require.ElementsMatch(t, []string{"9702-rgsn-holder", "9703-rgsn-fresh"}, created)
	require.Equal(t, []string{"9702-rgsn-holder"}, plan.RenamedForUniqueness)

	require.Len(t, pub.events, 1)
	require.Equal(t, "registrar.bulk_created", pub.events[0].Type)
	payload, ok := pub.events[0].Data.(*entities.RegistrarLifecycleEvent)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"9702-rgsn-holder", "9703-rgsn-fresh"}, payload.ClientIDs)

	// The registrar that used to go missing now exists, with a distinct name.
	got, err := repo.GetByClID(ctx, "9702-rgsn-holder", false)
	require.NoError(t, err)
	require.Equal(t, "Rgsn Holder-2", got.Name)

	// Run 2 is the one that used to repeat: nothing to create, nothing announced.
	pub.events = nil
	plan, created = runSync()
	require.Empty(t, plan.Creates, "the same registrars were planned again")
	require.Empty(t, created)
	require.Empty(t, pub.events, "registrar.bulk_created was emitted although nothing was created")
}
