package services_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
)

// The two shared credentials the JISC importers used to stamp on every object. Both sit in a public
// repository, so neither may reach the Admin API again.
var retiredImportAuthInfos = []string{"escr0W1mP*rt", "Import-2026-Data!"}

// fakeAdminAPI stands in for the Admin API ImportToAdminAPI talks to, and records what it is sent.
type fakeAdminAPI struct {
	mu               sync.Mutex
	existingContacts []entities.Contact
	contacts         []commands.CreateContactCommand
	domains          []commands.CreateDomainCommand
}

func (f *fakeAdminAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	list := func(data any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"Data": data, "Meta": map[string]string{"PageCursor": ""}})
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/registrars":
		list([]entities.RegistrarListItem{{ClID: "rar-a", Name: "Registrar A"}})
	case r.Method == http.MethodGet && r.URL.Path == "/hosts":
		list([]entities.Host{})
	case r.Method == http.MethodGet && r.URL.Path == "/contacts":
		list(f.existingContacts)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/tlds/"):
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(entities.TLD{Name: "ac.uk", AllowEscrowImport: true})
	case r.Method == http.MethodPost && r.URL.Path == "/contacts/bulk":
		var cmds []commands.CreateContactCommand
		_ = json.NewDecoder(r.Body).Decode(&cmds)
		f.contacts = append(f.contacts, cmds...)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPost && r.URL.Path == "/domains/bulk":
		var cmds []commands.CreateDomainCommand
		_ = json.NewDecoder(r.Body).Decode(&cmds)
		f.domains = append(f.domains, cmds...)
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPost && (r.URL.Path == "/registrars/bulk" || r.URL.Path == "/hosts/bulk"):
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/hostname/"):
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func runAdminAPIImport(t *testing.T, api *fakeAdminAPI) {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	jsonPath := writeJiscJSON(t,
		jiscRecord(1, "one.ac.uk", 100, "Registrar A"),
		jiscRecord(2, "two.ac.uk", 100, "Registrar A"),
		jiscRecord(3, "three.ac.uk", 100, "Registrar A"),
	)
	require.NoError(t, services.NewJiscService().ImportToAdminAPI(jsonPath, srv.URL, "test-token"))
}

func requireNoRetiredAuthInfo(t *testing.T, authInfos []string) {
	t.Helper()
	seen := map[string]struct{}{}
	for _, a := range authInfos {
		for _, retired := range retiredImportAuthInfos {
			require.NotEqual(t, retired, a, "a retired shared authInfo was sent to the Admin API")
		}
		require.NoError(t, entities.AuthInfoType(a).Validate(), "authInfo %q is not compliant", a)
		seen[a] = struct{}{}
	}
	require.Len(t, seen, len(authInfos), "an authInfo was shared between objects")
}

func TestJiscImportToAdminAPI_GeneratesAuthInfoPerObject(t *testing.T) {
	api := &fakeAdminAPI{}
	runAdminAPIImport(t, api)

	require.Len(t, api.contacts, 3)
	require.Len(t, api.domains, 3)
	var authInfos []string
	for _, c := range api.contacts {
		authInfos = append(authInfos, c.AuthInfo)
	}
	for _, d := range api.domains {
		authInfos = append(authInfos, d.AuthInfo)
	}
	requireNoRetiredAuthInfo(t, authInfos) // unique across contacts and domains together
}

// The importer is safe to re-run because it skips contacts that already exist. The authInfo must be minted
// only for a contact that is actually created, so a re-run does not send a fresh credential for an existing one.
func TestJiscImportToAdminAPI_ExistingContactIsNotSentAgain(t *testing.T) {
	// Record 1 has no registrant ID in the export (-1), so its contact is cd-<DomainID> = cd-1.
	api := &fakeAdminAPI{existingContacts: []entities.Contact{{ID: "cd-1"}}}
	runAdminAPIImport(t, api)

	require.Len(t, api.contacts, 2, "the contact that already exists must not be sent again")
	for _, c := range api.contacts {
		require.NotEqual(t, "cd-1", c.ID)
	}
	var authInfos []string
	for _, c := range api.contacts {
		authInfos = append(authInfos, c.AuthInfo)
	}
	requireNoRetiredAuthInfo(t, authInfos)
}
