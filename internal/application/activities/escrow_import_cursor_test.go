package activities

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

// resumedCursor runs ingestCursor for phase in an activity whose previous
// attempt heartbeated details (none when details is nil), the way a retry sees
// them.
func resumedCursor(t *testing.T, phase services.ImportPhase, details interface{}) services.ImportCursor {
	t.Helper()
	env := (&testsuite.WorkflowTestSuite{}).NewTestActivityEnvironment()
	if details != nil {
		env.SetHeartbeatDetails(details)
	}
	env.RegisterActivityWithOptions(func(ctx context.Context) (services.ImportCursor, error) {
		return ingestCursor(ctx, phase), nil
	}, activity.RegisterOptions{Name: "resume-under-test"})

	val, err := env.ExecuteActivity("resume-under-test")
	require.NoError(t, err)
	var got services.ImportCursor
	require.NoError(t, val.Get(&got))
	return got
}

func TestIngestCursor_FirstAttemptStartsAtTheTop(t *testing.T) {
	require.Equal(t, services.ImportCursor{}, resumedCursor(t, services.PhaseDomains, nil))
}

// What the importers heartbeat now comes back whole, LastClID included.
func TestIngestCursor_ResumesFromRecordedCursor(t *testing.T) {
	recorded := services.ImportCursor{Version: services.ImportCursorVersion, LastKey: "ns1.example", LastClID: "rar2", Processed: 10000, Total: 12345}
	require.Equal(t, recorded, resumedCursor(t, services.PhaseHosts, recorded))
}

// A workflow in flight across the deploy hands back what the old importers
// heartbeated: the cursor as preformatted JSON text. Taking that text as the
// key is the bug this replaces: "{" sorts after every name, so the resumed
// attempt found nothing left and skipped every remaining row.
//
// Decoded, a legacy contacts or NNDNs cursor is a staged key and resumes. A
// legacy domains cursor is a normalised name and a legacy hosts cursor has no
// clid; neither marks a boundary the query can resume from exactly, so those
// phases start over.
func TestIngestCursor_DecodesLegacyStringPayloads(t *testing.T) {
	for name, tc := range map[string]struct {
		phase   services.ImportPhase
		payload string
		want    services.ImportCursor
	}{
		"contacts": {
			phase:   services.PhaseContacts,
			payload: `{"lastKey":"C-0042","processed":5000,"total":9000,"skipped":7}`,
			want:    services.ImportCursor{LastKey: "C-0042", Processed: 5000, Total: 9000, Skipped: 7},
		},
		"nndns": {
			phase:   services.PhaseNNDNs,
			payload: `{"lastKey":"blocked.paco","processed":10000,"total":20001}`,
			want:    services.ImportCursor{LastKey: "blocked.paco", Processed: 10000, Total: 20001},
		},
		"key needing escapes": {
			phase:   services.PhaseContacts,
			payload: `{"lastKey":"we\"ird\\name","processed":1,"total":2}`,
			want:    services.ImportCursor{LastKey: `we"ird\name`, Processed: 1, Total: 2},
		},
		"domains start over": {
			phase:   services.PhaseDomains,
			payload: `{"lastKey":"a2.paco","processed":10000,"total":20001}`,
			want:    services.ImportCursor{},
		},
		"hosts start over": {
			phase:   services.PhaseHosts,
			payload: `{"lastKey":"ns1.example","processed":5000,"total":6000}`,
			want:    services.ImportCursor{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, resumedCursor(t, tc.phase, tc.payload))
		})
	}
}

// Details that are not a cursor in either form cannot be trusted as a key.
// Starting over re-upserts rows already written; guessing a key could skip
// rows that never were.
func TestIngestCursor_StartsOverOnUnreadableDetails(t *testing.T) {
	for name, details := range map[string]interface{}{
		"bare text": "example.paco",
		"number":    42,
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, services.ImportCursor{}, resumedCursor(t, services.PhaseContacts, details))
		})
	}
}
