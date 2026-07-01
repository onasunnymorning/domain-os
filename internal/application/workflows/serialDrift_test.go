package workflows

import (
	"os"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/serialdrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// testCase mirrors the YAML structure in testdata/serial_drift_cases.yaml.
type testCase struct {
	ID          string `yaml:"id"`
	Category    string `yaml:"category"`
	Description string `yaml:"description"`
	Master      struct {
		Nameserver string `yaml:"nameserver"`
		Serial     uint32 `yaml:"serial"`
		Refresh    uint32 `yaml:"refresh"`
		Retry      uint32 `yaml:"retry"`
		Expire     uint32 `yaml:"expire"`
	} `yaml:"master"`
	Slaves []struct {
		Nameserver string `yaml:"nameserver"`
		Serial     uint32 `yaml:"serial"`
		Error      string `yaml:"error"`
	} `yaml:"slaves"`
	Config struct {
		StalledAfterN   int     `yaml:"stalled_after_n"`
		ConfidenceN     int     `yaml:"confidence_n"`
		GraceMultiplier float64 `yaml:"grace_multiplier"`
	} `yaml:"config"`
	History []struct {
		Nameserver   string `yaml:"nameserver"`
		Serial       uint32 `yaml:"serial"`
		MasterSerial uint32 `yaml:"master_serial"`
	} `yaml:"history"`
	Expect struct {
		PerSlave map[string]struct {
			Status    string `yaml:"status"`
			DriftTier string `yaml:"drift_tier"`
		} `yaml:"per_slave"`
		OverallTier string `yaml:"overall_tier"`
	} `yaml:"expect"`
}

type testSuite struct {
	Cases []testCase `yaml:"cases"`
}

func loadTestCases(t *testing.T) []testCase {
	t.Helper()
	data, err := os.ReadFile("testdata/serial_drift_cases.yaml")
	require.NoError(t, err, "failed to read test cases file")

	var suite testSuite
	require.NoError(t, yaml.Unmarshal(data, &suite), "failed to parse test cases YAML")
	require.NotEmpty(t, suite.Cases, "no test cases found")
	return suite.Cases
}

func TestEvaluateDrift(t *testing.T) {
	cases := loadTestCases(t)

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			// Build master SOAQueryResult
			master := serialdrift.SOAQueryResult{
				Nameserver: tc.Master.Nameserver,
				Serial:     tc.Master.Serial,
				Refresh:    tc.Master.Refresh,
				Retry:      tc.Master.Retry,
				Expire:     tc.Master.Expire,
			}

			// Build slave SOAQueryResults
			var slaves []serialdrift.SOAQueryResult
			for _, s := range tc.Slaves {
				slaves = append(slaves, serialdrift.SOAQueryResult{
					Nameserver: s.Nameserver,
					Serial:     s.Serial,
					Error:      s.Error,
				})
			}

			// Build config
			config := serialdrift.Config{
				StalledAfterN:   tc.Config.StalledAfterN,
				ConfidenceN:     tc.Config.ConfidenceN,
				GraceMultiplier: tc.Config.GraceMultiplier,
			}

			// Build history
			var history []serialdrift.ObservationHistoryEntry
			for _, h := range tc.History {
				history = append(history, serialdrift.ObservationHistoryEntry{
					Nameserver:   h.Nameserver,
					Serial:       h.Serial,
					MasterSerial: h.MasterSerial,
				})
			}

			// Run EvaluateDrift
			results, overallTier := EvaluateDrift(master, slaves, config, history)

			// Assert overall tier
			assert.Equal(t, tc.Expect.OverallTier, overallTier,
				"overall tier mismatch for case %s", tc.ID)

			// Assert per-slave results
			for _, obs := range results {
				expected, ok := tc.Expect.PerSlave[obs.Nameserver]
				require.True(t, ok,
					"unexpected nameserver %s in results for case %s", obs.Nameserver, tc.ID)
				assert.Equal(t, expected.Status, obs.Status,
					"status mismatch for %s in case %s", obs.Nameserver, tc.ID)
				assert.Equal(t, expected.DriftTier, obs.DriftTier,
					"drift tier mismatch for %s in case %s", obs.Nameserver, tc.ID)
			}

			// Ensure all expected slaves were evaluated
			assert.Equal(t, len(tc.Expect.PerSlave), len(results),
				"number of evaluated slaves doesn't match expected for case %s", tc.ID)
		})
	}
}

func TestEvaluateDrift_EmptySlaves(t *testing.T) {
	master := serialdrift.SOAQueryResult{Nameserver: "ns1.master.example", Serial: 100}
	results, tier := EvaluateDrift(master, nil, serialdrift.Config{}, nil)
	assert.Empty(t, results)
	assert.Equal(t, "expected", tier)
}

func TestWorstTier(t *testing.T) {
	tests := []struct {
		a, b, want string
	}{
		{"expected", "expected", "expected"},
		{"expected", "warning", "warning"},
		{"expected", "critical", "critical"},
		{"warning", "expected", "warning"},
		{"warning", "warning", "warning"},
		{"warning", "critical", "critical"},
		{"critical", "expected", "critical"},
		{"critical", "warning", "critical"},
		{"critical", "critical", "critical"},
	}
	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			assert.Equal(t, tt.want, worstTier(tt.a, tt.b))
		})
	}
}
