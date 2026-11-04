package postgres

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func testCABundle(t *testing.T, now time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: "TMCH Test CA"}, NotBefore: now.AddDate(-1, 0, 0), NotAfter: now.AddDate(9, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// TestTMCHRepository runs the repository against the migrated test
// database, inside a transaction that is rolled back.
func TestTMCHRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewTMCHRepository(tx)
	now := time.Now().UTC().Truncate(time.Microsecond)
	gen := now.Add(-3 * time.Hour)
	event := func(set string) []entities.DomainEvent {
		return []entities.DomainEvent{entities.NewDomainEvent("test", "tmch."+set+".refreshed", set, "m", nil)}
	}
	eventsBefore := func() int64 {
		var n int64
		require.NoError(t, tx.Model(&DomainEventRecord{}).Where("type LIKE 'tmch.%'").Count(&n).Error)
		return n
	}

	t.Run("nothing fetched yet", func(t *testing.T) {
		status, err := repo.Status(ctx)
		require.NoError(t, err)
		assert.Nil(t, status.Certificates)
		assert.Nil(t, status.SMDRL)
		assert.Nil(t, status.DNL)
		_, err = repo.GetCertificates(ctx)
		assert.ErrorIs(t, err, entities.ErrTMCHDataMissing)
		revoked, err := repo.IsSMDRevoked(ctx, "0000001751373633628125-65535")
		require.NoError(t, err)
		assert.False(t, revoked)
		e, err := repo.LookupDNL(ctx, "example")
		require.NoError(t, err)
		assert.Nil(t, e)
	})

	t.Run("certificates", func(t *testing.T) {
		bundle, err := entities.NewTMCHCertificates(testCABundle(t, now), "https://ca.example/tmch.crt", now)
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceCertificates(ctx, bundle, event(entities.TMCHSetCertificates)))
		got, err := repo.GetCertificates(ctx)
		require.NoError(t, err)
		require.Len(t, got.Certificates, 1)
		assert.Equal(t, bundle.Certificates[0], got.Certificates[0])
		assert.Equal(t, now, got.FetchedAt)
		assert.Equal(t, "https://ca.example/tmch.crt", got.Source)
		status, err := repo.Status(ctx)
		require.NoError(t, err)
		require.NotNil(t, status.Certificates)
		assert.Equal(t, 1, status.Certificates.Entries)
		require.NotNil(t, status.Certificates.ExpiresAt)
		assert.Equal(t, bundle.ExpiresAt(), *status.Certificates.ExpiresAt)
		assert.Equal(t, int64(1), eventsBefore(), "the event is written with the set")

		// A second bundle replaces the first, it does not add to it.
		second, err := entities.NewTMCHCertificates(testCABundle(t, now.Add(time.Second)), "https://ca.example/tmch.crt", now.Add(time.Minute))
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceCertificates(ctx, second, nil))
		got, err = repo.GetCertificates(ctx)
		require.NoError(t, err)
		require.Len(t, got.Certificates, 1)
		assert.Equal(t, second.Certificates[0].SerialNumber, got.Certificates[0].SerialNumber)
		assert.Equal(t, int64(1), eventsBefore(), "no events were given, none written")
	})

	t.Run("smdrl", func(t *testing.T) {
		list, err := entities.NewSMDRevocationList(1, gen, []entities.SMDRevocation{
			{SMDID: "0000001751373633628125-65535", RevokedAt: gen},
			{SMDID: "0000001681375789102250-65535", RevokedAt: gen.Add(-time.Hour)},
		}, "https://ry.example/smdrl-latest.csv", now)
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceSMDRL(ctx, list, event(entities.TMCHSetSMDRL)))
		revoked, err := repo.IsSMDRevoked(ctx, " 0000001751373633628125-65535")
		require.NoError(t, err)
		assert.True(t, revoked)
		revoked, err = repo.IsSMDRevoked(ctx, "0000009999999999999999-65535")
		require.NoError(t, err)
		assert.False(t, revoked)
		status, err := repo.Status(ctx)
		require.NoError(t, err)
		require.NotNil(t, status.SMDRL)
		assert.Equal(t, 2, status.SMDRL.Entries)
		assert.Equal(t, gen, status.SMDRL.GeneratedAt)
		assert.Nil(t, status.SMDRL.ExpiresAt)

		// The next generation drops a revocation: it is gone, not kept.
		next, err := entities.NewSMDRevocationList(1, gen.Add(12*time.Hour), []entities.SMDRevocation{{SMDID: "0000001751373633628125-65535", RevokedAt: gen}}, "https://ry.example/smdrl-latest.csv", now.Add(time.Minute))
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceSMDRL(ctx, next, event(entities.TMCHSetSMDRL)))
		revoked, err = repo.IsSMDRevoked(ctx, "0000001681375789102250-65535")
		require.NoError(t, err)
		assert.False(t, revoked)
		status, err = repo.Status(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, status.SMDRL.Entries)
		assert.Equal(t, gen.Add(12*time.Hour), status.SMDRL.GeneratedAt)

		empty, err := entities.NewSMDRevocationList(1, gen.Add(24*time.Hour), nil, "https://ry.example/smdrl-latest.csv", now.Add(2*time.Minute))
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceSMDRL(ctx, empty, nil), "an empty list is stored: nothing revoked")
		revoked, err = repo.IsSMDRevoked(ctx, "0000001751373633628125-65535")
		require.NoError(t, err)
		assert.False(t, revoked)
	})

	t.Run("dnl", func(t *testing.T) {
		entries := make([]entities.DNLEntry, 0, tmchBatch+5)
		for i := 0; i < tmchBatch+5; i++ {
			entries = append(entries, entities.DNLEntry{Label: "label-" + big.NewInt(int64(i)).String(), LookupKey: "2026110400/1/1/1/key" + big.NewInt(int64(i)).String(), InsertedAt: gen})
		}
		entries = append(entries, entities.DNLEntry{Label: "Example-One", LookupKey: "2026110400/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF", InsertedAt: gen})
		list, err := entities.NewDNLList(1, gen, entries, "https://ry.example/dnl-latest.csv", now)
		require.NoError(t, err)
		require.NoError(t, repo.ReplaceDNL(ctx, list, event(entities.TMCHSetDNL)))
		e, err := repo.LookupDNL(ctx, "EXAMPLE-ONE")
		require.NoError(t, err)
		require.NotNil(t, e)
		assert.Equal(t, "example-one", e.Label)
		assert.Equal(t, "2026110400/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF", e.LookupKey)
		assert.Equal(t, gen, e.InsertedAt)
		e, err = repo.LookupDNL(ctx, "label-1003")
		require.NoError(t, err)
		require.NotNil(t, e, "past the first batch")
		e, err = repo.LookupDNL(ctx, "unlisted")
		require.NoError(t, err)
		assert.Nil(t, e)
		status, err := repo.Status(ctx)
		require.NoError(t, err)
		require.NotNil(t, status.DNL)
		assert.Equal(t, tmchBatch+6, status.DNL.Entries)
		assert.Equal(t, "https://ry.example/dnl-latest.csv", status.DNL.Source)
		assert.Equal(t, int64(4), eventsBefore())
	})
}
