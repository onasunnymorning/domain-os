package services_test

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/dataqa"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities/jisc"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func jiscRecord(domainID int, name string, registrarJoID int, registrarName string) jisc.JiscDomain {
	return jisc.JiscDomain{
		DomainID:           domainID,
		DomainName:         name,
		RegisteredDate:     "2020-01-01T00:00:00Z",
		RegisterExpireDate: "2030-01-01T00:00:00Z",
		Status:             "ACTIVE",
		DNS1:               "ns1.example.net,192.0.2.1",
		Registrant:         jisc.JiscRegistrant{ContactID: -1, Name: "Someone", Email: "someone@example.invalid", City: "Bath", Country: "GB"},
		Registrar:          jisc.JiscRegistrar{JoID: registrarJoID, Name: registrarName},
	}
}

func writeJiscJSON(t *testing.T, records ...jisc.JiscDomain) string {
	t.Helper()
	raw, err := json.Marshal(records)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "jisc.json")
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

// stageJisc stages the records through the real GenerateEscrowDB and opens the result.
func stageJisc(t *testing.T, records ...jisc.JiscDomain) (db *sql.DB, dbPath string) {
	t.Helper()
	jsonPath := writeJiscJSON(t, records...)
	require.NoError(t, services.NewJiscService().GenerateEscrowDB(jsonPath))
	dbPath = jsonPath[:len(jsonPath)-len(".json")] + ".db"
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, dbPath
}

func cleanRecords() []jisc.JiscDomain {
	return []jisc.JiscDomain{
		jiscRecord(1, "one.ac.uk", 100, "Registrar A"),
		jiscRecord(2, "two.ac.uk", 100, "Registrar A"),
		jiscRecord(3, "three.ac.uk", 200, "Registrar B"),
	}
}

func resolvedInput(parsed int64) services.JiscQAInput {
	return services.JiscQAInput{
		SourceKey:     "jisc.json",
		TLD:           "ac.uk",
		ParsedDomains: parsed,
		ClIDMap:       map[string]string{"100": "rar-a", "200": "rar-b"},
		ExistingClIDs: map[string]struct{}{"rar-a": {}, "rar-b": {}},
	}
}

func checkByRule(t *testing.T, r *dataqa.QAReport, rule string) dataqa.QACheck {
	t.Helper()
	for _, c := range r.Checks {
		if c.Rule == rule {
			return c
		}
	}
	t.Fatalf("report has no %q check", rule)
	return dataqa.QACheck{}
}

func TestRunJiscStagedQA_CleanDataOpensTheGate(t *testing.T) {
	db, _ := stageJisc(t, cleanRecords()...)

	report, err := services.RunJiscStagedQA(db, resolvedInput(3))
	require.NoError(t, err)

	require.True(t, report.Passed, "failed checks: %+v", report.Failed())
	require.Equal(t, services.JiscQAPipeline, report.Pipeline)
	// The four checks every pipeline must have (data-pipeline-qa), all blocking.
	for _, rule := range []string{"entity_count_mismatch", "referential_integrity", "required_field_null", "unmapped_identity"} {
		c := checkByRule(t, report, rule)
		require.Equal(t, dataqa.SeverityError, c.Severity, rule)
		require.True(t, c.Passed, rule)
		require.Zero(t, c.AffectedCount, rule)
	}
	require.Equal(t, int64(3), report.Summary["domains"])
	require.Equal(t, int64(3), report.Summary["contacts"])
	require.Equal(t, int64(2), report.Summary["registrars"])
	require.Equal(t, int64(3), report.Summary["parsed_domains"])
}

// GenerateEscrowDB logs and skips a failed insert, so a duplicate name in the export quietly shrinks the
// staged data. This is the truncation case INV-05 is about: it must close the gate, not import "successfully".
func TestRunJiscStagedQA_DuplicateSourceDomainClosesTheGate(t *testing.T) {
	records := append(cleanRecords(), jiscRecord(4, "one.ac.uk", 100, "Registrar A")) // same name as record 1
	db, _ := stageJisc(t, records...)

	report, err := services.RunJiscStagedQA(db, resolvedInput(int64(len(records))))
	require.NoError(t, err)

	require.False(t, report.Passed)
	c := checkByRule(t, report, "entity_count_mismatch")
	require.False(t, c.Passed)
	require.Contains(t, c.Message, "4 parsed")
	require.Equal(t, int64(3), report.Summary["domains"], "the duplicate never reached the staged database")
}

func TestRunJiscStagedQA_DanglingReferencesCloseTheGate(t *testing.T) {
	db, _ := stageJisc(t, cleanRecords()...)
	// A domain whose registrant contact is missing, and a nameserver link to a domain that is not staged.
	_, err := db.Exec(`DELETE FROM contacts WHERE id = 'C-1'`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO domain_nameservers (domain_name, nameserver) VALUES ('ghost.ac.uk', 'ns1.example.net')`)
	require.NoError(t, err)

	report, err := services.RunJiscStagedQA(db, resolvedInput(3))
	require.NoError(t, err)

	require.False(t, report.Passed)
	c := checkByRule(t, report, "referential_integrity")
	require.False(t, c.Passed)
	require.Equal(t, 2, c.AffectedCount)
	require.NotEmpty(t, c.SampledItems)
}

func TestRunJiscStagedQA_BlankRequiredFieldClosesTheGate(t *testing.T) {
	db, _ := stageJisc(t, cleanRecords()...)
	_, err := db.Exec(`UPDATE domains SET clid = '' WHERE name = 'two.ac.uk'`)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE contacts SET clid = NULL WHERE id = 'C-3'`)
	require.NoError(t, err)

	report, err := services.RunJiscStagedQA(db, resolvedInput(3))
	require.NoError(t, err)

	require.False(t, report.Passed)
	c := checkByRule(t, report, "required_field_null")
	require.False(t, c.Passed)
	require.Equal(t, 2, c.AffectedCount)
}

func TestRunJiscStagedQA_UnresolvedRegistrarsCloseTheGate(t *testing.T) {
	db, _ := stageJisc(t, cleanRecords()...)

	t.Run("registrar with no mapping", func(t *testing.T) {
		in := resolvedInput(3)
		delete(in.ClIDMap, "200")
		report, err := services.RunJiscStagedQA(db, in)
		require.NoError(t, err)
		require.False(t, report.Passed)
		c := checkByRule(t, report, "unmapped_identity")
		require.False(t, c.Passed)
		// Registrar 200 owns one contact and one domain. The shared nameserver host was staged under the
		// first registrar to use it (INSERT OR IGNORE), so it is not registrar 200's.
		require.Equal(t, 2, c.AffectedCount)
	})

	// The import maps a registrar it cannot find by name to a made-up ClID. If nothing by that ClID is
	// in Postgres, the foreign key rejects the objects partway through the write.
	t.Run("registrar mapped to a ClID that does not exist in Postgres", func(t *testing.T) {
		in := resolvedInput(3)
		in.ClIDMap["200"] = "r-200"
		report, err := services.RunJiscStagedQA(db, in)
		require.NoError(t, err)
		require.False(t, report.Passed)
		c := checkByRule(t, report, "unmapped_identity")
		require.False(t, c.Passed)
		require.Contains(t, fmt.Sprint(c.SampledItems), "r-200")
	})

	t.Run("a made-up ClID is fine when a registrar by that ClID does exist", func(t *testing.T) {
		in := resolvedInput(3)
		in.ClIDMap["200"] = "r-200"
		in.ExistingClIDs["r-200"] = struct{}{}
		report, err := services.RunJiscStagedQA(db, in)
		require.NoError(t, err)
		require.True(t, report.Passed, "failed checks: %+v", report.Failed())
	})
}

func TestRunJiscStagedQA_SamplesAreCapped(t *testing.T) {
	var records []jisc.JiscDomain
	for i := 1; i <= 120; i++ {
		records = append(records, jiscRecord(i, fmt.Sprintf("d%d.ac.uk", i), 1000+i, fmt.Sprintf("Registrar %d", i)))
	}
	db, _ := stageJisc(t, records...)

	in := resolvedInput(120)
	in.ClIDMap, in.ExistingClIDs = map[string]string{}, map[string]struct{}{} // none of the 120 registrars resolve
	report, err := services.RunJiscStagedQA(db, in)
	require.NoError(t, err)

	c := checkByRule(t, report, "unmapped_identity")
	require.False(t, c.Passed)
	b, err := json.Marshal(c.SampledItems)
	require.NoError(t, err)
	var sampled []map[string]any
	require.NoError(t, json.Unmarshal(b, &sampled))
	require.Len(t, sampled, dataqa.MaxSampledItems)
	// 120 registrars own a contact and a domain each, and the first also owns the one shared host.
	require.Equal(t, 120*2+1, c.AffectedCount, "the count still covers everything, only the sample is capped")
}

// QA must be safe to re-run at any time (data-pipeline-qa rule 6), so it must not change the staged database.
func TestRunJiscStagedQA_DoesNotModifyTheStagedDatabase(t *testing.T) {
	records := append(cleanRecords(), jiscRecord(4, "one.ac.uk", 100, "Registrar A")) // gives the checks something to find
	db, dbPath := stageJisc(t, records...)
	digest := func() [32]byte {
		raw, err := os.ReadFile(dbPath)
		require.NoError(t, err)
		return sha256.Sum256(raw)
	}
	before := digest()

	for i := 0; i < 2; i++ {
		_, err := services.RunJiscStagedQA(db, resolvedInput(int64(len(records))))
		require.NoError(t, err)
	}
	require.Equal(t, before, digest(), "QA changed the staged database")
}

// A check that cannot run is not a check that passed.
func TestRunJiscStagedQA_UnreadableStagedDataIsAnErrorNotAPass(t *testing.T) {
	db, _ := stageJisc(t, cleanRecords()...)
	_, err := db.Exec(`DROP TABLE domain_nameservers`)
	require.NoError(t, err)

	report, err := services.RunJiscStagedQA(db, resolvedInput(3))
	require.Error(t, err)
	require.Nil(t, report)
}
