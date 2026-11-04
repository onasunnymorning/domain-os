// Package tmch fetches and parses the data the Trademark Clearinghouse
// publishes to registries (ADR 0024 §4, roadmap 9a): the TMCH CA
// certificate bundle, the SMD revocation list and the DNL list.
//
// The list formats are those of the ICANN TMCH Functional Specifications
// (draft-ietf-regext-tmch-func-spec, "SMD Revocation List" and "Domain Name
// Label List"), the CSV shape RFC 7848 §5 refers to:
//
//	1,2026-11-04T00:00:00.0Z                 version, generation datetime
//	smd-id,insertion-datetime                 header (SMDRL)
//	0000001751373633628125-65535,2026-11-03T12:00:00.0Z
//
//	1,2026-11-04T00:00:00.0Z
//	DNL,lookup-key,insertion-datetime         header (DNL list)
//	example-one,2026110400/2/1/5/dkDMTBXNSG4E...,2026-11-03T00:00:00.0Z
//
// Timestamps are RFC 3339 in UTC with a fractional second. A file is
// complete, never a delta; the entities keep the generation time so a
// refresh can tell a new list from the one already stored.
package tmch

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The refusals are the domain's sentinels (INV-09), wrapped with what was
// wrong.
var (
	ErrEmptyList  = entities.ErrTMCHListEmpty
	ErrListHeader = entities.ErrTMCHListHeader
	ErrListRecord = entities.ErrTMCHListRecord
)

// listHeader is the two header lines of an SMDRL or DNL file.
type listHeader struct {
	version     int
	generatedAt time.Time
}

// readList reads the two header lines and returns the data records.
func readList(r io.Reader, wantColumns []string) (listHeader, [][]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	records, err := cr.ReadAll()
	if err != nil {
		return listHeader{}, nil, fmt.Errorf("%w: %w", ErrListRecord, err)
	}
	if len(records) < 2 {
		return listHeader{}, nil, ErrEmptyList
	}
	first := records[0]
	if len(first) != 2 {
		return listHeader{}, nil, fmt.Errorf("%w: first line has %d fields, want version and generation time", ErrListHeader, len(first))
	}
	version, err := strconv.Atoi(strings.TrimSpace(first[0]))
	if err != nil {
		return listHeader{}, nil, fmt.Errorf("%w: version %q", ErrListHeader, first[0])
	}
	generatedAt, err := parseTime(first[1])
	if err != nil {
		return listHeader{}, nil, fmt.Errorf("%w: generation time %q", ErrListHeader, first[1])
	}
	columns := records[1]
	if len(columns) != len(wantColumns) {
		return listHeader{}, nil, fmt.Errorf("%w: column line %v, want %v", ErrListHeader, columns, wantColumns)
	}
	for i, c := range columns {
		if !strings.EqualFold(strings.TrimSpace(c), wantColumns[i]) {
			return listHeader{}, nil, fmt.Errorf("%w: column line %v, want %v", ErrListHeader, columns, wantColumns)
		}
	}
	return listHeader{version: version, generatedAt: generatedAt}, records[2:], nil
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, strings.TrimSpace(s))
}

// ParseSMDRL parses an SMD revocation list file.
func ParseSMDRL(r io.Reader, source string, now time.Time) (*entities.SMDRevocationList, error) {
	header, records, err := readList(r, []string{"smd-id", "insertion-datetime"})
	if err != nil {
		return nil, err
	}
	entries := make([]entities.SMDRevocation, 0, len(records))
	for i, rec := range records {
		if len(rec) != 2 {
			return nil, fmt.Errorf("%w: SMDRL line %d has %d fields", ErrListRecord, i+3, len(rec))
		}
		revokedAt, err := parseTime(rec[1])
		if err != nil {
			return nil, fmt.Errorf("%w: SMDRL line %d: %w", ErrListRecord, i+3, err)
		}
		entries = append(entries, entities.SMDRevocation{SMDID: rec[0], RevokedAt: revokedAt})
	}
	return entities.NewSMDRevocationList(header.version, header.generatedAt, entries, source, now)
}

// ParseDNL parses a DNL (domain name label) list file.
func ParseDNL(r io.Reader, source string, now time.Time) (*entities.DNLList, error) {
	header, records, err := readList(r, []string{"DNL", "lookup-key", "insertion-datetime"})
	if err != nil {
		return nil, err
	}
	entries := make([]entities.DNLEntry, 0, len(records))
	for i, rec := range records {
		if len(rec) != 3 {
			return nil, fmt.Errorf("%w: DNL line %d has %d fields", ErrListRecord, i+3, len(rec))
		}
		insertedAt, err := parseTime(rec[2])
		if err != nil {
			return nil, fmt.Errorf("%w: DNL line %d: %w", ErrListRecord, i+3, err)
		}
		entries = append(entries, entities.DNLEntry{Label: rec[0], LookupKey: strings.TrimSpace(rec[1]), InsertedAt: insertedAt})
	}
	return entities.NewDNLList(header.version, header.generatedAt, entries, source, now)
}
