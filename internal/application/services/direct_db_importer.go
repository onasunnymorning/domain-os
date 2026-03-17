package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/netip"
	"os"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	dbModels "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
)

// RunReport collects details about the import execution
type RunReport struct {
	Timestamp       time.Time      `json:"timestamp"`
	SkippedHosts    []SkippedItem  `json:"skipped_hosts"`
	ModifiedDomains []ModifiedItem `json:"modified_domains"`
}

type SkippedItem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ModifiedItem struct {
	Name          string `json:"name"`
	Field         string `json:"field"`
	OriginalValue string `json:"original_value"`
	Info          string `json:"info"`
}

// DirectDBImporter handles high-speed imports directly into Postgres
type DirectDBImporter struct {
	PG     *pg.DB
	S3     *storage.S3Client
	IDGen  *snowflakeidgenerator.IDGenerator
	Report *RunReport
}

func NewDirectDBImporter() (*DirectDBImporter, error) {
	// Initialize Postgres connection
	opt, err := pg.ParseURL(fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASS", "postgres"),
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "domain_os"),
		getEnv("DB_SSLMODE", "disable"),
	))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PG URL: %w", err)
	}

	db := pg.Connect(opt)

	// S3 Client is optional for some use-cases (like local CLI import)
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		// Just log, don't fail. Methods using S3 will panic or fail if called, but ImportToDirectDB doesn't use it.
		// Or better, we could make the methods check for nil.
		// For now, let's just ignore the error here.
		// log.Printf("Warning: Failed to init S3 client: %v", err)
	}

	idGen, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to init id generator: %w", err)
	}

	return &DirectDBImporter{
		PG:    db,
		S3:    s3c,
		IDGen: idGen,
		Report: &RunReport{
			Timestamp:       time.Now(),
			SkippedHosts:    []SkippedItem{},
			ModifiedDomains: []ModifiedItem{},
		},
	}, nil
}

// SaveReport writes the run report to the specified path
func (s *DirectDBImporter) SaveReport(path string) error {
	data, err := json.MarshalIndent(s.Report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseJiscTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty string")
	}
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try SQL format seen in JISC export (YYYY-MM-DD HH:MM:SS)
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		// Assume UTC if generic
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unknown format")
}

// --- Import Contacts ---

func (s *DirectDBImporter) ImportContacts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, error) {
	const batchSize = 1000
	var total int64
	var skipped int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&totalRows); err != nil {
		log.Printf("IngestContacts: Failed to count total rows: %v", err)
	}
	// Count already processed if resuming
	var processedSoFar int64
	if lastKey != "" {
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM contacts WHERE id <= ?", lastKey).Scan(&processedSoFar); err != nil {
			log.Printf("IngestContacts: Failed to count processed rows: %v", err)
		}
	}

	total = processedSoFar // Start total from where we really are relative to source

	for {
		log.Printf("DEBUG: ImportContacts Loop Start. lastKey='%s'", lastKey)
		rows, err := sqliteDB.Query(`SELECT id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update" FROM contacts WHERE id > ? ORDER BY id LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestContacts: Query failed: %v", err)
			return total, skipped, err
		}

		var batch []*dbModels.Contact
		currentBatchMaxKey := ""

		rowCount := 0
		for rows.Next() {
			rowCount++
			var id, roid, voice, fax, email, clid, crrr, crdate, uprr, upDate sql.NullString
			if err := rows.Scan(&id, &roid, &voice, &fax, &email, &clid, &crrr, &crdate, &uprr, &upDate); err != nil {
				rows.Close()
				log.Printf("IngestContacts: Scan failed: %v", err)
				return total, skipped, err
			}
			currentBatchMaxKey = id.String

			// Strict Mapping Logic
			mapClid := func(r string) (string, bool) {
				tr := strings.TrimSpace(r)
				if tr == "" {
					return tr, true
				}
				if clidMap == nil {
					return tr, true
				}
				if v, ok := clidMap[tr]; ok && v != "" {
					return v, true
				}
				return "", false
			}

			mappedClID, ok := mapClid(clid.String)
			if !ok {
				skipped++
				continue
			}

			c := &entities.Contact{
				ID:        entities.ClIDType(id.String),
				RoID:      entities.RoidType(roid.String),
				Email:     email.String,
				ClID:      entities.ClIDType(mappedClID),
				AuthInfo:  "escr0W1mP*rt", // default
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			// Parse dates
			if crdate.Valid {
				if t, err := parseJiscTime(crdate.String); err == nil {
					c.CreatedAt = t
				}
			}
			if upDate.Valid {
				if t, err := parseJiscTime(upDate.String); err == nil {
					c.UpdatedAt = t
				}
			}
			// Map refs
			if crrr.Valid {
				if m, ok := mapClid(crrr.String); ok {
					c.CrRr = entities.ClIDType(m)
				}
			}
			if uprr.Valid {
				if m, ok := mapClid(uprr.String); ok {
					c.UpRr = entities.ClIDType(m)
				}
			}
			if voice.Valid {
				c.Voice = entities.E164Type(voice.String)
			}
			if fax.Valid {
				c.Fax = entities.E164Type(fax.String)
			}

			// Generate valid RoID for persistence
			newID := s.IDGen.GenerateID()
			if r, err := entities.NewRoidType(newID, entities.RoidTypeContact); err == nil {
				c.RoID = r
			} else {
				// Should not happen, but safeguard
				skipped++
				continue
			}

			batch = append(batch, dbModels.ToDBContact(c))
		}
		rows.Close()

		log.Printf("DEBUG: Iterated %d rows. Batch size: %d. currentBatchMaxKey='%s'", rowCount, len(batch), currentBatchMaxKey)

		if len(batch) == 0 {
			if currentBatchMaxKey == "" {
				log.Printf("DEBUG: Break condition met. Empty batch and empty maxKey.")
				break
			}
			log.Printf("DEBUG: Empty batch but maxKey='%s'. Proceeding (Skipping all?).", currentBatchMaxKey)
		}

		if len(batch) > 0 {
			// Direct Insert with Conflict Ignore
			// We must exclude relation slices
			batchLen := len(batch)
			if _, err := s.PG.Model(&batch).
				ExcludeColumn("doms_where_registrant", "doms_where_admin", "doms_where_tech", "doms_where_billing").
				OnConflict("DO NOTHING").Insert(); err != nil {
				return total, skipped, fmt.Errorf("bulk insert contacts failed: %w", err)
			}
			total += int64(batchLen)
		}

		if (total+skipped)%10000 == 0 {
			log.Printf("IngestContacts: Processed %d / %d records (Skipped: %d)", total, totalRows, skipped)
		}

		lastKey = currentBatchMaxKey
		if lastKey == "" && len(batch) > 0 {
			lastKey = batch[len(batch)-1].ID
		}

		// Create JSON payload for heartbeat
		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestContacts: Finished. Total: %d, Skipped: %d", total, skipped)
	return total, skipped, nil
}

// --- Import Hosts ---

func (s *DirectDBImporter) ImportHosts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, error) {
	const batchSize = 2500
	var total int64
	var skipped int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&totalRows); err != nil {
		log.Printf("IngestHosts: Failed to count total rows: %v", err)
	}
	// Count already processed if resuming
	var processedSoFar int64
	if lastKey != "" {
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM hosts WHERE name <= ?", lastKey).Scan(&processedSoFar); err != nil {
			log.Printf("IngestHosts: Failed to count processed rows: %v", err)
		}
	}

	total = processedSoFar

	for {
		rows, err := sqliteDB.Query(`SELECT name, clid, crrr, uprr FROM hosts WHERE name > ? ORDER BY name LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestHosts: Query failed: %v", err)
			return total, skipped, err
		}

		var entitiesBatch []*entities.Host
		currentBatchMaxKey := ""

		type rawHost struct {
			Name, ClID, CrRr, UpRr string
		}
		var rawHosts []rawHost

		for rows.Next() {
			var name, clid, crrr, uprr sql.NullString
			if err := rows.Scan(&name, &clid, &crrr, &uprr); err != nil {
				rows.Close()
				log.Printf("IngestHosts: Scan failed: %v", err)
				return total, skipped, err
			}
			rawHosts = append(rawHosts, rawHost{name.String, clid.String, crrr.String, uprr.String})
			currentBatchMaxKey = name.String
		}
		rows.Close()

		if len(rawHosts) == 0 {
			break
		}

		first := rawHosts[0].Name
		last := rawHosts[len(rawHosts)-1].Name

		addrMap := make(map[string][]string)
		if aRows, err := sqliteDB.Query(`SELECT host_name, ip_address FROM host_addresses WHERE host_name >= ? AND host_name <= ?`, first, last); err == nil {
			for aRows.Next() {
				var hn, ip sql.NullString
				if err := aRows.Scan(&hn, &ip); err == nil && hn.Valid {
					addrMap[hn.String] = append(addrMap[hn.String], ip.String)
				}
			}
			aRows.Close()
		}

		statusMap := make(map[string]entities.HostStatus)
		if sRows, err := sqliteDB.Query(`SELECT host_name, status FROM host_statuses WHERE host_name >= ? AND host_name <= ?`, first, last); err == nil {
			for sRows.Next() {
				var hn, st sql.NullString
				if err := sRows.Scan(&hn, &st); err == nil && hn.Valid {
					hs := statusMap[hn.String]
					val := strings.ToLower(st.String)
					if val == "ok" {
						hs.OK = true
					}
					if val == "linked" {
						hs.Linked = true
					}
					statusMap[hn.String] = hs
				}
			}
			sRows.Close()
		}

		for _, r := range rawHosts {
			mapClid := func(v string) (string, bool) {
				tv := strings.TrimSpace(v)
				if tv == "" {
					return tv, true
				}
				if clidMap == nil {
					return tv, true
				}
				if mv, ok := clidMap[tv]; ok {
					return mv, true
				}
				return "", false
			}

			mClID, ok := mapClid(r.ClID)
			if !ok {
				skipped++
				continue
			}

			dn, err := entities.NewDomainName(r.Name)
			if err != nil || dn == nil {
				msg := fmt.Sprintf("Invalid name (ClID: %s)", r.ClID)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				s.Report.SkippedHosts = append(s.Report.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				skipped++
				continue
			}
			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeHost)
			if err != nil {
				msg := fmt.Sprintf("Failed to generate RoID: %v", err)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				s.Report.SkippedHosts = append(s.Report.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				skipped++
				continue
			}

			h := &entities.Host{
				Name:      *dn,
				ClID:      entities.ClIDType(mClID),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
				RoID:      roid,
			}
			if m, ok := mapClid(r.CrRr); ok {
				h.CrRr = entities.ClIDType(m)
			}
			if m, ok := mapClid(r.UpRr); ok {
				h.UpRr = entities.ClIDType(m)
			}

			addrs := addrMap[r.Name]
			for _, ip := range addrs {
				if parsed, err := netip.ParseAddr(ip); err == nil {
					h.Addresses = append(h.Addresses, parsed)
				}
			}
			h.Status = statusMap[r.Name]

			entitiesBatch = append(entitiesBatch, h)
		}

		if len(entitiesBatch) > 0 {
			var dbHosts []*dbModels.Host
			var dbAddrs []*dbModels.HostAddress
			batchLen := len(entitiesBatch)

			for _, ent := range entitiesBatch {
				dbH := dbModels.ToDBHost(ent)
				dbHosts = append(dbHosts, dbH)
				for _, a := range dbH.Addresses {
					val := a
					dbAddrs = append(dbAddrs, &val)
				}
			}

			if _, err := s.PG.Model(&dbHosts).ExcludeColumn("addresses").OnConflict("DO NOTHING").Insert(); err != nil {
				return total, skipped, fmt.Errorf("bulk insert hosts failed: %w", err)
			}
			if len(dbAddrs) > 0 {
				if _, err := s.PG.Model(&dbAddrs).OnConflict("DO NOTHING").Insert(); err != nil {
					return total, skipped, fmt.Errorf("bulk insert host addresses failed: %w", err)
				}
			}
			total += int64(batchLen)
		}

		if (total+skipped)%10000 == 0 {
			log.Printf("IngestHosts: Processed %d / %d records (Skipped: %d)", total, totalRows, skipped)
		}

		lastKey = currentBatchMaxKey

		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestHosts: Finished. Total: %d, Skipped: %d", total, skipped)
	return total, skipped, nil
}

// --- Import Domains ---

func (s *DirectDBImporter) ImportDomains(ctx context.Context, sqliteDB *sql.DB, tld string, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, error) {
	const batchSize = 2500
	var total int64
	var skipped int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM domains").Scan(&totalRows); err != nil {
		log.Printf("IngestDomains: Failed to count total rows: %v", err)
	}
	// Count already processed if resuming
	var processedSoFar int64
	if lastKey != "" {
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM domains WHERE name <= ?", lastKey).Scan(&processedSoFar); err != nil {
			log.Printf("IngestDomains: Failed to count processed rows: %v", err)
		}
	}

	total = processedSoFar

	for {
		rows, err := sqliteDB.Query(`SELECT name, registrant, clid, crrr, crdate, exdate, uprr, uname, originalname FROM domains WHERE name > ? ORDER BY name LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestDomains: Query failed: %v", err)
			return total, skipped, err
		}

		var rawDomains []struct {
			Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org string
		}
		currentBatchMaxKey := ""

		for rows.Next() {
			var r struct {
				Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org sql.NullString
			}
			if err := rows.Scan(&r.Name, &r.Reg, &r.ClID, &r.CrRr, &r.CrDate, &r.ExDate, &r.UpRr, &r.UName, &r.Org); err != nil {
				rows.Close()
				log.Printf("IngestDomains: Scan failed: %v", err)
				return total, skipped, err
			}
			rawDomains = append(rawDomains, struct{ Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org string }{
				r.Name.String, r.Reg.String, r.ClID.String, r.CrRr.String, r.CrDate.String, r.ExDate.String, r.UpRr.String, r.UName.String, r.Org.String,
			})
			currentBatchMaxKey = r.Name.String
		}
		rows.Close()

		if len(rawDomains) == 0 {
			break
		}

		var entitiesBatch []*entities.Domain

		for _, r := range rawDomains {
			mapClid := func(v string) (string, bool) {
				tv := strings.TrimSpace(v)
				if tv == "" {
					return tv, true
				}
				if clidMap == nil {
					return tv, true
				}
				if mv, ok := clidMap[tv]; ok {
					return mv, true
				}
				return "", false
			}
			mClID, ok := mapClid(r.ClID)
			if !ok {
				skipped++
				continue
			}

			dn, _ := entities.NewDomainName(r.Name)
			tldn, _ := entities.NewDomainName(tld)

			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeDomain)
			if err != nil {
				skipped++
				continue
			}

			d := &entities.Domain{
				Name:         *dn,
				TLDName:      *tldn,
				ClID:         entities.ClIDType(mClID),
				AuthInfo:     "escr0W1mP*rt",
				RegistrantID: entities.ClIDType(r.Reg),
				UName:        entities.DomainName(r.UName),
				RoID:         roid,
				UpdatedAt:    time.Now().UTC(),
			}
			if t, err := parseJiscTime(r.CrDate); err == nil {
				d.CreatedAt = t
			} else {
				d.CreatedAt = time.Now().UTC()
			}
			if t, err := parseJiscTime(r.ExDate); err == nil && !t.IsZero() {
				d.ExpiryDate = t
			} else {
				// Fallback to avoid not-null constraint failure
				fallback := time.Now().AddDate(1, 0, 0).UTC()
				msg := fmt.Sprintf("Setting fallback: %s. Parse Err: %v", fallback, err)
				log.Printf("MODIFY DOMAIN: '%s' missing/invalid ExpiryDate ('%s'). %s", d.Name, r.ExDate, msg)
				s.Report.ModifiedDomains = append(s.Report.ModifiedDomains, ModifiedItem{
					Name:          d.Name.String(),
					Field:         "ExpiryDate",
					OriginalValue: r.ExDate,
					Info:          msg,
				})
				d.ExpiryDate = fallback
			}
			if m, ok := mapClid(r.CrRr); ok {
				d.CrRr = entities.ClIDType(m)
			}
			if m, ok := mapClid(r.UpRr); ok {
				d.UpRr = entities.ClIDType(m)
			}

			entitiesBatch = append(entitiesBatch, d)
		}

		if len(entitiesBatch) > 0 {
			var dbBatch []*dbModels.Domain
			batchLen := len(entitiesBatch)
			for _, ent := range entitiesBatch {
				dbBatch = append(dbBatch, dbModels.ToDBDomain(ent))
			}

			if _, err := s.PG.Model(&dbBatch).
				ExcludeColumn("hosts", "tld").
				OnConflict("DO NOTHING").Insert(); err != nil {
				return total, skipped, fmt.Errorf("bulk insert domains failed: %w", err)
			}
			total += int64(batchLen)
		}

		if (total+skipped)%10000 == 0 {
			log.Printf("IngestDomains: Processed %d / %d records (Skipped: %d)", total, totalRows, skipped)
		}

		lastKey = currentBatchMaxKey
		if len(entitiesBatch) > 0 {
			// lastKey should be string representation of name for cursor
			lastKey = entitiesBatch[len(entitiesBatch)-1].Name.String()
		}

		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestDomains: Finished. Total: %d, Skipped: %d", total, skipped)
	return total, skipped, nil
}

// --- Link Domain Hosts ---

func (s *DirectDBImporter) LinkDomainHosts(ctx context.Context, sqliteDB *sql.DB, lastKey string, heartbeat func(processed string)) (int64, error) {
	const batchSize = 5000
	var total int64
	var lastDomain, lastNS string

	if lastKey != "" {
		parts := strings.SplitN(lastKey, "|", 2)
		if len(parts) == 2 {
			lastDomain, lastNS = parts[0], parts[1]
		}
	}

	for {
		rows, err := sqliteDB.Query(`SELECT domain_name, nameserver FROM domain_nameservers WHERE (domain_name > ?) OR (domain_name = ? AND nameserver > ?) ORDER BY domain_name, nameserver LIMIT ?`, lastDomain, lastDomain, lastNS, batchSize)
		if err != nil {
			return total, err
		}

		type link struct {
			DomainName string
			HostName   string
		}
		var links []link

		for rows.Next() {
			var d, n sql.NullString
			if err := rows.Scan(&d, &n); err != nil {
				rows.Close()
				return total, err
			}
			if d.Valid && n.Valid {
				links = append(links, link{d.String, n.String})
				lastDomain = d.String
				lastNS = n.String
			}
		}
		rows.Close()

		if len(links) == 0 {
			break
		}

		var valueStrings []string
		var args []interface{}
		for _, l := range links {
			valueStrings = append(valueStrings, "(?, ?)")
			args = append(args, l.DomainName, l.HostName)
		}

		// Join table insert
		q := fmt.Sprintf(`
			INSERT INTO domain_hosts (domain_ro_id, host_ro_id)
			SELECT d.ro_id, h.ro_id 
			FROM (VALUES %s) AS v(dn, hn)
			JOIN domains d ON d.name = v.dn
			JOIN hosts h ON h.name = v.hn
			ON CONFLICT DO NOTHING
		`, strings.Join(valueStrings, ","))

		if _, err := s.PG.Exec(q, args...); err != nil {
			return total, fmt.Errorf("bulk link failed: %w", err)
		}

		total += int64(len(links))
		heartbeat(fmt.Sprintf("%s|%s", lastDomain, lastNS))
	}

	return total, nil
}
