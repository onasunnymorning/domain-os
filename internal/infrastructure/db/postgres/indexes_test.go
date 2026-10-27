package postgres

import (
	"testing"
)

// TestAutoMigrateIndexes pins the index set AutoMigrate leaves on the two
// hottest write tables: the composites that serve every lookup by cl_id or
// tld_name as a left prefix, the partial indexes that hold only the rows
// their sweeps read, and none of the single-column or full boolean indexes
// they replaced. GORM never drops an index, so a removed tag alone would
// leave the old one in place on every existing database; the legacy cleanup
// in AutoMigrate is what this test guards.
func TestAutoMigrateIndexes(t *testing.T) {
	db := getTestDB()
	// A database from before this change still has the indexes the struct
	// tags used to create. Put them back and migrate again: the drops, not
	// the absence of a tag, are what must remove them.
	for _, legacy := range []string{
		"CREATE INDEX IF NOT EXISTS idx_domains_cl_id ON domains (cl_id)",
		"CREATE INDEX IF NOT EXISTS idx_domains_tld_name ON domains (tld_name)",
		"CREATE INDEX IF NOT EXISTS idx_domains_purge_date ON domains (purge_date)",
		"CREATE INDEX IF NOT EXISTS idx_domains_pending_restore_end ON domains (pending_restore_end)",
		"CREATE INDEX IF NOT EXISTS idx_domain_events_published ON domain_events (published)",
		"CREATE INDEX IF NOT EXISTS idx_domain_events_source ON domain_events (source)",
		"CREATE INDEX IF NOT EXISTS idx_domain_events_subject ON domain_events (subject)",
	} {
		if err := db.Exec(legacy).Error; err != nil {
			t.Fatalf("seed legacy index: %v", err)
		}
	}
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate on the legacy schema: %v", err)
	}
	var rows []struct {
		Indexname string
		Indexdef  string
	}
	if err := db.Raw("SELECT indexname, indexdef FROM pg_indexes WHERE tablename IN ('domains', 'domain_events')").Scan(&rows).Error; err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	defs := map[string]string{}
	for _, r := range rows {
		defs[r.Indexname] = r.Indexdef
	}

	wantPresent := map[string]string{
		"idx_domains_clid_tld_expiry":             "",
		"idx_domains_tld_expiry":                  "",
		"idx_domains_pending_delete_purge":        "WHERE (pending_delete = true)",
		"idx_domains_pending_restore_pending_end": "WHERE (pending_restore = true)",
		"idx_domain_events_unpublished":           "WHERE (published = false)",
		"idx_domain_events_occurred_at":           "",
		"idx_events_subject_occurred":             "",
	}
	for name, predicate := range wantPresent {
		def, ok := defs[name]
		if !ok {
			t.Errorf("index %s is missing", name)
			continue
		}
		if predicate != "" && !contains(def, predicate) {
			t.Errorf("index %s is not partial on %q: %s", name, predicate, def)
		}
	}

	for _, name := range []string{
		"idx_domains_cl_id", "idx_domains_tld_name", // left prefixes of the composites
		"idx_domains_purge_date", "idx_domains_pending_restore_end", "idx_domain_events_published", // replaced by partial indexes
		"idx_domain_events_source", "idx_domain_events_subject", // earlier cleanup
	} {
		if def, ok := defs[name]; ok {
			t.Errorf("index %s should have been dropped: %s", name, def)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
