package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/api"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities/jisc"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// JiscService handles the import and analysis of JISC domain data
type JiscService struct {
	db *sql.DB
}

// NewJiscService creates a new JiscService
func NewJiscService() *JiscService {
	return &JiscService{}
}

// GenerateEscrowDB generates a standard escrow SQLite DB from JISC JSON
func (s *JiscService) GenerateEscrowDB(jsonPath string) error {
	// Determine output filename: same as jsonPath but .db
	dbPath := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + ".db"

	log.Printf("Generating detailed escrow DB at: %s", dbPath)
	os.Remove(dbPath) // Start fresh

	var err error
	s.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer s.db.Close()

	// Create Standard Escrow Schema
	if err := s.createEscrowSchema(); err != nil {
		return fmt.Errorf("failed to create escrow schema: %w", err)
	}

	// Begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Prepare statements
	stmtRegistrar, _ := tx.Prepare(`INSERT OR IGNORE INTO registrars (id, name, status, crdate) VALUES (?, ?, 'ok', datetime('now'))`)
	stmtContact, _ := tx.Prepare(`INSERT INTO contacts (id, clid, voice, fax, email, crdate) VALUES (?, ?, ?, ?, ?, datetime('now'))`)
	stmtPostal, _ := tx.Prepare(`INSERT INTO contact_postal_info (contact_id, type, name, org, street1, city, postal_code, country_code) VALUES (?, 'loc', ?, ?, ?, ?, ?, ?)`)
	stmtDomain, _ := tx.Prepare(`INSERT INTO domains (name, roid, registrant, clid, crdate, exdate, status) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	stmtNS, _ := tx.Prepare(`INSERT INTO domain_nameservers (domain_name, nameserver) VALUES (?, ?)`)
	stmtStatus, _ := tx.Prepare(`INSERT INTO domain_statuses (domain_name, status) VALUES (?, ?)`)
	stmtHost, _ := tx.Prepare(`INSERT OR IGNORE INTO hosts (name, clid, crrr, uprr) VALUES (?, ?, ?, ?)`)
	stmtHostAddr, _ := tx.Prepare(`INSERT OR IGNORE INTO host_addresses (host_name, ip_address) VALUES (?, ?)`)
	stmtHostStatus, _ := tx.Prepare(`INSERT OR IGNORE INTO host_statuses (host_name, status) VALUES (?, ?)`)

	defer stmtRegistrar.Close()
	defer stmtContact.Close()
	defer stmtPostal.Close()
	defer stmtDomain.Close()
	defer stmtNS.Close()
	defer stmtStatus.Close()
	defer stmtHost.Close()
	defer stmtHostAddr.Close()
	defer stmtHostStatus.Close()

	log.Printf("Importing data from %s...", jsonPath)
	file, err := os.Open(jsonPath)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	// Read opening bracket
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("expected start of JSON array")
	}

	count := 0
	for dec.More() {
		var d jisc.JiscDomain
		if err := dec.Decode(&d); err != nil {
			return err
		}

		// 1. Registrar
		registrarID := fmt.Sprintf("%d", d.Registrar.JoID)
		if _, err := stmtRegistrar.Exec(registrarID, d.Registrar.Name); err != nil {
			log.Printf("Error inserting registrar %s: %v", registrarID, err)
		}

		// 2. Contact (Registrant)
		// Generate a unique contact ID since source often has -1
		contactID := fmt.Sprintf("C-%d-%d", d.DomainID, d.Registrant.ContactID)
		if d.Registrant.ContactID == -1 {
			// If -1, maybe make it unique per domain or per registrant hash?
			// Simple approach: C-<DomainID> to ensure 1:1 map as implied by nested structure
			contactID = fmt.Sprintf("C-%d", d.DomainID)
		}

		_, err = stmtContact.Exec(contactID, registrarID, d.Registrant.Phone, d.Registrant.Fax, d.Registrant.Email)
		if err != nil {
			log.Printf("Error inserting contact %s: %v", contactID, err)
		}

		_, err = stmtPostal.Exec(contactID, d.Registrant.Name, d.Registrant.CompanyName, d.Registrant.Address, d.Registrant.City, d.Registrant.PCode, d.Registrant.Country)
		if err != nil {
			log.Printf("Error inserting postal %s: %v", contactID, err)
		}

		// 3. Domain
		// Map Status: ACTIVE -> ok? The escrow schema uses EPP statuses usually.
		// JISC has "ACTIVE", "PENDING_CREATE", etc.
		// We can just store the JISC status or map it. Escrow usually expects EPP like 'ok', 'clientHold'.
		// Storing as is for now.
		domainID := fmt.Sprintf("%d", d.DomainID)
		_, err = stmtDomain.Exec(d.DomainName, domainID, contactID, registrarID, d.RegisteredDate, d.RegisterExpireDate, d.Status)
		if err != nil {
			log.Printf("Error inserting domain %s: %v", d.DomainName, err)
		}

		// 4. Nameservers & Hosts
		dnsList := []string{d.DNS1, d.DNS2, d.DNS3, d.DNS4}
		for _, dns := range dnsList {
			dns = strings.TrimSpace(dns)
			if dns != "" {
				parts := strings.Split(dns, ",")
				hostName := strings.TrimSpace(parts[0])
				if hostName == "" {
					continue
				}

				stmtNS.Exec(d.DomainName, hostName)
				// Create Host entry (assuming same registrar as domain for ClID)
				stmtHost.Exec(hostName, registrarID, registrarID, registrarID)

				// Parse IPs
				for i := 1; i < len(parts); i++ {
					ip := strings.TrimSpace(parts[i])
					if ip != "" {
						stmtHostAddr.Exec(hostName, ip)
					}
				}

				// Basic host statuses implied by being part of active domain
				stmtHostStatus.Exec(hostName, "ok")
				stmtHostStatus.Exec(hostName, "linked")
			}
		}

		// 5. Status
		stmtStatus.Exec(d.DomainName, d.Status)

		count++
		if count%1000 == 0 {
			fmt.Printf("\rProcessed %d domains...", count)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("\rProcessed %d domains. DB created at %s\n", count, dbPath)
	return nil
}

func (s *JiscService) createEscrowSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS domains (
		name TEXT PRIMARY KEY,
		roid TEXT,
		uname TEXT,
		idntableid TEXT,
		originalname TEXT,
		registrant TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		exdate TEXT,
		uprr TEXT,
		"update" TEXT,
		status TEXT -- Added for simple status mapping if not using link table fully
	);

	CREATE TABLE IF NOT EXISTS domain_nameservers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		nameserver TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	CREATE TABLE IF NOT EXISTS domain_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	CREATE TABLE IF NOT EXISTS registrars (
		id TEXT PRIMARY KEY,
		name TEXT,
		gurid INTEGER,
		status TEXT,
		voice TEXT,
		fax TEXT,
		email TEXT,
		url TEXT,
		crdate TEXT,
		"update" TEXT
	);

	CREATE TABLE IF NOT EXISTS contacts (
		id TEXT PRIMARY KEY,
		roid TEXT,
		voice TEXT,
		fax TEXT,
		email TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		uprr TEXT,
		"update" TEXT
	);

	CREATE TABLE IF NOT EXISTS contact_postal_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contact_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT,
		org TEXT,
		street1 TEXT,
		street2 TEXT,
		street3 TEXT,
		city TEXT,
		state_province TEXT,
		postal_code TEXT,
		country_code TEXT,
		FOREIGN KEY (contact_id) REFERENCES contacts(id)
	);

	CREATE INDEX IF NOT EXISTS idx_domains_name ON domains(name);
	CREATE INDEX IF NOT EXISTS idx_domains_registrar ON domains(clid);
	
	CREATE TABLE IF NOT EXISTS hosts (
		name TEXT PRIMARY KEY,
		clid TEXT,
		crrr TEXT,
		uprr TEXT,
		crdate TEXT,
		"update" TEXT
	);

	CREATE TABLE IF NOT EXISTS host_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		FOREIGN KEY (host_name) REFERENCES hosts(name),
		UNIQUE(host_name, ip_address)
	);

	CREATE TABLE IF NOT EXISTS host_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (host_name) REFERENCES hosts(name),
		UNIQUE(host_name, status)
	);
	
	CREATE INDEX IF NOT EXISTS idx_hosts_name ON hosts(name);
	`
	_, err := s.db.Exec(schema)
	return err
}

// ImportToDirectDB generates an escrow DB from JSON then imports it directly to Postgres
func (s *JiscService) ImportToDirectDB(jsonPath string) error {
	// 1. Generate SQLite from JSON
	start := time.Now()
	if err := s.GenerateEscrowDB(jsonPath); err != nil {
		return fmt.Errorf("failed to generate escrow db: %w", err)
	}
	log.Printf("SQLite generation took %v", time.Since(start))

	dbPath := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + ".db"

	// Open SQLite for reading during import
	sqliteDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite db: %w", err)
	}
	defer sqliteDB.Close()

	// 2. Initialize Direct Importer
	importer, err := NewDirectDBImporter()
	if err != nil {
		return fmt.Errorf("failed to init direct importer: %w", err)
	}
	defer importer.PG.Close()

	ctx := context.Background()

	// 3. Build Registrar Mapping (JISC ID/Name -> System ClID)
	// Query Postgres for existing registrars
	log.Println("Building registrar mapping from Postgres...")
	clidMap := make(map[string]string)

	// Query all registrars from Postgres
	// Assuming table is "registrars", columns "name", "cl_id"
	// We map the JISC 'registrarID' (which we stored as d.Registrar.JoID in 'id' column of sqlite) to existing ClIDs.
	// But wait, in GenerateEscrowDB, we stored `d.Registrar.Name` as 'name' and `registrarID` as parsed JoID.
	// We need to map Name -> ClID.

	// Fetch all registrars from PG
	var pgRegistrars []struct {
		Name string
		ClID string
	}
	if _, err := importer.PG.Query(&pgRegistrars, `SELECT name, cl_id FROM registrars`); err != nil {
		return fmt.Errorf("failed to fetch registrars from pg: %w", err)
	}

	// Map: Lowercase Name -> ClID
	nameToClID := make(map[string]string)
	for _, r := range pgRegistrars {
		nameToClID[strings.ToLower(r.Name)] = r.ClID
	}

	// Now we need to map the SQLite 'clid' values (which are JoIDs) to these ClIDs.
	// Read registrars from SQLite
	rows, err := sqliteDB.Query("SELECT id, name FROM registrars")
	if err != nil {
		return fmt.Errorf("failed to read sqlite registrars: %w", err)
	}
	defer rows.Close()

	missingCount := 0
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		if clid, ok := nameToClID[strings.ToLower(name)]; ok {
			clidMap[id] = clid // Map JoID (in sqlite ClID col) -> Real ClID
		} else {
			// Registrar missing in system?
			// We could create it here, or just log warning.
			// DirectDBImporter skips if mapping missing.
			// For now, let's create a deterministic ClID if missing?
			// Or better, fail/warn.
			// Reusing existing logic: "r-<JoID>"
			fallbackClID := fmt.Sprintf("r-%s", id)

			// Check if we can just use this fallback.
			// Ideally we should create the registrar in PG first if it doesn't exist.
			// For this task, let's assume we map to fallback if not found.
			clidMap[id] = fallbackClID
			missingCount++
		}
	}
	log.Printf("Registrar Mapping built. %d registrars mapped (%d missing in PG, mapped to fallback).", len(clidMap), missingCount)

	// 4. Run Import Phases
	noopHeartbeat := func(s string) {} // CLI doesn't need heartbeat callbacks

	log.Println("Importing Contacts...")
	cTotal, cInserted, cUpdated, cSkipped, err := importer.ImportContacts(ctx, sqliteDB, clidMap, "", noopHeartbeat)
	if err != nil {
		return fmt.Errorf("ImportContacts failed: %w", err)
	}
	log.Printf("Contacts: %d total, %d inserted, %d updated, %d skipped", cTotal, cInserted, cUpdated, cSkipped)

	log.Println("Importing Hosts...")
	hTotal, hInserted, hUpdated, err := importer.ImportHosts(ctx, sqliteDB, clidMap, "", noopHeartbeat)
	if err != nil {
		return fmt.Errorf("ImportHosts failed: %w", err)
	}
	log.Printf("Hosts: %d total, %d inserted, %d updated", hTotal, hInserted, hUpdated)

	log.Println("Importing Domains...")
	// TLD is assumed ac.uk from context, but maybe verify/pass?
	// The importer needs tld arg.
	dTotal, dInserted, dUpdated, err := importer.ImportDomains(ctx, sqliteDB, "ac.uk", clidMap, "", noopHeartbeat)
	if err != nil {
		return fmt.Errorf("ImportDomains failed: %w", err)
	}
	log.Printf("Domains: %d total, %d inserted, %d updated", dTotal, dInserted, dUpdated)

	log.Println("Linking Domain Hosts...")
	lTotal, lInserted, err := importer.LinkDomainHosts(ctx, sqliteDB, "", noopHeartbeat)
	if err != nil {
		return fmt.Errorf("LinkDomainHosts failed: %w", err)
	}
	log.Printf("Links: %d total, %d inserted", lTotal, lInserted)

	// Save Run Report
	reportPath := strings.TrimSuffix(jsonPath, filepath.Ext(jsonPath)) + "_runreport.json"
	if err := importer.SaveReport(reportPath); err != nil {
		log.Printf("Warning: Failed to save run report: %v", err)
	} else {
		log.Printf("Run report saved to: %s", reportPath)
	}

	return nil
}

// Analyze performs the full analysis process: import JSON to SQLite and generate report
func (s *JiscService) Analyze(jsonPath string, dbPath string) error {
	// 1. Setup Database
	if dbPath == "" {
		dbPath = ":memory:"
		log.Println("Using in-memory database for analysis")
	} else {
		log.Printf("Using SQLite database at: %s", dbPath)
		// Remove existing DB if it exists to start fresh
		os.Remove(dbPath)
	}

	var err error
	s.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer s.db.Close()

	// 2. Create Schema
	if err := s.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// 3. Import JSON Data
	if err := s.importJSON(jsonPath); err != nil {
		return fmt.Errorf("failed to import JSON data: %w", err)
	}

	// 4. Generate Report
	if err := s.generateReport(); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	return nil
}

func (s *JiscService) createSchema() error {
	schema := `
	CREATE TABLE domains (
		id INTEGER PRIMARY KEY,
		domain_id INTEGER,
		name TEXT,
		registered_date TEXT,
		expire_date TEXT,
		status TEXT,
		registrant_name TEXT,
		registrant_company TEXT,
		registrant_email TEXT,
		registrar_name TEXT,
		registrar_id INTEGER,
		notes TEXT,
		private BOOLEAN
	);

	CREATE TABLE nameservers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_id INTEGER,
		host TEXT,
		FOREIGN KEY(domain_id) REFERENCES domains(domain_id)
	);

	CREATE INDEX idx_domains_name ON domains(name);
	CREATE INDEX idx_domains_registrar ON domains(registrar_name);
	CREATE INDEX idx_domains_registrant_company ON domains(registrant_company);
	`
	_, err := s.db.Exec(schema)
	return err
}

func (s *JiscService) importJSON(jsonPath string) error {
	log.Printf("Importing data from %s...", jsonPath)

	file, err := os.Open(jsonPath)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)

	// Read opening bracket
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("expected start of JSON array")
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmtDomain, err := tx.Prepare(`
		INSERT INTO domains (
			domain_id, name, registered_date, expire_date, status,
			registrant_name, registrant_company, registrant_email,
			registrar_name, registrar_id, notes, private
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmtDomain.Close()

	stmtNS, err := tx.Prepare(`INSERT INTO nameservers (domain_id, host) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmtNS.Close()

	count := 0
	for dec.More() {
		var d jisc.JiscDomain
		if err := dec.Decode(&d); err != nil {
			return err
		}

		_, err := stmtDomain.Exec(
			d.DomainID, d.DomainName, d.RegisteredDate, d.RegisterExpireDate, d.Status,
			d.Registrant.Name, d.Registrant.CompanyName, d.Registrant.Email,
			d.Registrar.Name, d.Registrar.JoID, d.Notes, d.Private,
		)
		if err != nil {
			return fmt.Errorf("failed to insert domain %s: %w", d.DomainName, err)
		}

		// Insert nameservers
		dnsList := []string{d.DNS1, d.DNS2, d.DNS3, d.DNS4}
		for _, dns := range dnsList {
			if dns != "" {
				// Some DNS entries might be comma separated or contain IPs, simple cleanup if needed
				// For now taking as is, or splitting if comma presence?
				// Example: "ns0.diamond.ac.uk,193.62.221.97"
				host := strings.Split(dns, ",")[0]
				_, err := stmtNS.Exec(d.DomainID, host)
				if err != nil {
					return err
				}
			}
		}

		count++
		if count%1000 == 0 {
			fmt.Printf("\rProcessed %d domains...", count)
		}
	}

	fmt.Printf("\rProcessed %d domains. Committing...        \n", count)
	return tx.Commit()
}

func (s *JiscService) generateReport() error {
	log.Println("\n--- Analysis Report ---")

	// Total Domains
	var totalDomains int
	if err := s.db.QueryRow("SELECT count(*) FROM domains").Scan(&totalDomains); err != nil {
		return err
	}
	fmt.Printf("\nTotal Domains: %d\n", totalDomains)

	// Unique Registrars
	var uniqueRegistrars int
	if err := s.db.QueryRow("SELECT count(DISTINCT registrar_name) FROM domains").Scan(&uniqueRegistrars); err != nil {
		return err
	}
	fmt.Printf("Unique Registrars: %d\n", uniqueRegistrars)

	// Domains per Registrar (Top 10)
	fmt.Println("\nTop 10 Registrars by Volume:")
	rows, err := s.db.Query("SELECT registrar_name, count(*) as c FROM domains GROUP BY registrar_name ORDER BY c DESC LIMIT 10")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return err
		}
		fmt.Printf("  - %s: %d\n", name, count)
	}

	// TLD/SLD Breakdown (Simple suffix check)
	// Currently all seem to be .ac.uk based on filename, but checking suffix dist
	fmt.Println("\nSuffix Distribution (Top 10):")
	// Extract suffix. Primitive check: everything after the first dot? or last two parts?
	// Assuming .ac.uk context, but let's see unique endings
	// Using SQLite substr and instr might be complex for full TLD parsing,
	// but we can group by the tail.
	// Let's just do a simple "ends with" check for common ones if needed, OR just count by status

	// Status Breakdown
	fmt.Println("\nStatus Distribution:")
	rowsStatus, err := s.db.Query("SELECT status, count(*) as c FROM domains GROUP BY status ORDER BY c DESC")
	if err != nil {
		return err
	}
	defer rowsStatus.Close()
	for rowsStatus.Next() {
		var status string
		var count int
		rowsStatus.Scan(&status, &count)
		fmt.Printf("  - %s: %d\n", status, count)
	}

	// Nameserver Count
	var totalNS int
	if err := s.db.QueryRow("SELECT count(*) FROM nameservers").Scan(&totalNS); err != nil {
		return err
	}
	fmt.Printf("\nTotal Nameserver Records: %d\n", totalNS)

	// Unique Hosts
	var uniqueHosts int
	if err := s.db.QueryRow("SELECT count(DISTINCT host) FROM nameservers").Scan(&uniqueHosts); err != nil {
		return err
	}
	fmt.Printf("Unique Nameserver Hosts: %d\n", uniqueHosts)

	return nil
}

// ImportToAdminAPI imports JISC data using the Admin API
func (s *JiscService) ImportToAdminAPI(jsonPath, apiURL, token string) error {
	client := api.NewAdminClient(apiURL, token)

	log.Printf("Importing data from %s to API at %s...", jsonPath, apiURL)
	file, err := os.Open(jsonPath)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '[' {
		return fmt.Errorf("expected start of JSON array")
	}

	// Maps for deduplication
	registrars := make(map[string]commands.CreateRegistrarCommand)
	// Cache for existing registrars mapping: RegistrarName (lowercase) -> ClID
	existingRegistrars := make(map[string]string)
	// Set of existing ClIDs for idempotency
	existingIDs := make(map[string]bool)

	// 0. Pre-fetch existing Registrars
	log.Println("Fetching existing registrars...")
	existingRarsList, err := client.ListRegistrars()
	if err != nil {
		return fmt.Errorf("failed to list registrars: %w", err)
	}
	log.Printf("Found %d existing registrars", len(existingRarsList))
	if len(existingRarsList) > 0 {
		log.Printf("Debug: First registrar ClID: '%s', Name: '%s'", existingRarsList[0].ClID, existingRarsList[0].Name)
	}
	for _, r := range existingRarsList {
		existingRegistrars[strings.ToLower(r.Name)] = string(r.ClID)
		existingIDs[string(r.ClID)] = true
	}

	// 0b. Pre-fetch existing Hosts for idempotency
	existingHosts := make(map[string]bool)
	log.Println("Fetching existing hosts...")
	existingHostsList, err := client.ListHosts()
	if err != nil {
		return fmt.Errorf("failed to list hosts: %w", err)
	}
	log.Printf("Found %d existing hosts", len(existingHostsList))
	for _, h := range existingHostsList {
		// Key: hostname|registrarID (lowercase hostname to be safe)
		key := fmt.Sprintf("%s|%s", strings.ToLower(h.Name.String()), h.ClID)
		existingHosts[key] = true
	}

	// 0c. Pre-fetch existing Contacts for idempotency
	existingContacts := make(map[string]bool)
	log.Println("Fetching existing contacts...")
	existingContactsList, err := client.ListContacts()
	if err != nil {
		return fmt.Errorf("failed to list contacts: %w", err)
	}
	log.Printf("Found %d existing contacts", len(existingContactsList))
	for _, c := range existingContactsList {
		existingContacts[string(c.ID)] = true
	}

	contacts := make(map[string]commands.CreateContactCommand)
	hosts := make(map[string]commands.CreateHostCommand)
	var domainCmds []commands.CreateDomainCommand

	// Cache for verified TLDs to avoid spamming API
	verifiedTLDs := make(map[string]bool)

	// Store domain -> hosts mapping for linking later
	// key: domainName, value: []hostName
	domainHosts := make(map[string][]string)

	count := 0
	for dec.More() {
		var d jisc.JiscDomain
		if err := dec.Decode(&d); err != nil {
			return err
		}
		// 0. Verify TLD
		parts := strings.Split(d.DomainName, ".")
		tldName := ""
		if len(parts) >= 2 {
			tldName = strings.Join(parts[len(parts)-2:], ".") // Try last two parts (ac.uk)
		}

		if tldName != "" {
			if _, checked := verifiedTLDs[tldName]; !checked {
				tld, err := client.GetTLD(tldName)
				if err != nil {
					return fmt.Errorf("failed to verify TLD %s: %w", tldName, err)
				}
				if tld == nil {
					// Try single part TLD if 2-part failed? e.g. "uk"
					// Retry with last part
					tldName = parts[len(parts)-1]
					tld, err = client.GetTLD(tldName)
					if err != nil {
						return fmt.Errorf("failed to verify TLD %s: %w", tldName, err)
					}
					if tld == nil {
						return fmt.Errorf("TLD %s does not exist", tldName)
					}
				}

				if !tld.AllowEscrowImport {
					return fmt.Errorf("TLD %s does not allow escrow import", tldName)
				}
				verifiedTLDs[tldName] = true
				log.Printf("Verified TLD %s (EscrowImportEnabled: true)", tldName)
			}
		}

		// 1. Prepare Registrar
		// Check if registrar exists
		var rarClID string
		normalizedRarName := strings.ToLower(d.Registrar.Name)
		if existingID, ok := existingRegistrars[normalizedRarName]; ok {
			rarClID = existingID
		} else {
			// Create new if not mapped/created yet
			// CLID convention: r-<JoID>
			generatedClID := fmt.Sprintf("r-%d", d.Registrar.JoID)
			rarClID = generatedClID

			// Check if we already staged it for creation
			if _, staged := registrars[rarClID]; !staged {
				// IDEMPOTENCY CHECK: If ID exists in DB but name didn't match above,
				// we assume it's the same or we can't create it anyway.
				// We skip creation to avoid 500 duplicate key error.
				if existingIDs[rarClID] {
					// Log optional warning?
					// log.Printf("Skipping creation of %s (ID %s already exists)", d.Registrar.Name, rarClID)
				} else {
					// Create dummy postal info
					addr, _ := entities.NewAddress("Unknown", "GB")
					pi, _ := entities.NewRegistrarPostalInfo(entities.PostalInfoEnumTypeLOC, addr)

					registrars[rarClID] = commands.CreateRegistrarCommand{
						ClID:       rarClID,
						Name:       d.Registrar.Name,
						Email:      "import@placeholder.com", // Required
						PostalInfo: [2]*entities.RegistrarPostalInfo{pi, nil},
						Status:     "ok",
					}
				}
				// Add to existing map so subsequent domains use this new ID
				existingRegistrars[normalizedRarName] = rarClID
			}
		}

		// 2. Prepare Contact
		// ID Logic: if > 0 use c-<ID>, else cd-<DomainID>
		// Short prefixes to ensure < 16 chars (ClIDType limit)
		// Note: JISC format for nested contact might be incomplete
		var contactID string
		if d.Registrant.ContactID > 0 {
			contactID = fmt.Sprintf("c-%d", d.Registrant.ContactID)
		} else {
			contactID = fmt.Sprintf("cd-%d", d.DomainID)
		}

		if len(contactID) > 16 {
			log.Printf("Warning: Generated Contact ID '%s' exceeds 16 chars", contactID)
		}

		if _, exists := contacts[contactID]; !exists {
			// Postal Info
			// Use generic fallback if fields are empty
			params := d.Registrant
			if params.Address == "" {
				params.Address = "Unknown"
			}
			if params.Country == "" {
				params.Country = "GB"
			}
			if params.City == "" {
				params.City = "Unknown"
			}
			// Name fallback (PostalLineType min length 1)
			if params.Name == "" {
				if params.CompanyName != "" {
					params.Name = params.CompanyName
				} else {
					params.Name = "JISC Contact"
				}
			}

			addr, err := entities.NewAddress(params.City, params.Country)
			if err != nil {
				// Fallback if country code invalid, etc.
				addr = &entities.Address{
					City:        entities.PostalLineType(params.City),
					CountryCode: "GB",
				}
			}
			addr.Street1 = entities.OptPostalLineType(params.Address)
			addr.PostalCode = entities.PCType(params.PCode)

			pi := &entities.ContactPostalInfo{
				Type:    "loc",
				Name:    entities.PostalLineType(params.Name),
				Org:     entities.OptPostalLineType(params.CompanyName),
				Address: addr, // Pass pointer directly
			}

			// Validate postal info locally to warn
			if !pi.IsValid() {
				log.Printf("Warning: Invalid PostalInfo generated for contact %s. Name: '%s', City: '%s'", contactID, params.Name, params.City)
			}

			// Required fields
			email := params.Email
			if email == "" {
				email = "placeholder@example.com"
			}

			if len(rarClID) < 3 || len(rarClID) > 16 {
				log.Printf("Warning: Invalid ClID detected for contact creation: '%s' (Provider: %s)", rarClID, d.Registrar.Name)
			}
			_, errID := entities.NewClIDType(contactID)
			if errID != nil {
				log.Printf("ERROR: Generated Contact ID '%s' is Invalid: %v", contactID, errID)
			}

			// IDEMPOTENCY CHECK
			if existingContacts[contactID] {
				// log.Printf("Skipping creation of contact %s (already exists)", contactID)
			} else {
				contacts[contactID] = commands.CreateContactCommand{
					ID:         contactID,
					ClID:       rarClID,
					CrRr:       rarClID, // Explicitly set Creator
					UpRr:       rarClID, // Explicitly set Updater
					Email:      email,
					AuthInfo:   "Import-2026-Data!",
					PostalInfo: [2]*entities.ContactPostalInfo{pi, nil},
					Status:     entities.ContactStatus{OK: true},
				}
			}
		}

		// 3. Prepare Hosts
		dnsList := []string{d.DNS1, d.DNS2, d.DNS3, d.DNS4}
		var currentDomainHosts []string
		for _, dns := range dnsList {
			if dns == "" {
				continue
			}
			hostName := strings.Split(dns, ",")[0]
			currentDomainHosts = append(currentDomainHosts, hostName)

			if _, exists := hosts[hostName]; !exists {
				// IDEMPOTENCY CHECK: Check if host already exists for this registrar
				// Note: Hosts in JISC might be shared or independent.
				// But CreateHostCommand is specific to a registrar (rarClID).
				// If host exists for THIS registrar, skip creation.
				key := fmt.Sprintf("%s|%s", strings.ToLower(hostName), rarClID)
				if existingHosts[key] {
					// log.Printf("Skipping creation of %s (already exists for %s)", hostName, rarClID)
					continue
				}

				hosts[hostName] = commands.CreateHostCommand{
					Name:   hostName,
					ClID:   entities.ClIDType(rarClID), // Assign to the registrar of the first domain using it
					Status: entities.HostStatus{OK: true},
				}
			}
		}
		domainHosts[d.DomainName] = currentDomainHosts

		// 4. Prepare Domain
		// Parse dates
		// Assumption: Date format is YYYY-MM-DD or similar. JISC example: "2003-08-20"
		// Go time layout: "2006-01-02"
		regDate, _ := time.Parse("2006-01-02", d.RegisteredDate)
		expDate, _ := time.Parse("2006-01-02", d.RegisterExpireDate)

		domainCmds = append(domainCmds, commands.CreateDomainCommand{
			Name:         d.DomainName,
			ClID:         rarClID,
			RegistrantID: contactID,
			AdminID:      contactID,
			TechID:       contactID,
			BillingID:    contactID,
			CreatedAt:    regDate,
			ExpiryDate:   expDate,
			AuthInfo:     "Import-2026-Data!",
			Status:       entities.DomainStatus{OK: true},
		})

		count++
		if count%1000 == 0 {
			fmt.Printf("\rParsed %d domains...", count)
		}
	}
	fmt.Printf("\rParsed %d domains. Starting API Import...\n", count)

	// Execute Bulk Imports
	// Note: Maps are unordered, but API shouldn't care for creation order among same type

	// 1. Registrars
	var rarList []commands.CreateRegistrarCommand
	for _, c := range registrars {
		rarList = append(rarList, c)
	}
	log.Printf("Importing %d Registrars...", len(rarList))
	if err := client.BulkCreateRegistrars(rarList); err != nil {
		return fmt.Errorf("failed to import registrars: %w", err)
	}

	// 2. Contacts
	var contactList []commands.CreateContactCommand
	for _, c := range contacts {
		contactList = append(contactList, c)
	}
	log.Printf("Importing %d Contacts...", len(contactList))
	// Batch contacts to avoid massive payload?
	batchSize := 1000
	for i := 0; i < len(contactList); i += batchSize {
		end := i + batchSize
		if end > len(contactList) {
			end = len(contactList)
		}
		if err := client.BulkCreateContacts(contactList[i:end]); err != nil {
			return fmt.Errorf("failed to import contacts batch %d: %w", i, err)
		}
	}

	// 3. Hosts
	var hostList []commands.CreateHostCommand
	for _, c := range hosts {
		hostList = append(hostList, c)
	}
	log.Printf("Importing %d Hosts...", len(hostList))
	for i := 0; i < len(hostList); i += batchSize {
		end := i + batchSize
		if end > len(hostList) {
			end = len(hostList)
		}
		if err := client.BulkCreateHosts(hostList[i:end]); err != nil {
			return fmt.Errorf("failed to import hosts batch %d: %w", i, err)
		}
	}

	// 4. Domains
	log.Printf("Importing %d Domains...", len(domainCmds))
	for i := 0; i < len(domainCmds); i += batchSize {
		end := i + batchSize
		if end > len(domainCmds) {
			end = len(domainCmds)
		}
		if err := client.BulkCreateDomains(domainCmds[i:end]); err != nil {
			return fmt.Errorf("failed to import domains batch %d: %w", i, err)
		}
	}

	// 5. Link Hosts
	log.Println("Linking Hosts to Domains...")
	linkingCount := 0
	for domain, hostNames := range domainHosts {
		for _, host := range hostNames {
			if err := client.AddHostToDomainByHostName(domain, host); err != nil {
				// Log error but continue?
				log.Printf("Failed to link host %s to domain %s: %v", host, domain, err)
			}
		}
		linkingCount++
		if linkingCount%500 == 0 {
			fmt.Printf("\rLinked hosts for %d domains...", linkingCount)
		}
	}
	fmt.Printf("\rLinked hosts for %d domains. Done.\n", linkingCount)

	return nil
}
