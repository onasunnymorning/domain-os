package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	_ "modernc.org/sqlite"
)

// retiredEscrowAuthInfo is the constant the import used to stamp on every domain and contact.
// It is committed to a public repository, so it must never reach the Admin API again.
const retiredEscrowAuthInfo = "escr0W1mP*rt"

// importPageSize mirrors the page size of importDomainsChunked and importContactsChunked, so the
// tests below cross a page boundary.
const importPageSize = 2000

// adminAPIRecorder stands in for the Admin API and records what the import sends it.
type adminAPIRecorder struct {
	mu sync.Mutex

	// failBulk makes the bulk endpoints answer 409, which sends the import down its per-item fallback.
	failBulk bool

	domainBulkCalls    [][]commands.CreateDomainCommand
	domainSingleCalls  []commands.CreateDomainCommand
	contactBulkCalls   [][]commands.CreateContactCommand
	contactSingleCalls []commands.CreateContactCommand
}

func (r *adminAPIRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status := http.StatusCreated
	switch req.URL.Path {
	case "/domains/bulk":
		var cmds []commands.CreateDomainCommand
		_ = json.NewDecoder(req.Body).Decode(&cmds)
		r.domainBulkCalls = append(r.domainBulkCalls, cmds)
		if r.failBulk {
			status = http.StatusConflict
		}
	case "/domains":
		var cmd commands.CreateDomainCommand
		_ = json.NewDecoder(req.Body).Decode(&cmd)
		r.domainSingleCalls = append(r.domainSingleCalls, cmd)
	case "/contacts/bulk":
		var cmds []commands.CreateContactCommand
		_ = json.NewDecoder(req.Body).Decode(&cmds)
		r.contactBulkCalls = append(r.contactBulkCalls, cmds)
		if r.failBulk {
			status = http.StatusConflict
		}
	case "/contacts":
		var cmd commands.CreateContactCommand
		_ = json.NewDecoder(req.Body).Decode(&cmd)
		r.contactSingleCalls = append(r.contactSingleCalls, cmd)
	default:
		status = http.StatusNotFound
	}
	w.WriteHeader(status)
}

// startAdminAPI points the activities at a recording fake Admin API for the length of the test.
func startAdminAPI(t *testing.T, failBulk bool) *adminAPIRecorder {
	t.Helper()
	rec := &adminAPIRecorder{failBulk: failBulk}
	srv := httptest.NewServer(rec)
	t.Cleanup(srv.Close)

	prev := BASEURL
	BASEURL = srv.URL
	t.Cleanup(func() { BASEURL = prev })
	t.Setenv("ADMIN_TOKEN", "test-token") // the bearer token fallback when Auth0 is not configured
	return rec
}

// runAsActivity runs fn inside a Temporal activity context, which the import helpers need for heartbeats.
func runAsActivity(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	env.RegisterActivityWithOptions(fn, activity.RegisterOptions{Name: "import-under-test"})
	_, err := env.ExecuteActivity("import-under-test")
	require.NoError(t, err)
}

func stagedDBForAuthInfo(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "staged.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE domains (name TEXT PRIMARY KEY, registrant TEXT, clid TEXT, crrr TEXT, crdate TEXT, exdate TEXT, uprr TEXT, uname TEXT, originalname TEXT);
		CREATE TABLE domain_statuses (domain_name TEXT, status TEXT);
		CREATE TABLE domain_rgp_statuses (domain_name TEXT, rgp_status TEXT);
		CREATE TABLE contacts (id TEXT PRIMARY KEY, roid TEXT, voice TEXT, fax TEXT, email TEXT, clid TEXT, crrr TEXT, crdate TEXT, uprr TEXT, "update" TEXT);
		CREATE TABLE contact_statuses (contact_id TEXT, status TEXT);
		CREATE TABLE contact_postal_info (contact_id TEXT, type TEXT, name TEXT, org TEXT, street1 TEXT, street2 TEXT, street3 TEXT, city TEXT, state_province TEXT, postal_code TEXT, country_code TEXT);
	`)
	require.NoError(t, err)
	return db
}

// seedDomains inserts n domains for a mapped registrar, plus one for an unmapped registrar that sorts last.
func seedDomains(t *testing.T, db *sql.DB, n int) {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		_, err := tx.Exec(`INSERT INTO domains (name, clid) VALUES (?, 'escrowRar')`, fmt.Sprintf("d%05d.radio", i))
		require.NoError(t, err)
	}
	_, err = tx.Exec(`INSERT INTO domains (name, clid) VALUES ('zz-unmapped.radio', 'unknownRar')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

func seedContacts(t *testing.T, db *sql.DB, n int) {
	t.Helper()
	tx, err := db.Begin()
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		_, err := tx.Exec(`INSERT INTO contacts (id, email, clid) VALUES (?, 'someone@example.invalid', 'escrowRar')`, fmt.Sprintf("c%05d", i))
		require.NoError(t, err)
	}
	_, err = tx.Exec(`INSERT INTO contacts (id, email, clid) VALUES ('zz-unmapped', 'x@example.invalid', 'unknownRar')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

// requireGeneratedAuthInfos asserts every authInfo is compliant, is not the retired constant, and
// that none is shared with another object.
func requireGeneratedAuthInfos(t *testing.T, authInfos []string) {
	t.Helper()
	seen := make(map[string]struct{}, len(authInfos))
	for _, a := range authInfos {
		require.NotEqual(t, retiredEscrowAuthInfo, a, "the retired shared authInfo was sent to the Admin API")
		require.NoError(t, entities.AuthInfoType(a).Validate(), "authInfo %q is not compliant", a)
		seen[a] = struct{}{}
	}
	require.Len(t, seen, len(authInfos), "an authInfo was shared between objects")
}

func TestImportDomainsChunked_GeneratesAuthInfoPerDomain(t *testing.T) {
	db := stagedDBForAuthInfo(t)
	seedDomains(t, db, importPageSize+50)
	api := startAdminAPI(t, false)

	var events []ReportEvent
	counts, tallies := map[string]int64{}, map[string]int64{}
	runAsActivity(t, func(ctx context.Context) error {
		a := &EscrowImportActivities{}
		return a.importDomainsChunked(ctx, db, "radio", counts, map[string]string{"escrowRar": "mappedRar"}, &events, tallies, nil, "")
	})

	// Batching is untouched: two pages, one bulk request each, and nothing falls back to per-item calls.
	require.Len(t, api.domainBulkCalls, 2)
	require.Len(t, api.domainBulkCalls[0], importPageSize)
	require.Len(t, api.domainBulkCalls[1], 50)
	require.Empty(t, api.domainSingleCalls)
	require.Equal(t, int64(importPageSize+50), counts["domains_imported"])

	var authInfos []string
	for _, page := range api.domainBulkCalls {
		for _, c := range page {
			require.NotEqual(t, "zz-unmapped.radio", c.Name, "a domain of an unmapped registrar must be skipped")
			authInfos = append(authInfos, c.AuthInfo)
		}
	}
	require.Len(t, authInfos, importPageSize+50)
	requireGeneratedAuthInfos(t, authInfos) // unique across the page boundary too
	require.Equal(t, int64(1), tallies["domains_skipped_unmapped"])
}

// When the bulk request is rejected the import retries the same page one object at a time. A domain
// must arrive with the authInfo it was first sent with, not a freshly generated one.
func TestImportDomainsChunked_FallbackReusesTheSameAuthInfo(t *testing.T) {
	db := stagedDBForAuthInfo(t)
	seedDomains(t, db, 30)
	api := startAdminAPI(t, true)

	var events []ReportEvent
	runAsActivity(t, func(ctx context.Context) error {
		a := &EscrowImportActivities{}
		return a.importDomainsChunked(ctx, db, "radio", map[string]int64{}, map[string]string{"escrowRar": "mappedRar"}, &events, map[string]int64{}, nil, "")
	})

	require.Len(t, api.domainBulkCalls, 1)
	require.Len(t, api.domainSingleCalls, 30)
	viaBulk := make(map[string]string, 30)
	for _, c := range api.domainBulkCalls[0] {
		viaBulk[c.Name] = c.AuthInfo
	}
	var authInfos []string
	for _, c := range api.domainSingleCalls {
		require.Equal(t, viaBulk[c.Name], c.AuthInfo, "%s changed authInfo between the bulk request and its fallback", c.Name)
		authInfos = append(authInfos, c.AuthInfo)
	}
	requireGeneratedAuthInfos(t, authInfos)
}

func TestImportContactsChunked_GeneratesAuthInfoPerContact(t *testing.T) {
	db := stagedDBForAuthInfo(t)
	seedContacts(t, db, importPageSize+50)
	api := startAdminAPI(t, false)

	var events []ReportEvent
	counts, tallies := map[string]int64{}, map[string]int64{}
	runAsActivity(t, func(ctx context.Context) error {
		a := &EscrowImportActivities{}
		return a.importContactsChunked(ctx, db, counts, map[string]string{"escrowRar": "mappedRar"}, &events, tallies, nil, "")
	})

	require.Len(t, api.contactBulkCalls, 2)
	require.Len(t, api.contactBulkCalls[0], importPageSize)
	require.Len(t, api.contactBulkCalls[1], 50)
	require.Empty(t, api.contactSingleCalls)
	require.Equal(t, int64(importPageSize+50), counts["contacts_imported"])

	var authInfos []string
	for _, page := range api.contactBulkCalls {
		for _, c := range page {
			require.NotEqual(t, "zz-unmapped", c.ID, "a contact of an unmapped registrar must be skipped")
			authInfos = append(authInfos, c.AuthInfo)
		}
	}
	require.Len(t, authInfos, importPageSize+50)
	requireGeneratedAuthInfos(t, authInfos)
	require.Equal(t, int64(1), tallies["contacts_skipped_unmapped"])
}

func TestImportContactsChunked_FallbackReusesTheSameAuthInfo(t *testing.T) {
	db := stagedDBForAuthInfo(t)
	seedContacts(t, db, 30)
	api := startAdminAPI(t, true)

	var events []ReportEvent
	runAsActivity(t, func(ctx context.Context) error {
		a := &EscrowImportActivities{}
		return a.importContactsChunked(ctx, db, map[string]int64{}, map[string]string{"escrowRar": "mappedRar"}, &events, map[string]int64{}, nil, "")
	})

	require.Len(t, api.contactBulkCalls, 1)
	require.Len(t, api.contactSingleCalls, 30)
	viaBulk := make(map[string]string, 30)
	for _, c := range api.contactBulkCalls[0] {
		viaBulk[c.ID] = c.AuthInfo
	}
	var authInfos []string
	for _, c := range api.contactSingleCalls {
		require.Equal(t, viaBulk[c.ID], c.AuthInfo, "%s changed authInfo between the bulk request and its fallback", c.ID)
		authInfos = append(authInfos, c.AuthInfo)
	}
	requireGeneratedAuthInfos(t, authInfos)
}
