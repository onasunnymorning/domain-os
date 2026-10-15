package services

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ImportCursor is how far a DirectDBImporter phase has got: the staged key of
// the last row it committed and its counts at that point. The importers report
// one after every committed batch and take one to resume from, so a resumed
// run carries on after that row and ends with the counts an uninterrupted run
// would have.
//
// The JSON names are the ones the importers wrote when they heartbeated this
// as preformatted text, so ParseImportCursor reads those payloads too. Those
// legacy cursors carry no Version; see ResumeCursor for which can be trusted.
type ImportCursor struct {
	// Version is ImportCursorVersion for a cursor the importers record now
	// and 0 for a legacy one.
	Version int `json:"v,omitempty"`
	// LastKey is the staged sort key of the last committed row: the contact
	// id, host name, domain name or NNDN aname. "" means start at the top.
	LastKey string `json:"lastKey"`
	// LastClID breaks ties between same-name hosts, which ImportHosts pages
	// through on (name, clid). Other phases leave it empty.
	LastClID string `json:"lastClID,omitempty"`
	// Processed is the phase's running total: rows written so far.
	Processed int64 `json:"processed"`
	// Total is the number of rows staged for the phase.
	Total int64 `json:"total"`
	// Skipped is the contacts left out so far (ImportContacts only).
	Skipped int64 `json:"skipped,omitempty"`
}

// ImportCursorVersion marks the cursors the importers record: the staged key
// of the last committed row, with the clid for hosts.
const ImportCursorVersion = 1

// ImportPhase names a cursor-paged DirectDBImporter phase.
type ImportPhase string

const (
	PhaseContacts ImportPhase = "contacts"
	PhaseHosts    ImportPhase = "hosts"
	PhaseDomains  ImportPhase = "domains"
	PhaseNNDNs    ImportPhase = "nndns"
)

// ResumeCursor is where phase resumes from c: c itself, or the zero cursor
// (start over with the counts reset) when c cannot be resumed from exactly.
// restarted reports the second case.
//
// A legacy domains cursor holds the domain's normalised name rather than the
// staged name the query compares, so staged names between the two would be
// skipped; a legacy hosts cursor has no clid, so the rest of its name would be
// written and counted again. Neither boundary can be recovered from the
// cursor. Legacy contacts and NNDNs cursors were staged keys, and resume.
// Starting over is safe because every phase upserts.
func ResumeCursor(phase ImportPhase, c ImportCursor) (cursor ImportCursor, restarted bool) {
	if c.LastKey == "" || c.Version >= ImportCursorVersion {
		return c, false
	}
	switch phase {
	case PhaseHosts, PhaseDomains:
		return ImportCursor{}, true
	default:
		return c, false
	}
}

// ParseImportCursor reads a cursor written as JSON text. Blank text is the
// zero cursor: start at the top.
func ParseImportCursor(s string) (ImportCursor, error) {
	var c ImportCursor
	if strings.TrimSpace(s) == "" {
		return c, nil
	}
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return ImportCursor{}, fmt.Errorf("parse import cursor: %w", err)
	}
	return c, nil
}
