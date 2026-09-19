package services

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/onasunnymorning/domain-os/internal/application/dataqa"
)

// JiscQAPipeline identifies the JISC direct import in a QA report.
const JiscQAPipeline = "jisc-direct-import"

// JiscQAInput carries what the QA needs beyond the staged SQLite database itself.
type JiscQAInput struct {
	// SourceKey identifies the data being validated (the JSON export path).
	SourceKey string
	// TLD is the TLD the domains will be imported under.
	TLD string
	// ParsedDomains is the number of records decoded from the source, before staging.
	ParsedDomains int64
	// ClIDMap maps a staged registrar ID to the ClID the import will write its objects under.
	ClIDMap map[string]string
	// ExistingClIDs is the set of registrar ClIDs that exist in Postgres. The import writes objects
	// under a ClID foreign-keyed to a registrar, so one outside this set cannot be written.
	ExistingClIDs map[string]struct{}
}

// RunJiscStagedQA validates the SQLite database GenerateEscrowDB staged from a JISC export, before any of
// it is written to Postgres, and returns the QA report whose Passed field is the gate.
//
// It only reads: every statement is a SELECT, so it is safe to re-run at any time. It needs no network;
// the two Postgres-derived inputs (ClIDMap, ExistingClIDs) are passed in.
//
// Staging is where data goes missing quietly. GenerateEscrowDB logs and skips every failed insert, so a
// duplicate domain in the export, or a contact that failed to insert, leaves the staged database smaller
// than the source with no error. These checks are what turn that into a stop.
func RunJiscStagedQA(db *sql.DB, in JiscQAInput) (*dataqa.QAReport, error) {
	if db == nil {
		return nil, fmt.Errorf("staged database is required")
	}
	report := dataqa.NewQAReport(JiscQAPipeline, in.SourceKey, map[string]string{"tld": in.TLD})
	q := &jiscQA{db: db}

	for _, t := range []struct{ key, table string }{
		{"contacts", "contacts"}, {"hosts", "hosts"}, {"domains", "domains"},
		{"registrars", "registrars"}, {"domain_hosts", "domain_nameservers"},
	} {
		n, err := q.count("SELECT COUNT(*) FROM " + t.table) // #nosec G202 -- t.table is one of the compile-time literals above
		if err != nil {
			return nil, fmt.Errorf("count %s: %w", t.table, err)
		}
		report.Summary[t.key] = n
	}
	report.Summary["parsed_domains"] = in.ParsedDomains

	report.AddCheck(q.entityCounts(in, report.Summary))
	report.AddCheck(q.referentialIntegrity())
	report.AddCheck(q.requiredFields())
	report.AddCheck(q.unmappedIdentity(in))
	return report, nil
}

type jiscQA struct{ db *sql.DB }

func (q *jiscQA) count(query string, args ...any) (int64, error) {
	var n int64
	if err := q.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// unverifiable is the result of a check whose query failed. A check that cannot be run is not a check
// that passed, so it closes the gate.
func unverifiable(rule, description string, err error) dataqa.QACheck {
	return dataqa.QACheck{
		Rule:        rule,
		Description: description,
		Severity:    dataqa.SeverityError,
		Passed:      false,
		Message:     fmt.Sprintf("Query failed, unable to verify: %v", err),
	}
}

// sample returns up to dataqa.MaxSampledItems values of column from the query, which must select one text column.
func (q *jiscQA) sample(query string) []string {
	rows, err := q.db.Query(query + fmt.Sprintf(" LIMIT %d", dataqa.MaxSampledItems)) // #nosec G202 -- the query is a literal at every call site and the limit is a constant integer
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v sql.NullString
		if rows.Scan(&v) == nil {
			out = append(out, v.String)
		}
	}
	return out
}

// entityCounts is entity_count_mismatch: the staged counts match the parse phase. Every source record
// yields exactly one domain and one contact, so both should equal the number of records decoded.
func (q *jiscQA) entityCounts(in JiscQAInput, summary map[string]int64) dataqa.QACheck {
	const rule, desc = "entity_count_mismatch", "Staged domain and contact counts match the number of records parsed from the source"
	domains, contacts := summary["domains"], summary["contacts"]
	missing := abs64(in.ParsedDomains-domains) + abs64(in.ParsedDomains-contacts)

	check := dataqa.QACheck{
		Rule: rule, Description: desc, Severity: dataqa.SeverityError,
		Passed:        missing == 0,
		AffectedCount: int(missing),
		Detail: map[string]any{
			"parsed":  in.ParsedDomains,
			"domains": map[string]int64{"expected": in.ParsedDomains, "staged": domains},
			"contacts": map[string]int64{
				"expected": in.ParsedDomains, "staged": contacts,
			},
		},
	}
	if check.Passed {
		check.Message = fmt.Sprintf("Counts match: %d domains and %d contacts staged, %d parsed", domains, contacts, in.ParsedDomains)
	} else {
		check.Message = fmt.Sprintf("Staging lost records: %d parsed, %d domains and %d contacts staged (duplicate names or failed inserts)", in.ParsedDomains, domains, contacts)
	}
	return check
}

// referentialIntegrity is referential_integrity: references resolve within the staged data.
func (q *jiscQA) referentialIntegrity() dataqa.QACheck {
	const rule, desc = "referential_integrity", "Every domain registrant resolves to a staged contact, and every nameserver link to a staged domain and host"
	refs := []struct{ name, where, sampleCol string }{
		{"domains.registrant", "FROM domains WHERE registrant IS NOT NULL AND registrant != '' AND registrant NOT IN (SELECT id FROM contacts)", "name"},
		{"domain_nameservers.domain_name", "FROM domain_nameservers WHERE domain_name NOT IN (SELECT name FROM domains)", "domain_name"},
		{"domain_nameservers.nameserver", "FROM domain_nameservers WHERE nameserver NOT IN (SELECT name FROM hosts)", "nameserver"},
	}
	var total int64
	byRef := map[string]int64{}
	var samples []map[string]string
	for _, r := range refs {
		n, err := q.count("SELECT COUNT(*) " + r.where) // #nosec G202 -- r.where is a compile-time literal above
		if err != nil {
			return unverifiable(rule, desc, err)
		}
		byRef[r.name] = n
		total += n
		if n > 0 && len(samples) < dataqa.MaxSampledItems {
			for _, v := range q.sample("SELECT " + r.sampleCol + " " + r.where) { // #nosec G202 -- both parts are compile-time literals above
				if len(samples) == dataqa.MaxSampledItems {
					break
				}
				samples = append(samples, map[string]string{"reference": r.name, "value": v})
			}
		}
	}
	check := dataqa.QACheck{
		Rule: rule, Description: desc, Severity: dataqa.SeverityError,
		Passed: total == 0, AffectedCount: int(total), Detail: byRef,
	}
	if total == 0 {
		check.Message = "All references resolve"
	} else {
		check.Message = fmt.Sprintf("%d references do not resolve within the staged data", total)
		check.SampledItems = samples
	}
	return check
}

// requiredFields is required_field_null: no NULL or blank in the columns the import cannot do without.
func (q *jiscQA) requiredFields() dataqa.QACheck {
	const rule, desc = "required_field_null", "No NULL or blank in required columns (domains: name, clid, registrant; contacts: id, clid; hosts: name, clid)"
	cols := []struct{ table, col, key string }{
		{"domains", "name", "name"}, {"domains", "clid", "name"}, {"domains", "registrant", "name"},
		{"contacts", "id", "id"}, {"contacts", "clid", "id"},
		{"hosts", "name", "name"}, {"hosts", "clid", "name"},
	}
	var total int64
	byCol := map[string]int64{}
	var samples []map[string]string
	for _, c := range cols {
		where := fmt.Sprintf("FROM %s WHERE %s IS NULL OR TRIM(%s) = ''", c.table, c.col, c.col)
		n, err := q.count("SELECT COUNT(*) " + where) // #nosec G202 -- table and column names are compile-time literals above
		if err != nil {
			return unverifiable(rule, desc, err)
		}
		byCol[c.table+"."+c.col] = n
		total += n
		if n > 0 && len(samples) < dataqa.MaxSampledItems {
			for _, v := range q.sample("SELECT " + c.key + " " + where) { // #nosec G202 -- built from compile-time literals above
				if len(samples) == dataqa.MaxSampledItems {
					break
				}
				samples = append(samples, map[string]string{"column": c.table + "." + c.col, "key": v})
			}
		}
	}
	check := dataqa.QACheck{
		Rule: rule, Description: desc, Severity: dataqa.SeverityError,
		Passed: total == 0, AffectedCount: int(total), Detail: byCol,
	}
	if total == 0 {
		check.Message = "All required fields are populated"
	} else {
		check.Message = fmt.Sprintf("%d required values are NULL or blank", total)
		check.SampledItems = samples
	}
	return check
}

// unmappedIdentity is unmapped_identity: every registrar the staged objects belong to resolves to a
// registrar that exists in Postgres. An object whose ClID does not is either skipped silently by the
// import or, when the ClID is mapped but absent, rejected by the foreign key partway through a run that
// has already written earlier batches.
func (q *jiscQA) unmappedIdentity(in JiscQAInput) dataqa.QACheck {
	const rule, desc = "unmapped_identity", "Every registrar referenced by staged contacts, hosts and domains maps to a registrar that exists in Postgres"

	// Rows per staged registrar ID, across the three object tables.
	rowsFor := map[string]int64{}
	for _, table := range []string{"contacts", "hosts", "domains"} {
		rows, err := q.db.Query("SELECT TRIM(clid), COUNT(*) FROM " + table + " WHERE clid IS NOT NULL AND TRIM(clid) != '' GROUP BY TRIM(clid)") // #nosec G202 -- table is a compile-time literal above
		if err != nil {
			return unverifiable(rule, desc, err)
		}
		for rows.Next() {
			var id string
			var n int64
			if err := rows.Scan(&id, &n); err != nil {
				rows.Close()
				return unverifiable(rule, desc, err)
			}
			rowsFor[id] += n
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return unverifiable(rule, desc, err)
		}
		rows.Close()
	}

	type bad struct {
		Registrar string `json:"registrar"`
		Reason    string `json:"reason"`
		Rows      int64  `json:"rows"`
	}
	var unresolved []bad
	var affected int64
	for id, n := range rowsFor {
		clid, mapped := in.ClIDMap[id]
		switch _, exists := in.ExistingClIDs[clid]; {
		case !mapped || clid == "":
			unresolved = append(unresolved, bad{id, "not mapped to any registrar", n})
		case !exists:
			unresolved = append(unresolved, bad{id, fmt.Sprintf("maps to ClID %q, which does not exist in Postgres", clid), n})
		default:
			continue
		}
		affected += n
	}
	// Most affected first, then by ID, so the sample is the part that matters and the report is stable.
	sort.Slice(unresolved, func(i, j int) bool {
		if unresolved[i].Rows != unresolved[j].Rows {
			return unresolved[i].Rows > unresolved[j].Rows
		}
		return unresolved[i].Registrar < unresolved[j].Registrar
	})

	check := dataqa.QACheck{
		Rule: rule, Description: desc, Severity: dataqa.SeverityError,
		Passed: len(unresolved) == 0, AffectedCount: int(affected),
		Detail: map[string]int{"registrarsReferenced": len(rowsFor), "registrarsUnresolved": len(unresolved)},
	}
	if len(unresolved) == 0 {
		check.Message = fmt.Sprintf("All %d referenced registrars resolve to a registrar in Postgres", len(rowsFor))
		return check
	}
	check.Message = fmt.Sprintf("%d of %d referenced registrars do not resolve to a registrar in Postgres, affecting %d objects", len(unresolved), len(rowsFor), affected)
	if len(unresolved) > dataqa.MaxSampledItems {
		unresolved = unresolved[:dataqa.MaxSampledItems]
	}
	check.SampledItems = unresolved
	return check
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
