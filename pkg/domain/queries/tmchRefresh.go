package queries

import (
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Outcomes of refreshing one TMCH data set.
const (
	TMCHRefreshed = "refreshed" // a newer generation was fetched and stored
	TMCHUnchanged = "unchanged" // the Clearinghouse still publishes the stored generation
	TMCHFailed    = "failed"    // the fetch or the store failed; what was stored stands
)

// TMCHSetRefresh is one set's outcome in a refresh run.
type TMCHSetRefresh struct {
	Set     string                  `json:"set"`
	Outcome string                  `json:"outcome"`
	Error   string                  `json:"error,omitempty"`
	Status  *entities.TMCHSetStatus `json:"status,omitempty"`
}

// TMCHRefreshResult is a refresh run over the three sets (ADR 0024 §4,
// roadmap 9a): what each one did, with the status now stored.
type TMCHRefreshResult struct {
	StartedAt   time.Time        `json:"startedAt"`
	CompletedAt time.Time        `json:"completedAt"`
	Sets        []TMCHSetRefresh `json:"sets"`
}

// Failed counts the sets whose refresh failed.
func (r TMCHRefreshResult) Failed() int {
	n := 0
	for _, s := range r.Sets {
		if s.Outcome == TMCHFailed {
			n++
		}
	}
	return n
}

// Refreshed counts the sets that were replaced by a newer generation.
func (r TMCHRefreshResult) Refreshed() int {
	n := 0
	for _, s := range r.Sets {
		if s.Outcome == TMCHRefreshed {
			n++
		}
	}
	return n
}
