package rdap

// ICANN RST v2.0 StandardRDAP conformance, per ADR 0015.
//
// Each RST case will be one test named after its case ID. Until a case can be
// tested it is listed in rstPendingCases with the phase of
// docs/rdap/ARCHITECTURE.md that delivers it, so `make rst-rdap` reads as a
// readiness scoreboard from the first PR. TestRST_RDAPCatalogueIsComplete
// fails if the pinned spec has a case this file does not account for.
//
// The spec is pinned in testdata/rst; see testdata/rst/PROVENANCE.md for the
// release and how to move to a newer one.

import (
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// rstPending skips a requirement the server does not meet yet. reason names
// the phase of docs/rdap/ARCHITECTURE.md that delivers it.
func rstPending(t *testing.T, reason string) {
	t.Helper()
	t.Skipf("RST pending: %s", reason)
}

// rstPendingCases are the StandardRDAP cases with no implementation to test
// yet, each with the phase that delivers it. TestRST_RDAPCatalogueIsComplete
// keeps this list and the pinned spec in step.
var rstPendingCases = map[string]string{
	"rdap-01": "Phase 2, domain lookup; graded by the conformance tool from Phase 6",
	"rdap-02": "Phase 3, nameserver lookup; graded by the conformance tool from Phase 6",
	"rdap-03": "Phase 3, registrar entity lookup; graded by the conformance tool from Phase 6",
	"rdap-04": "Phase 1, /help; graded by the conformance tool from Phase 6",
	"rdap-05": "Phase 2, HEAD on a domain",
	"rdap-06": "Phase 3, HEAD on a nameserver",
	"rdap-07": "Phase 3, HEAD on an entity",
	"rdap-08": "Phase 1, 404 for a domain that does not exist",
	"rdap-09": "Phase 1, 404 for a nameserver that does not exist",
	"rdap-10": "Phase 1, 404 for an entity that does not exist",
	"rdap-91": "Phase 1, TLS on the node; verified per deployment in Phase 7",
	"rdap-92": "Phase 7, identical responses over IPv4 and IPv6 (per deployment)",
}

func TestRST_RDAPPendingCases(t *testing.T) {
	ids := make([]string, 0, len(rstPendingCases))
	for id := range rstPendingCases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) { rstPending(t, rstPendingCases[id]) })
	}
}

// rstImplementedCases have their own TestRST_ function in this file.
var rstImplementedCases = []string{}

// TestRST_RDAPCatalogueIsComplete fails when the pinned spec and this file
// disagree: a case in the spec that is neither implemented nor pending, or a
// case here that the spec no longer has. It is what makes moving to a newer
// RST release (testdata/rst/PROVENANCE.md) surface every new case.
func TestRST_RDAPCatalogueIsComplete(t *testing.T) {
	raw, err := os.ReadFile("testdata/rst/rdap-cases.yaml")
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.NotEmpty(t, spec)

	accounted := map[string]bool{}
	for _, id := range rstImplementedCases {
		accounted[id] = true
	}
	for id := range rstPendingCases {
		require.False(t, accounted[id], "%s is both implemented and pending", id)
		accounted[id] = true
	}

	for id := range spec {
		assert.True(t, accounted[id], "RST case %s is in the pinned spec but not in rst_rdap_test.go", id)
	}
	for id := range accounted {
		_, ok := spec[id]
		assert.True(t, ok, "%s is accounted for here but is not in the pinned spec", id)
	}
}
