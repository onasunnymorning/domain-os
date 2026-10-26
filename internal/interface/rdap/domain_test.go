package rdap

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// update rewrites the golden responses: go test ./internal/interface/rdap/ -run TestDomainGolden -update
var update = flag.Bool("update", false, "rewrite testdata/golden")

var goldenNow = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

func registrant() *entities.Contact {
	return &entities.Contact{
		ID: "reg-1", RoID: "11_CONT-TEST",
		PostalInfo: [2]*entities.ContactPostalInfo{{
			Type: "int", Name: "Jane Registrant", Org: "Example Org",
			Address: &entities.Address{Street1: "1 Main St", City: "Springfield", StateProvince: "IL", PostalCode: "62701", CountryCode: "US"},
		}},
		Voice: "+1.5555551234", Email: "jane@example.test",
	}
}

func tech() *entities.Contact {
	return &entities.Contact{
		ID: "tech-1", RoID: "12_CONT-TEST",
		PostalInfo: [2]*entities.ContactPostalInfo{{
			Type: "int", Name: "Tom Tech", Address: &entities.Address{City: "Paris", CountryCode: "FR"},
		}},
		Voice: "+33.155550000", Email: "tom@example.test",
	}
}

// fullDomain is an IDN gTLD domain with glue, an external nameserver, DS
// records, a transfer and a registrar with an RDAP base URL.
func fullDomain() *interfaces.RDAPDomain {
	d := &entities.Domain{
		RoID: "100_DOM-TEST", Name: "xn--bcher-kva.example", UName: "bücher.example", TLDName: "example",
		ClID: "rar-1", RegistrantID: "reg-1", TechID: "tech-1", AdminID: "reg-1",
		CreatedAt:     time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		ExpiryDate:    time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC),
		TransferredAt: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		Status:        entities.DomainStatus{ClientTransferProhibited: true, ServerDeleteProhibited: true},
		RGPStatus:     entities.DomainRGPStatus{RenewPeriodEnd: goldenNow.Add(24 * time.Hour)},
		Hosts: []*entities.Host{
			{RoID: "200_HOST-TEST", Name: "ns1.xn--bcher-kva.example", InBailiwick: true, Addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("2001:db8::1")}},
			{RoID: "201_HOST-TEST", Name: "ns.dns.example.net"},
		},
		DSRecords: []entities.DSRecord{{KeyTag: 12345, Algorithm: 13, DigestType: 2, Digest: "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE4B9D4C3A1F2E9C9E7A8B6C5D"}},
	}
	return &interfaces.RDAPDomain{
		Domain: d,
		Registrar: &entities.Registrar{
			ClID: "rar-1", Name: "Example Registrar, Inc.", GurID: 9995,
			Voice: "+1.5555550000", Email: "abuse@registrar.example", URL: "https://registrar.example",
			RdapBaseURL: "https://rdap.registrar.example",
		},
		TLDType:       entities.TLDTypeGTLD,
		ContactPolicy: ptr(entities.NewContactDataPolicy()),
		Publication:   rdappolicy.GTLDRegistrationDataPolicy2024(),
		Contacts:      map[rdappolicy.Role]*entities.Contact{rdappolicy.RoleRegistrant: registrant(), rdappolicy.RoleTech: tech(), rdappolicy.RoleAdmin: registrant()},
	}
}

// thickToThin is fullDomain after its TLD's policy stopped collecting
// contacts, with the old contacts still linked.
func thickToThin() *interfaces.RDAPDomain {
	rec := fullDomain()
	rec.Domain.Name, rec.Domain.UName = "moved.example", ""
	rec.ContactPolicy = &entities.ContactDataPolicy{
		RegistrantContactDataPolicy: entities.ContactDataPolicyTypeProhibited,
		TechContactDataPolicy:       entities.ContactDataPolicyTypeProhibited,
		AdminContactDataPolicy:      entities.ContactDataPolicyTypeProhibited,
		BillingContactDataPolicy:    entities.ContactDataPolicyTypeProhibited,
	}
	return rec
}

// ccTLD is a ccTLD domain under cctld-minimal, with the registrant disclosing
// everything and a registrar with no IANA ID.
func ccTLD() *interfaces.RDAPDomain {
	c := registrant()
	c.Disclose = *entities.NewDiscloseStruct(true)
	return &interfaces.RDAPDomain{
		Domain: &entities.Domain{
			RoID: "300_DOM-TEST", Name: "example.cc", TLDName: "cc", ClID: "local-rar", RegistrantID: "reg-1",
			CreatedAt: time.Date(2020, 5, 5, 0, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2020, 5, 5, 0, 0, 0, 0, time.UTC),
			ExpiryDate: time.Date(2027, 5, 5, 0, 0, 0, 0, time.UTC), Status: entities.DomainStatus{OK: true},
		},
		Registrar:     &entities.Registrar{ClID: "local-rar", Name: "Local Registrar"},
		TLDType:       entities.TLDTypeCCTLD,
		ContactPolicy: ptr(entities.NewContactDataPolicy()),
		Publication:   rdappolicy.CCTLDMinimal(),
		Contacts:      map[rdappolicy.Role]*entities.Contact{rdappolicy.RoleRegistrant: c},
	}
}

func goldenServer(t *testing.T) *testServer {
	t.Helper()
	cfg := testConfig("https://rdap.example.test/")
	srv, err := NewServer(Options{Config: cfg, Lookups: &fakeLookups{}, Now: func() time.Time { return goldenNow }})
	require.NoError(t, err)
	return &testServer{Server: srv}
}

func TestDomainGolden(t *testing.T) {
	ts := goldenServer(t)
	for name, rec := range map[string]*interfaces.RDAPDomain{
		"gtld-thick":         fullDomain(),
		"gtld-thick-to-thin": thickToThin(),
		"cctld-minimal":      ccTLD(),
	} {
		t.Run(name, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, "https://rdap.example.test/"+string(rec.Domain.TLDName)+"/domain/"+string(rec.Domain.Name), nil)
			require.NoError(t, err)
			got, err := json.MarshalIndent(ts.buildDomain(r, rec, rdappolicy.Anonymous()), "", "  ")
			require.NoError(t, err)
			got = append(got, '\n')

			path := filepath.Join("testdata", "golden", name+".json")
			if *update {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
				require.NoError(t, os.WriteFile(path, got, 0o600))
			}
			want, err := os.ReadFile(path) // #nosec G304 -- test-only: a fixed path under testdata
			require.NoError(t, err, "run with -update to create %s", path)
			assert.Equal(t, string(want), string(got))
		})
	}
}

// decoded is a domain response as a client reads it.
func decoded(t *testing.T, ts *testServer, rec *interfaces.RDAPDomain) map[string]any {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "https://rdap.example.test/domain/"+string(rec.Domain.Name), nil)
	require.NoError(t, err)
	raw, err := json.Marshal(ts.buildDomain(r, rec, rdappolicy.Anonymous()))
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

func roles(resp map[string]any) []string {
	var out []string
	for _, e := range resp["entities"].([]any) {
		for _, r := range e.(map[string]any)["roles"].([]any) {
			out = append(out, r.(string))
		}
	}
	return out
}

func TestThickToThinShowsNoContactsAndRedactsNothing(t *testing.T) {
	resp := decoded(t, goldenServer(t), thickToThin())
	assert.Equal(t, []string{"registrar"}, roles(resp))
	assert.NotContains(t, resp, "redacted", "a role that is not collected is absent, not redacted")
	raw, _ := json.Marshal(resp)
	for _, leak := range []string{"Jane", "jane@", "Tom", "tom@", "62701", "Springfield"} {
		assert.NotContains(t, string(raw), leak)
	}
}

func TestGTLDThickRedactsWithheldFields(t *testing.T) {
	resp := decoded(t, goldenServer(t), fullDomain())
	assert.Equal(t, []string{"registrar", "registrant", "technical"}, roles(resp), "admin is not published under the RDP")
	raw, _ := json.Marshal(resp)
	for _, leak := range []string{"Jane Registrant", "jane@example.test", "1 Main St", "Springfield", "62701", "5555551234", "11_CONT-TEST", "Tom Tech", "tom@example.test"} {
		assert.NotContains(t, string(raw), leak, "withheld values never reach the response")
	}
	assert.Contains(t, string(raw), `"cc":"US"`)
	assert.Contains(t, string(raw), `"IL"`)

	var names []string
	for _, rd := range resp["redacted"].([]any) {
		names = append(names, rd.(map[string]any)["name"].(map[string]any)["type"].(string))
	}
	for _, want := range []string{"Registry Registrant ID", "Registrant Name", "Registrant Street", "Registrant City", "Registrant Postal Code", "Registrant Phone", "Registrant Email", "Registrant Organization", "Tech Name", "Tech Email"} {
		assert.Contains(t, names, want)
	}
	assert.NotContains(t, names, "Registrant Fax", "an absent field is not listed as redacted")
}

func TestDomainStatuses(t *testing.T) {
	d := &entities.Domain{Status: entities.DomainStatus{PendingDelete: true}, RGPStatus: entities.DomainRGPStatus{RedemptionPeriodEnd: goldenNow.Add(time.Hour)}}
	assert.Equal(t, []string{"pending delete", "redemption period"}, statuses(d, goldenNow))
	assert.Equal(t, []string{"active"}, statuses(&entities.Domain{Status: entities.DomainStatus{OK: true}}, goldenNow))
	assert.Equal(t, []string{"active"}, statuses(&entities.Domain{}, goldenNow), "a domain with no status is active")
	for epp := range domainStatus {
		assert.NotEmpty(t, domainStatus[epp])
	}
}

func TestDomainOverHTTP(t *testing.T) {
	rec := fullDomain()
	ts := startServer(t, testConfig("https://rdap.example.test/"), &fakeLookups{domains: map[string]*interfaces.RDAPDomain{string(rec.Domain.Name): rec}})

	resp, body := ts.do(t, http.MethodGet, "example/domain/bücher.example")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "xn--bcher-kva.example", got["ldhName"])
	assert.Contains(t, got["rdapConformance"], "redacted")

	head, headBody := ts.do(t, http.MethodHead, "example/domain/xn--bcher-kva.example")
	assert.Equal(t, http.StatusOK, head.StatusCode)
	assert.Empty(t, headBody)
	assert.Equal(t, "*", head.Header.Get("Access-Control-Allow-Origin"))

	metrics := opsGet(ts.Server, "/metrics").Body.String()
	assert.Contains(t, metrics, `rdap_redactions_total{field="name",method="emptyValue",tld="example"}`)
	assert.False(t, bytes.Contains([]byte(metrics), []byte("Jane")), "metrics never carry values")
}
