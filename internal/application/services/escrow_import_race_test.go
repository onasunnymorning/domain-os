package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Create* methods fan out over CONCURRENT_CLIENTS goroutines that all
// record into svc.Import. Before the importMu guard, both the counters and the
// Errors slice were written unsynchronised: counts were lost and the append
// could corrupt the slice. Run with -race.
func TestCreateContacts_ConcurrentImportAccountingIsRaceFree(t *testing.T) {
	const (
		mapped   = 300 // resolve through RegistrarMapping and get a 201
		unmapped = 200 // fail the mapping lookup and record a failure
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	originalBaseURL := BASE_URL
	BASE_URL = srv.URL
	defer func() { BASE_URL = originalBaseURL }()

	svc := &XMLEscrowService{
		RegistrarMapping: entities.RegistrarMapping{
			"known": entities.RdeRegistrarInfo{RegistrarClID: "known-clid"},
		},
	}

	cmds := make([]commands.CreateContactCommand, 0, mapped+unmapped)
	for i := 0; i < mapped; i++ {
		cmds = append(cmds, commands.CreateContactCommand{ID: "ok", ClID: "known"})
	}
	for i := 0; i < unmapped; i++ {
		cmds = append(cmds, commands.CreateContactCommand{ID: "bad", ClID: "missing"})
	}

	require.NoError(t, svc.CreateContacts(cmds, "test-token"))

	// Every command must be accounted for exactly once. Under the race these
	// totals came up short, because concurrent counter increments were lost.
	assert.Equal(t, mapped, svc.Import.Contacts.Created, "created count lost increments")
	assert.Equal(t, unmapped, svc.Import.Contacts.Failed, "failed count lost increments")
	assert.Len(t, svc.Import.Errors, unmapped, "Errors slice lost or duplicated entries")
	for _, e := range svc.Import.Errors {
		assert.True(t, strings.Contains(e, "not found in mapping"), "unexpected error recorded: %s", e)
	}
}

// A transport failure used to return an error that the worker goroutines threw
// away, so the object was never created, never counted as failed, and appeared
// nowhere in the import report — the run looked clean.
func TestCreateContacts_TransportFailureIsRecorded(t *testing.T) {
	const n = 50

	// Closed immediately: every request fails at client.Do.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()

	originalBaseURL := BASE_URL
	BASE_URL = srv.URL
	defer func() { BASE_URL = originalBaseURL }()

	svc := &XMLEscrowService{
		RegistrarMapping: entities.RegistrarMapping{
			"known": entities.RdeRegistrarInfo{RegistrarClID: "known-clid"},
		},
	}

	cmds := make([]commands.CreateContactCommand, 0, n)
	for i := 0; i < n; i++ {
		cmds = append(cmds, commands.CreateContactCommand{ID: "unreachable", ClID: "known"})
	}

	require.NoError(t, svc.CreateContacts(cmds, "test-token"))

	assert.Equal(t, 0, svc.Import.Contacts.Created)
	assert.Equal(t, n, svc.Import.Contacts.Failed, "transport failures must be counted, not dropped")
	assert.Len(t, svc.Import.Errors, n, "every transport failure must appear in the import report")
}
