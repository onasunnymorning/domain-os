package services_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/stretchr/testify/require"
)

func TestParseImportCursor(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want services.ImportCursor
	}{
		"blank is the top": {text: "  \n", want: services.ImportCursor{}},
		// Exactly what the importers used to heartbeat and write to the S3
		// cursor files, built the way they built it.
		"legacy contacts payload": {
			text: fmt.Sprintf(`{"lastKey":%q,"processed":%d,"total":%d,"skipped":%d}`, "C-0042", 5000, 9000, 7),
			want: services.ImportCursor{LastKey: "C-0042", Processed: 5000, Total: 9000, Skipped: 7},
		},
		"legacy hosts payload, no clid": {
			text: fmt.Sprintf(`{"lastKey":%q,"processed":%d,"total":%d}`, "ns1.example", 5000, 6000),
			want: services.ImportCursor{LastKey: "ns1.example", Processed: 5000, Total: 6000},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := services.ParseImportCursor(tc.text)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseImportCursor_RejectsWhatIsNotACursor(t *testing.T) {
	for _, text := range []string{"ns1.example", `{"lastKey":`, `"quoted"`} {
		got, err := services.ParseImportCursor(text)
		require.Error(t, err, text)
		require.Equal(t, services.ImportCursor{}, got, "a failed parse must not hand back a partial cursor")
	}
}

// A cursor written as JSON reads back unchanged: the S3 cursor files depend on it.
func TestParseImportCursor_RoundTrip(t *testing.T) {
	c := services.ImportCursor{LastKey: "b.example", LastClID: "rar1", Processed: 2, Total: 5, Skipped: 1}
	data, err := json.Marshal(c)
	require.NoError(t, err)
	got, err := services.ParseImportCursor(string(data))
	require.NoError(t, err)
	require.Equal(t, c, got)
}
