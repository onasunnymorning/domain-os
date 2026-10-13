package services

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/lockout"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/require"
)

type authTest struct {
	svc     *EPPAuthenticationService
	repo    *memstore.EPPCredentials
	uses    *EPPCredentialUseRecorder
	lockout *lockout.Memory
	cert    *x509.Certificate
}

var authSource = netip.MustParseAddr("192.0.2.7")

// newAuthTest seeds rar-001 with login rar-001, password first-pw-1, a pinned
// certificate and 192.0.2.0/24.
func newAuthTest(t *testing.T) *authTest {
	t.Helper()
	repo := memstore.NewEPPCredentials("rar-001", "rar-002")
	certPEM := testClientCertPEM(t)
	_, err := repo.UpdateAccess(context.Background(), "rar-001", func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		if _, err := a.AddLogin("rar-001", entities.EPPLoginRoleTransform, "seed", eppSvcNow); err != nil {
			return nil, err
		}
		if _, err := a.AddPassword("rar-001", "first-pw-1", prefixHasher{}, "seed", eppSvcNow); err != nil {
			return nil, err
		}
		if _, err := a.AddCertificatePin(certPEM, nil, "seed", eppSvcNow); err != nil {
			return nil, err
		}
		_, err := a.AddAllowedNetwork("192.0.2.0/24", "", "seed", eppSvcNow)
		return nil, err
	})
	require.NoError(t, err)
	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	lo := lockout.NewMemory(lockout.Policy{MaxFailures: 3, Window: time.Minute, BaseLock: time.Minute, MaxLock: time.Hour, StrikeMemory: time.Hour})
	uses := NewEPPCredentialUseRecorder(repo)
	svc := NewEPPAuthenticationService(repo, prefixHasher{}, lo, uses, 2)
	svc.now = func() time.Time { return eppSvcNow }
	return &authTest{svc: svc, repo: repo, uses: uses, lockout: lo, cert: cert}
}

func (a *authTest) cmd() commands.EPPLoginCommand {
	return commands.EPPLoginCommand{LoginID: "rar-001", Password: "first-pw-1", Source: authSource, Certificate: a.cert, CheckSource: true}
}

func TestEPPLoginSucceeds(t *testing.T) {
	at := newAuthTest(t)
	got, err := at.svc.Login(context.Background(), at.cmd())
	require.NoError(t, err)
	require.Equal(t, entities.ClIDType("rar-001"), got.ClID)
	require.Equal(t, entities.EPPLoginRoleTransform, got.Role)
	require.Equal(t, at.cert.NotAfter.UTC(), got.CertificateNotAfter)
	require.Len(t, got.CredentialIDs(), 3)
	require.False(t, got.PasswordReplaced())
}

func TestEPPLoginRefusals(t *testing.T) {
	for name, tc := range map[string]struct {
		change func(*commands.EPPLoginCommand)
		want   error
	}{
		"unknown login":  {func(c *commands.EPPLoginCommand) { c.LoginID = "nobody-here" }, entities.ErrEPPUnknownLogin},
		"wrong password": {func(c *commands.EPPLoginCommand) { c.Password = "wrong-pw-1" }, entities.ErrEPPPasswordMismatch},
		"wrong source":   {func(c *commands.EPPLoginCommand) { c.Source = netip.MustParseAddr("198.51.100.1") }, entities.ErrEPPSourceNotAllowed},
		"no certificate": {func(c *commands.EPPLoginCommand) { c.Certificate = nil }, entities.ErrEPPCertificateMissing},
	} {
		t.Run(name, func(t *testing.T) {
			at := newAuthTest(t)
			cmd := at.cmd()
			tc.change(&cmd)
			_, err := at.svc.Login(context.Background(), cmd)
			require.ErrorIs(t, err, tc.want)
			require.ErrorIs(t, err, entities.ErrEPPAuthentication)
		})
	}
}

func TestEPPLoginLockoutCountsOnlyAdmittedFailures(t *testing.T) {
	at := newAuthTest(t)
	ctx := context.Background()

	// A stranger without an allowed source or a pinned certificate cannot
	// lock the login out, however often they try.
	stranger := at.cmd()
	stranger.Source = netip.MustParseAddr("198.51.100.1")
	stranger.Password = "guess-pw-1"
	for i := 0; i < 10; i++ {
		_, err := at.svc.Login(ctx, stranger)
		require.ErrorIs(t, err, entities.ErrEPPSourceNotAllowed)
	}
	_, err := at.svc.Login(ctx, at.cmd())
	require.NoError(t, err)

	wrong := at.cmd()
	wrong.Password = "wrong-pw-1"
	for i := 0; i < 3; i++ {
		_, err := at.svc.Login(ctx, wrong)
		require.ErrorIs(t, err, entities.ErrEPPPasswordMismatch)
	}
	_, err = at.svc.Login(ctx, at.cmd())
	require.ErrorIs(t, err, entities.ErrEPPLoginLocked, "even the right password is refused while locked")

	// The lockout is per source: the same login from another allowed
	// address is not locked.
	elsewhere := at.cmd()
	elsewhere.Source = netip.MustParseAddr("192.0.2.8")
	_, err = at.svc.Login(ctx, elsewhere)
	require.NoError(t, err)
}

func TestEPPLoginSuccessClearsFailures(t *testing.T) {
	at := newAuthTest(t)
	ctx := context.Background()
	wrong := at.cmd()
	wrong.Password = "wrong-pw-1"
	for i := 0; i < 2; i++ {
		_, _ = at.svc.Login(ctx, wrong)
	}
	_, err := at.svc.Login(ctx, at.cmd())
	require.NoError(t, err)
	for i := 0; i < 2; i++ {
		_, _ = at.svc.Login(ctx, wrong)
	}
	_, err = at.svc.Login(ctx, at.cmd())
	require.NoError(t, err, "two failures after a success do not reach the threshold of three")
}

func TestEPPLoginNewPassword(t *testing.T) {
	at := newAuthTest(t)
	ctx := context.Background()
	cmd := at.cmd()
	cmd.NewPassword = "second-pw-2"
	got, err := at.svc.Login(ctx, cmd)
	require.NoError(t, err)
	require.True(t, got.PasswordReplaced())
	before, err := at.repo.GetAccess(ctx, "rar-001")
	require.NoError(t, err)
	presented := before.Logins[0].Passwords[0]
	require.Equal(t, presented.ID, got.PasswordID, "the session is tied to the password it presented")
	require.Contains(t, got.CredentialIDs(), got.ReplacementPasswordID, "and to the one it set")

	require.NoError(t, at.uses.Flush(ctx))
	after, err := at.repo.GetAccess(ctx, "rar-001")
	require.NoError(t, err)
	require.NotNil(t, after.Logins[0].Passwords[0].LastUsedAt, "the presented password was used")
	require.Nil(t, after.Logins[0].Passwords[1].LastUsedAt, "the new one has not been yet")

	_, err = at.svc.Login(ctx, at.cmd())
	require.ErrorIs(t, err, entities.ErrEPPPasswordMismatch, "the old password stops working at once")
	next := at.cmd()
	next.Password = "second-pw-2"
	_, err = at.svc.Login(ctx, next)
	require.NoError(t, err)

	events := at.repo.Events()
	last := events[len(events)-1]
	require.Equal(t, entities.EventEPPPasswordReplaced, last.Type)
	require.Equal(t, "domain-os/epp", last.Source)
	require.Equal(t, "rar-001", last.Actor)
}

func TestEPPLoginRefusesABadNewPassword(t *testing.T) {
	at := newAuthTest(t)
	cmd := at.cmd()
	cmd.NewPassword = "x"
	_, err := at.svc.Login(context.Background(), cmd)
	require.ErrorIs(t, err, entities.ErrInvalidEPPPassword)
	_, err = at.svc.Login(context.Background(), at.cmd())
	require.NoError(t, err, "the old password still works")
}

func TestEPPLoginRecordsUsesCoalesced(t *testing.T) {
	at := newAuthTest(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := at.svc.Login(ctx, at.cmd())
		require.NoError(t, err)
	}
	before := len(at.repo.Events())
	require.NoError(t, at.uses.Flush(ctx))
	require.Len(t, at.repo.Events(), before, "recording use writes no events")

	a, err := at.repo.GetAccess(ctx, "rar-001")
	require.NoError(t, err)
	for _, lc := range []*entities.EPPCredentialLifecycle{
		&a.Logins[0].Passwords[0].EPPCredentialLifecycle, &a.CertificatePins[0].EPPCredentialLifecycle, &a.AllowedNetworks[0].EPPCredentialLifecycle,
	} {
		require.NotNil(t, lc.LastUsedAt)
		require.Equal(t, "192.0.2.7", lc.LastUsedFrom)
	}
}

func TestEPPLoginStoreOutage(t *testing.T) {
	at := newAuthTest(t)
	at.repo.Fail(errors.New("connection refused"))
	_, err := at.svc.Login(context.Background(), at.cmd())
	require.ErrorIs(t, err, entities.ErrEPPCredentialsUnavailable)
	require.NotErrorIs(t, err, entities.ErrEPPAuthentication, "an outage is not a failed login")
}

func TestEPPUseRecorderKeepsUsesWhenAFlushFails(t *testing.T) {
	at := newAuthTest(t)
	ctx := context.Background()
	_, err := at.svc.Login(ctx, at.cmd())
	require.NoError(t, err)
	at.repo.Fail(errors.New("down"))
	require.Error(t, at.uses.Flush(ctx))
	at.repo.Fail(nil)
	require.NoError(t, at.uses.Flush(ctx))
	a, err := at.repo.GetAccess(ctx, "rar-001")
	require.NoError(t, err)
	require.NotNil(t, a.AllowedNetworks[0].LastUsedAt, "the failed flush's uses were written by the next one")
}

var _ repositories.EPPCredentialRepository = (*memstore.EPPCredentials)(nil)
