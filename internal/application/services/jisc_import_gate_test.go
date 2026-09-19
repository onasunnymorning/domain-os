package services_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/dataqa"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities/jisc"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The registrar name seedFixtures gives its registrar. ImportToDirectDB resolves JISC registrars to
// Postgres registrars by name, so the export has to use it.
const seededRegistrarName = "AuthInfo Import Registrar"

// gateRecords builds export records that resolve to the seeded registrar and share one nameserver host.
func gateRecords(names ...string) []jisc.JiscDomain {
	var records []jisc.JiscDomain
	for i, name := range names {
		r := jiscRecord(810001+i, name, 555, seededRegistrarName)
		r.DNS1 = "ns1.jisc-gate-test.example.net,192.0.2.7"
		records = append(records, r)
	}
	return records
}

func readQAReport(t *testing.T, jsonPath string) dataqa.QAReport {
	t.Helper()
	raw, err := os.ReadFile(strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + "_qa-report.json")
	require.NoError(t, err, "the QA report must be written before the gate decides")
	var report dataqa.QAReport
	require.NoError(t, json.Unmarshal(raw, &report))
	return report
}

func countRows(t *testing.T, db *gorm.DB, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(query, args...).Scan(&n).Error)
	return n
}

// The export repeats a domain name, which staging drops without an error. Before the gate this imported
// "successfully" with a domain missing; now it has to stop before anything reaches Postgres.
func TestJiscImportToDirectDB_QAFailureStopsBeforeAnyWrite(t *testing.T) {
	db := authInfoTestDB(t)
	clID := seedFixtures(t, db, "ac.uk", 91003)
	jsonPath := writeJiscJSON(t, gateRecords("qa-one.ac.uk", "qa-two.ac.uk", "qa-three.ac.uk", "qa-one.ac.uk")...)

	err := services.NewJiscService().ImportToDirectDB(jsonPath)

	require.Error(t, err)
	require.Contains(t, err.Error(), "entity_count_mismatch")
	require.Contains(t, err.Error(), "nothing was written to Postgres")

	report := readQAReport(t, jsonPath)
	require.False(t, report.Passed)

	require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM contacts WHERE cl_id = ?`, clID), "contacts were written past a failed gate")
	require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM domains WHERE tld_name = 'ac.uk'`), "domains were written past a failed gate")
	require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM hosts WHERE cl_id = ?`, clID), "hosts were written past a failed gate")
	_, statErr := os.Stat(strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + "_runreport.json")
	require.True(t, os.IsNotExist(statErr), "the run report is only for a run that reached the writes")
}

// A registrar the export names that Postgres does not have would be mapped to a made-up ClID and rejected
// by the foreign key mid-write. The gate reports it up front.
func TestJiscImportToDirectDB_UnknownRegistrarStopsBeforeAnyWrite(t *testing.T) {
	db := authInfoTestDB(t)
	clID := seedFixtures(t, db, "ac.uk", 91003)
	records := gateRecords("qa-one.ac.uk", "qa-two.ac.uk")
	records[1].Registrar = jisc.JiscRegistrar{JoID: 556, Name: "A Registrar Postgres Has Never Heard Of"}
	jsonPath := writeJiscJSON(t, records...)

	err := services.NewJiscService().ImportToDirectDB(jsonPath)

	require.Error(t, err)
	require.Contains(t, err.Error(), "unmapped_identity")
	report := readQAReport(t, jsonPath)
	require.False(t, report.Passed)
	require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM contacts WHERE cl_id = ?`, clID))
	require.Zero(t, countRows(t, db, `SELECT COUNT(*) FROM domains WHERE tld_name = 'ac.uk'`))
}

func TestJiscImportToDirectDB_PassingQAImportsWithGeneratedAuthInfos(t *testing.T) {
	db := authInfoTestDB(t)
	clID := seedFixtures(t, db, "ac.uk", 91003)
	jsonPath := writeJiscJSON(t, gateRecords("qa-one.ac.uk", "qa-two.ac.uk", "qa-three.ac.uk")...)

	require.NoError(t, services.NewJiscService().ImportToDirectDB(jsonPath))

	report := readQAReport(t, jsonPath)
	require.True(t, report.Passed, "failed checks: %+v", report.Failed())
	require.Len(t, report.Checks, 4)

	require.Equal(t, int64(3), countRows(t, db, `SELECT COUNT(*) FROM domains WHERE tld_name = 'ac.uk'`))
	require.Equal(t, int64(3), countRows(t, db, `SELECT COUNT(*) FROM contacts WHERE cl_id = ?`, clID))
	requireGeneratedAuthInfos(t, 3, storedAuthInfos(t, db, `SELECT name AS key, auth_info FROM domains WHERE tld_name = ?`, "ac.uk"))
	requireGeneratedAuthInfos(t, 3, storedAuthInfos(t, db, `SELECT id AS key, auth_info FROM contacts WHERE cl_id = ?`, clID))
	_, err := os.Stat(strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + "_runreport.json")
	require.NoError(t, err, "a run that passed the gate writes its run report")
}
