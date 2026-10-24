package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// failingEvents fails every publish, as a database refusing the outbox
// row would.
type failingEvents struct{}

func (failingEvents) Publish(context.Context, ...entities.DomainEvent) error {
	return errors.New("outbox unavailable")
}

// A domain change and the event that records it commit together: when the
// event cannot be written, the change is rolled back and the caller told.
func TestDomainChangesCommitWithTheirEvents(t *testing.T) {
	f := newEPPFixture(t)
	ctx := appcontext.WithEventSource(appcontext.WithUserID(context.Background(), "epp:"+f.rar1), "domain-os/epp")
	scope := entities.RegistrarClID(f.rar1)
	countEvents := func(subject, typ string) int64 {
		var n int64
		require.NoError(t, f.db.Table("domain_events").Where("subject = ? AND type = ?", subject, typ).Count(&n).Error)
		return n
	}
	create := commands.EPPCreateDomainCommand{
		Name: "tx." + f.tld, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}},
	}
	var real repositories.EventPublisher = f.events

	t.Run("a create whose event cannot be written creates nothing", func(t *testing.T) {
		f.registry.eventPublisher = failingEvents{}
		defer func() { f.registry.eventPublisher = real }()
		_, err := f.domains.Create(ctx, scope, create)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outbox unavailable")
		_, err = f.domainRepo.GetDomainByName(context.Background(), create.Name, false)
		assert.ErrorIs(t, err, entities.ErrDomainNotFound, "rolled back with its event")
	})

	_, err := f.domains.Create(ctx, scope, create)
	require.NoError(t, err)
	assert.Equal(t, int64(1), countEvents(create.Name, "domain.registered"))

	t.Run("an update whose event cannot be written changes nothing", func(t *testing.T) {
		f.registry.eventPublisher = failingEvents{}
		defer func() { f.registry.eventPublisher = real }()
		_, err := f.registry.SetStatus(ctx, create.Name, entities.DomainStatusClientHold)
		require.Error(t, err)
		d, err := f.domainRepo.GetDomainByName(context.Background(), create.Name, false)
		require.NoError(t, err)
		assert.False(t, d.Status.ClientHold)
	})

	t.Run("events made over EPP name EPP as their source", func(t *testing.T) {
		var source string
		require.NoError(t, f.db.Table("domain_events").Select("source").
			Where("subject = ? AND type = ?", create.Name, "domain.registered").Scan(&source).Error)
		assert.Equal(t, "domain-os/epp", source)
	})
}

// A domain change's reads run in its transaction, on its connection: with
// one connection in the pool, a create and a renew still finish, where a
// read that went back to the pool would wait for a second connection
// until its deadline.
func TestDomainChangesNeedOneConnection(t *testing.T) {
	f := newEPPFixture(t)
	sqlDB, err := f.db.DB()
	require.NoError(t, err)
	stats := sqlDB.Stats()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.SetMaxOpenConns(stats.MaxOpenConnections) })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	scope := entities.RegistrarClID(f.rar1)
	name := "pool." + f.tld
	_, err = f.domains.Create(ctx, scope, commands.EPPCreateDomainCommand{
		Name: name, Period: 1, PeriodUnit: "y", RegistrantID: "c" + f.rar1,
		Contacts: []commands.EPPDomainContact{{Type: "tech", ID: "c" + f.rar1}},
	})
	require.NoError(t, err, "create, priced and registered in one connection")
	d, err := f.domainRepo.GetDomainByName(ctx, name, false)
	require.NoError(t, err)
	_, err = f.domains.Renew(ctx, scope, commands.EPPRenewDomainCommand{Name: name, CurExpDate: d.ExpiryDate.UTC().Format("2006-01-02"), Period: 1, PeriodUnit: "y"})
	require.NoError(t, err, "renew, priced and written in one connection")
}
