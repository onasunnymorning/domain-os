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
// as preformatted text, so ParseImportCursor reads those payloads too.
type ImportCursor struct {
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
