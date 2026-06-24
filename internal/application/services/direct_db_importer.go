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
	dbModels "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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

func (s *DirectDBImporter) ImportContacts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, int64, error) {
	const batchSize = 1000
	var total int64
	var inserted int64
	var updated int64

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

	total = processedSoFar

	for {
		log.Printf("DEBUG: ImportContacts Loop Start. lastKey='%s'", lastKey)
		rows, err := sqliteDB.Query(`SELECT id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update" FROM contacts WHERE id > ? ORDER BY id LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestContacts: Query failed: %v", err)
			return total, inserted, updated, err
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
				return total, inserted, updated, err
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
				// Record skipped (unmapped CLID or invalid data)
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
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			batch = append(batch, dbModels.ToDBContact(c))
		}
		rows.Close()

		if len(batch) == 0 {
			if currentBatchMaxKey == "" {
				break
			}
		}

		if len(batch) > 0 {
			// Count existing records to track inserts vs updates
			var existingCount int64
			var ids []string
			for _, c := range batch {
				ids = append(ids, c.ID)
			}
			if _, err := s.PG.Query(pg.Scan(&existingCount), "SELECT COUNT(*) FROM contacts WHERE id IN (?)", pg.In(ids)); err != nil {
				log.Printf("IngestContacts: pre-count query failed: %v", err)
			}

			// Upsert: insert new, update existing (preserve ro_id, auth_info, created_at)
			batchLen := len(batch)
			if _, err := s.PG.Model(&batch).
				ExcludeColumn("doms_where_registrant", "doms_where_admin", "doms_where_tech", "doms_where_billing").
				OnConflict("(id) DO UPDATE").
				Set("voice = EXCLUDED.voice, fax = EXCLUDED.fax, email = EXCLUDED.email, cl_id = EXCLUDED.cl_id, cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, updated_at = EXCLUDED.updated_at").
				Insert(); err != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert contacts failed: %w", err)
			}
			newInserts := int64(batchLen) - existingCount
			if newInserts < 0 {
				newInserts = 0
			}
			inserted += newInserts
			updated += existingCount
			total += int64(batchLen)
		}

		if (total)%10000 == 0 {
			log.Printf("IngestContacts: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = currentBatchMaxKey
		if lastKey == "" && len(batch) > 0 {
			lastKey = batch[len(batch)-1].ID
		}

		// Create JSON payload for heartbeat
		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestContacts: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
}

// --- Import Hosts ---

func (s *DirectDBImporter) ImportHosts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, int64, error) {
	const batchSize = 2500
	var total int64
	var inserted int64
	var updated int64

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
			return total, inserted, updated, err
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
				return total, inserted, updated, err
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
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			dn, err := entities.NewDomainName(r.Name)
			if err != nil || dn == nil {
				msg := fmt.Sprintf("Invalid name (ClID: %s)", r.ClID)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				s.Report.SkippedHosts = append(s.Report.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				// Record skipped (unmapped CLID or invalid data)
				continue
			}
			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeHost)
			if err != nil {
				msg := fmt.Sprintf("Failed to generate RoID: %v", err)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				s.Report.SkippedHosts = append(s.Report.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				// Record skipped (unmapped CLID or invalid data)
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

			// Count existing records to track inserts vs updates
			var existingCount int64
			var names []string
			for _, h := range dbHosts {
				names = append(names, h.Name)
			}
			if _, err := s.PG.Query(pg.Scan(&existingCount), "SELECT COUNT(*) FROM hosts WHERE name IN (?)", pg.In(names)); err != nil {
				log.Printf("IngestHosts: pre-count query failed: %v", err)
			}

			// Upsert: insert new, update existing (preserve ro_id, in_bailiwick, created_at)
			if _, err := s.PG.Model(&dbHosts).ExcludeColumn("addresses").
				OnConflict("(name, cl_id) DO UPDATE").
				Set("cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, updated_at = EXCLUDED.updated_at, ok = EXCLUDED.ok, linked = EXCLUDED.linked, pending_create = EXCLUDED.pending_create, pending_delete = EXCLUDED.pending_delete, pending_update = EXCLUDED.pending_update, pending_transfer = EXCLUDED.pending_transfer, client_delete_prohibited = EXCLUDED.client_delete_prohibited, client_update_prohibited = EXCLUDED.client_update_prohibited, server_delete_prohibited = EXCLUDED.server_delete_prohibited, server_update_prohibited = EXCLUDED.server_update_prohibited").
				Insert(); err != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert hosts failed: %w", err)
			}
			if len(dbAddrs) > 0 {
				if _, err := s.PG.Model(&dbAddrs).OnConflict("DO NOTHING").Insert(); err != nil {
					return total, inserted, updated, fmt.Errorf("bulk insert host addresses failed: %w", err)
				}
			}
			newInserts := int64(batchLen) - existingCount
			if newInserts < 0 {
				newInserts = 0
			}
			inserted += newInserts
			updated += existingCount
			total += int64(batchLen)
		}

		if (total)%10000 == 0 {
			log.Printf("IngestHosts: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = currentBatchMaxKey

		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestHosts: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
}

// --- Import Domains ---

func (s *DirectDBImporter) ImportDomains(ctx context.Context, sqliteDB *sql.DB, tld string, clidMap map[string]string, lastKey string, heartbeat func(processed string)) (int64, int64, int64, error) {
	const batchSize = 2500
	var total int64
	var inserted int64
	var updated int64

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
			return total, inserted, updated, err
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
				return total, inserted, updated, err
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

		first := rawDomains[0].Name
		last := rawDomains[len(rawDomains)-1].Name

		statusMap := make(map[string]entities.DomainStatus)
		if sRows, err := sqliteDB.Query(`SELECT domain_name, status FROM domain_statuses WHERE domain_name >= ? AND domain_name <= ?`, first, last); err == nil {
			for sRows.Next() {
				var dn, st sql.NullString
				if err := sRows.Scan(&dn, &st); err == nil && dn.Valid && st.Valid {
					ds := statusMap[dn.String]

					// Normalize status string fully to handle case, snake_case, or kebab-case
					normalizedStatus := strings.ToLower(st.String)
					normalizedStatus = strings.ReplaceAll(normalizedStatus, "_", "")
					normalizedStatus = strings.ReplaceAll(normalizedStatus, "-", "")

					switch normalizedStatus {
					case "ok":
						ds.OK = true
					case "inactive":
						ds.Inactive = true
					case "clienttransferprohibited":
						ds.ClientTransferProhibited = true
					case "clientupdateprohibited":
						ds.ClientUpdateProhibited = true
					case "clientdeleteprohibited":
						ds.ClientDeleteProhibited = true
					case "clientrenewprohibited":
						ds.ClientRenewProhibited = true
					case "clienthold":
						ds.ClientHold = true
					case "servertransferprohibited":
						ds.ServerTransferProhibited = true
					case "serverupdateprohibited":
						ds.ServerUpdateProhibited = true
					case "serverdeleteprohibited":
						ds.ServerDeleteProhibited = true
					case "serverrenewprohibited":
						ds.ServerRenewProhibited = true
					case "serverhold":
						ds.ServerHold = true
					case "pendingcreate":
						ds.PendingCreate = true
					case "pendingrenew":
						ds.PendingRenew = true
					case "pendingtransfer":
						ds.PendingTransfer = true
					case "pendingupdate":
						ds.PendingUpdate = true
					case "pendingrestore":
						ds.PendingRestore = true
					case "pendingdelete":
						ds.PendingDelete = true
					}
					statusMap[dn.String] = ds
				}
			}
			sRows.Close()
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
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			dn, _ := entities.NewDomainName(r.Name)
			tldn, _ := entities.NewDomainName(tld)

			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeDomain)
			if err != nil {
				// Record skipped (unmapped CLID or invalid data)
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

			if st, ok := statusMap[r.Name]; ok {
				d.Status = st
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

			// Count existing records to track inserts vs updates
			var existingCount int64
			var names []string
			for _, d := range dbBatch {
				names = append(names, d.Name)
			}
			if _, err := s.PG.Query(pg.Scan(&existingCount), "SELECT COUNT(*) FROM domains WHERE name IN (?)", pg.In(names)); err != nil {
				log.Printf("IngestDomains: pre-count query failed: %v", err)
			}

			// Upsert: insert new, update existing (preserve ro_id, auth_info, created_at)
			if _, err := s.PG.Model(&dbBatch).
				ExcludeColumn("hosts", "tld").
				OnConflict("(name) DO UPDATE").
				Set("cl_id = EXCLUDED.cl_id, cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, registrant_id = EXCLUDED.registrant_id, expiry_date = EXCLUDED.expiry_date, u_name = EXCLUDED.u_name, original_name = EXCLUDED.original_name, drop_catch = EXCLUDED.drop_catch, renewed_years = EXCLUDED.renewed_years, updated_at = EXCLUDED.updated_at, tld_name = EXCLUDED.tld_name, ok = EXCLUDED.ok, inactive = EXCLUDED.inactive, client_transfer_prohibited = EXCLUDED.client_transfer_prohibited, client_update_prohibited = EXCLUDED.client_update_prohibited, client_delete_prohibited = EXCLUDED.client_delete_prohibited, client_renew_prohibited = EXCLUDED.client_renew_prohibited, client_hold = EXCLUDED.client_hold, server_transfer_prohibited = EXCLUDED.server_transfer_prohibited, server_update_prohibited = EXCLUDED.server_update_prohibited, server_delete_prohibited = EXCLUDED.server_delete_prohibited, server_renew_prohibited = EXCLUDED.server_renew_prohibited, server_hold = EXCLUDED.server_hold, pending_create = EXCLUDED.pending_create, pending_renew = EXCLUDED.pending_renew, pending_transfer = EXCLUDED.pending_transfer, pending_update = EXCLUDED.pending_update, pending_restore = EXCLUDED.pending_restore, pending_delete = EXCLUDED.pending_delete").
				Insert(); err != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert domains failed: %w", err)
			}
			newInserts := int64(batchLen) - existingCount
			if newInserts < 0 {
				newInserts = 0
			}
			inserted += newInserts
			updated += existingCount
			total += int64(batchLen)
		}

		if (total)%10000 == 0 {
			log.Printf("IngestDomains: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = currentBatchMaxKey
		if len(entitiesBatch) > 0 {
			// lastKey should be string representation of name for cursor
			lastKey = entitiesBatch[len(entitiesBatch)-1].Name.String()
		}

		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestDomains: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
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

// --- Import NNDNs ---

func (s *DirectDBImporter) ImportNNDNs(ctx context.Context, sqliteDB *sql.DB, tld string, lastKey string, heartbeat func(processed string)) (int64, int64, int64, error) {
	const batchSize = 2500
	var total int64
	var inserted int64
	var updated int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM nndns").Scan(&totalRows); err != nil {
		log.Printf("IngestNNDNs: Failed to count total rows: %v", err)
	}
	// Count already processed if resuming
	var processedSoFar int64
	if lastKey != "" {
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM nndns WHERE aname <= ?", lastKey).Scan(&processedSoFar); err != nil {
			log.Printf("IngestNNDNs: Failed to count processed rows: %v", err)
		}
	}

	total = processedSoFar

	for {
		rows, err := sqliteDB.Query(`SELECT aname, uname, idntableid, originalname, namestate, crdate FROM nndns WHERE aname > ? ORDER BY aname LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestNNDNs: Query failed: %v", err)
			return total, inserted, updated, err
		}

		var rawNNDNs []struct {
			AName, UName, IDNTableID, OriginalName, NameState, CrDate string
		}
		currentBatchMaxKey := ""

		for rows.Next() {
			var r struct {
				AName, UName, IDNTableID, OriginalName, NameState, CrDate sql.NullString
			}
			if err := rows.Scan(&r.AName, &r.UName, &r.IDNTableID, &r.OriginalName, &r.NameState, &r.CrDate); err != nil {
				rows.Close()
				log.Printf("IngestNNDNs: Scan failed: %v", err)
				return total, inserted, updated, err
			}
			rawNNDNs = append(rawNNDNs, struct{ AName, UName, IDNTableID, OriginalName, NameState, CrDate string }{
				r.AName.String, r.UName.String, r.IDNTableID.String, r.OriginalName.String, r.NameState.String, r.CrDate.String,
			})
			currentBatchMaxKey = r.AName.String
		}
		rows.Close()

		if len(rawNNDNs) == 0 {
			break
		}

		var entitiesBatch []*dbModels.NNDN

		for _, r := range rawNNDNs {
			createdAt := time.Now().UTC()
			if t, err := parseJiscTime(r.CrDate); err == nil && !t.IsZero() {
				createdAt = t
			}

			n := &dbModels.NNDN{
				Name:      strings.ToLower(r.AName),
				UName:     r.UName,
				TLDName:   strings.ToLower(tld),
				NameState: r.NameState,
				Reason:    "", // Not mapped from escrow directly
				CreatedAt: createdAt,
				UpdatedAt: time.Now().UTC(),
			}

			entitiesBatch = append(entitiesBatch, n)
		}

		if len(entitiesBatch) > 0 {
			batchLen := len(entitiesBatch)

			// Count existing records to track inserts vs updates
			var existingCount int64
			var names []string
			for _, n := range entitiesBatch {
				names = append(names, n.Name)
			}
			if _, err := s.PG.Query(pg.Scan(&existingCount), "SELECT COUNT(*) FROM nndns WHERE name IN (?)", pg.In(names)); err != nil {
				log.Printf("IngestNNDNs: pre-count query failed: %v", err)
			}

			// Upsert: insert new, update existing (preserve created_at)
			if _, err := s.PG.Model(&entitiesBatch).
				ExcludeColumn("tld").
				OnConflict("(name) DO UPDATE").
				Set("name_state = EXCLUDED.name_state, u_name = EXCLUDED.u_name, updated_at = EXCLUDED.updated_at").
				Insert(); err != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert NNDNs failed: %w", err)
			}
			newInserts := int64(batchLen) - existingCount
			if newInserts < 0 {
				newInserts = 0
			}
			inserted += newInserts
			updated += existingCount
			total += int64(batchLen)
		}

		if (total)%10000 == 0 {
			log.Printf("IngestNNDNs: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = currentBatchMaxKey

		payload := fmt.Sprintf(`{"lastKey":"%s","processed":%d,"total":%d}`, lastKey, total, totalRows)
		heartbeat(payload)
	}
	log.Printf("IngestNNDNs: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
}

// --- Accredit Registrars ---

// AccreditRegistrars accredits all unique registrars mapped in the staging DB for the given TLD
func (s *DirectDBImporter) AccreditRegistrars(ctx context.Context, sqliteDB *sql.DB, tld string, heartbeat func(processed string)) (int64, error) {
	var total int64

	// We MUST query registrar_mapping to get the actual Postgres registrar_cl_id.
	// Querying the registrars table directly might return the escrow ID, violating the FK constraint.
	rows, err := sqliteDB.Query(`SELECT DISTINCT registrar_clid FROM registrar_mapping WHERE registrar_clid IS NOT NULL AND registrar_clid != ''`)
	if err != nil {
		log.Printf("AccreditRegistrars: Query failed: %v", err)
		return 0, nil // Graceful exit to not block ingestion if registrar_mapping table is missing
	}
	defer rows.Close()

	var clIDs []string
	for rows.Next() {
		var clid sql.NullString
		if err := rows.Scan(&clid); err != nil {
			log.Printf("AccreditRegistrars: Scan failed: %v", err)
			return total, err
		}
		if clid.Valid && strings.TrimSpace(clid.String) != "" {
			clIDs = append(clIDs, strings.TrimSpace(clid.String))
		}
	}

	if len(clIDs) == 0 {
		log.Printf("AccreditRegistrars: Finished. No registrars found to accredit.")
		return 0, nil
	}

	const batchSize = 1000
	for i := 0; i < len(clIDs); i += batchSize {
		end := i + batchSize
		if end > len(clIDs) {
			end = len(clIDs)
		}

		batch := clIDs[i:end]
		var valueStrings []string
		var args []interface{}

		for _, clid := range batch {
			valueStrings = append(valueStrings, "(?, ?)")
			args = append(args, strings.ToLower(tld), clid)
		}

		q := fmt.Sprintf(`
			INSERT INTO accreditations (tld_name, registrar_cl_id) 
			VALUES %s 
			ON CONFLICT DO NOTHING
		`, strings.Join(valueStrings, ","))

		res, err := s.PG.Exec(q, args...)
		if err != nil {
			return total, fmt.Errorf("bulk insert accreditations failed: %w", err)
		}

		total += int64(res.RowsAffected())
		heartbeat(fmt.Sprintf("%d/%d", i+len(batch), len(clIDs)))
	}

	log.Printf("AccreditRegistrars: Finished. Total new accreditations: %d", total)
	return total, nil
}
