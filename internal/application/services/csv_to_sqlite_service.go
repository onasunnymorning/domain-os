package services

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// CSVToSQLiteService converts escrow CSV files to SQLite database
type CSVToSQLiteService struct {
	baseFilename string
	db           *sql.DB
}

// NewCSVToSQLiteService creates a new CSV to SQLite conversion service
func NewCSVToSQLiteService(baseFilename string) *CSVToSQLiteService {
	return &CSVToSQLiteService{
		baseFilename: baseFilename,
	}
}

// ConvertToSQLite creates an SQLite database from the CSV files
func (svc *CSVToSQLiteService) ConvertToSQLite(dbPath string, heartbeat HeartbeatFunc) error {
	log.Printf("Converting CSV files to SQLite database: %s", dbPath)

	// Open/create SQLite database
	var err error
	svc.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer svc.db.Close()

	// Create database schema
	if err := svc.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Begin transaction for better performance
	tx, err := svc.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Import data from CSV files
	// Contacts first so downstream steps (domains) can reference them
	if err := svc.importContacts(tx, heartbeat); err != nil {
		return fmt.Errorf("failed to import contacts: %w", err)
	}

	if err := svc.importContactPostalInfo(tx); err != nil {
		return fmt.Errorf("failed to import contact postal info: %w", err)
	}

	if err := svc.importContactStatuses(tx); err != nil {
		return fmt.Errorf("failed to import contact statuses: %w", err)
	}

	if err := svc.importDomains(tx, heartbeat); err != nil {
		return fmt.Errorf("failed to import domains: %w", err)
	}

	if err := svc.importDomainNameservers(tx); err != nil {
		return fmt.Errorf("failed to import domain nameservers: %w", err)
	}

	if err := svc.importHosts(tx, heartbeat); err != nil {
		return fmt.Errorf("failed to import hosts: %w", err)
	}

	if err := svc.importHostAddresses(tx); err != nil {
		return fmt.Errorf("failed to import host addresses: %w", err)
	}

	if err := svc.importDomainStatuses(tx); err != nil {
		return fmt.Errorf("failed to import domain statuses: %w", err)
	}

	if err := svc.importDomainRgpStatuses(tx); err != nil {
		return fmt.Errorf("failed to import domain RGP statuses: %w", err)
	}

	if err := svc.importHostStatuses(tx); err != nil {
		return fmt.Errorf("failed to import host statuses: %w", err)
	}

	if err := svc.importRegistrars(tx); err != nil {
		return fmt.Errorf("failed to import registrars: %w", err)
	}

	if err := svc.importRegistrarPostalInfo(tx); err != nil {
		return fmt.Errorf("failed to import registrar postal info: %w", err)
	}

	// Optionally import registrar mapping JSON so downstream import can map clids without extra files
	if err := svc.importRegistrarMapping(tx); err != nil {
		return fmt.Errorf("failed to import registrar mapping: %w", err)
	}

	if err := svc.importNNDNs(tx); err != nil {
		return fmt.Errorf("failed to import NNDNs: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Create indexes for better query performance
	if err := svc.createIndexes(); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	// Enrich registrar object counts from imported data tables.
	// The registrars CSV doesn't include domain/host/contact counts, so we
	// compute them from the actual data. ResolveRegistrars now uses a LEFT JOIN
	// query instead, but this keeps the DB self-consistent for other consumers.
	if err := svc.enrichRegistrarCounts(); err != nil {
		return fmt.Errorf("failed to enrich registrar counts: %w", err)
	}

	log.Printf("✅ Successfully created SQLite database with escrow data")
	return nil
}

// createSchema creates the database tables
func (svc *CSVToSQLiteService) createSchema() error {
	schema := `
	-- Core domain data
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
		"update" TEXT
	);

	-- Domain nameservers (many-to-many relationship)
	CREATE TABLE IF NOT EXISTS domain_nameservers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		nameserver TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Domain statuses
	CREATE TABLE IF NOT EXISTS domain_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Domain RGP statuses
	CREATE TABLE IF NOT EXISTS domain_rgp_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		rgp_status TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Hosts
	CREATE TABLE IF NOT EXISTS hosts (
		name TEXT PRIMARY KEY,
		roid TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		uprr TEXT,
		"update" TEXT
	);

	-- Host IP addresses
	CREATE TABLE IF NOT EXISTS host_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		ip_version TEXT,
		FOREIGN KEY (host_name) REFERENCES hosts(name)
	);

	-- Host statuses
	CREATE TABLE IF NOT EXISTS host_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (host_name) REFERENCES hosts(name)
	);

	-- Registrar data
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
		"update" TEXT,
		domain_count INTEGER DEFAULT 0,
		host_count INTEGER DEFAULT 0,
		contact_count INTEGER DEFAULT 0
	);

	-- Registrar postal information
	CREATE TABLE IF NOT EXISTS registrar_postal_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		registrar_id TEXT NOT NULL,
		type TEXT NOT NULL,
		street1 TEXT,
		street2 TEXT,
		street3 TEXT,
		city TEXT,
		state_province TEXT,
		postal_code TEXT,
		country_code TEXT,
		FOREIGN KEY (registrar_id) REFERENCES registrars(id)
	);

	-- Registrar mapping from escrow registrar IDs to mapped registrar clids in our system
	CREATE TABLE IF NOT EXISTS registrar_mapping (
		escrow_id TEXT PRIMARY KEY,
		name TEXT,
		gurid INTEGER,
		registrar_clid TEXT
	);

	-- Contacts (linked contacts only are exported by the analyzer)
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

	-- Contact postal info (optional INT/LOC rows)
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

	-- Contact statuses
	CREATE TABLE IF NOT EXISTS contact_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contact_id TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (contact_id) REFERENCES contacts(id)
	);

	-- NNDNs
	CREATE TABLE IF NOT EXISTS nndns (
		aname TEXT PRIMARY KEY,
		uname TEXT,
		idntableid TEXT,
		originalname TEXT,
		namestate TEXT,
		crdate TEXT
	);
	`

	_, err := svc.db.Exec(schema)
	return err
}

// createIndexes creates indexes for better query performance
func (svc *CSVToSQLiteService) createIndexes() error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_domain_nameservers_domain ON domain_nameservers(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_domain_nameservers_ns ON domain_nameservers(nameserver)",
		"CREATE INDEX IF NOT EXISTS idx_domain_statuses_domain ON domain_statuses(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_domain_rgp_statuses_domain ON domain_rgp_statuses(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_host_addresses_host ON host_addresses(host_name)",
		"CREATE INDEX IF NOT EXISTS idx_host_addresses_ip ON host_addresses(ip_address)",
		"CREATE INDEX IF NOT EXISTS idx_host_statuses_host ON host_statuses(host_name)",
		"CREATE INDEX IF NOT EXISTS idx_domains_registrant ON domains(registrant)",
		"CREATE INDEX IF NOT EXISTS idx_domains_clid ON domains(clid)",
		"CREATE INDEX IF NOT EXISTS idx_registrar_postal_info_registrar ON registrar_postal_info(registrar_id)",
		"CREATE INDEX IF NOT EXISTS idx_registrars_name ON registrars(name)",
		"CREATE INDEX IF NOT EXISTS idx_registrars_gurid ON registrars(gurid)",
		"CREATE INDEX IF NOT EXISTS idx_registrar_mapping_clid ON registrar_mapping(registrar_clid)",
		// Hosts
		"CREATE INDEX IF NOT EXISTS idx_hosts_clid ON hosts(clid)",
		// Contacts
		"CREATE INDEX IF NOT EXISTS idx_contacts_clid ON contacts(clid)",
		"CREATE INDEX IF NOT EXISTS idx_contact_statuses_contact ON contact_statuses(contact_id)",
		"CREATE INDEX IF NOT EXISTS idx_contact_postal_contact ON contact_postal_info(contact_id)",
	}

	for _, indexSQL := range indexes {
		if _, err := svc.db.Exec(indexSQL); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	log.Printf("✅ Created database indexes for fast lookups")
	return nil
}

// enrichRegistrarCounts populates domain_count, host_count, and contact_count
// on the registrars table by counting actual references in the data tables.
// This ensures the counts reflect reality rather than relying on external
// analysis files or stale metadata.
func (svc *CSVToSQLiteService) enrichRegistrarCounts() error {
	_, err := svc.db.Exec(`
		UPDATE registrars SET
			domain_count = COALESCE((SELECT COUNT(*) FROM domains WHERE clid = registrars.id), 0),
			host_count = COALESCE((SELECT COUNT(*) FROM hosts WHERE clid = registrars.id), 0),
			contact_count = COALESCE((SELECT COUNT(*) FROM contacts WHERE clid = registrars.id), 0)
	`)
	if err != nil {
		return fmt.Errorf("enrichRegistrarCounts: %w", err)
	}
	log.Printf("✅ Enriched registrar counts from imported data")
	return nil
}

// importDomains imports domain data from CSV
func (svc *CSVToSQLiteService) importDomains(tx *sql.Tx, heartbeat HeartbeatFunc) error {
	csvFile := svc.baseFilename + "-domains.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping domains: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare(`
		INSERT INTO domains (name, roid, uname, idntableid, originalname, registrant, clid, crrr, crdate, exdate, uprr, "update")
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if len(record) < 12 {
			continue // Skip malformed records
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1], record[2], record[3], record[4], record[5],
			record[6], record[7], record[8], record[9], record[10], record[11])
		if err != nil {
			log.Printf("Warning: failed to insert domain %s: %v", record[0], err)
			continue
		}
		count++

		if count%5000 == 0 && heartbeat != nil {
			heartbeat("importing domains", count)
		}
	}

	log.Printf("✅ Imported %d domains", count)
	return nil
}

// importDomainNameservers imports domain nameserver relationships
func (svc *CSVToSQLiteService) importDomainNameservers(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-domainNameservers.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping domain nameservers: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO domain_nameservers (domain_name, nameserver) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 2 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), strings.ToLower(record[1]))
		if err != nil {
			log.Printf("Warning: failed to insert domain nameserver %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d domain-nameserver relationships", count)
	return nil
}

// importDomainStatuses imports domain status data
func (svc *CSVToSQLiteService) importDomainStatuses(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-domainStatuses.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping domain statuses: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO domain_statuses (domain_name, status) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 2 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1])
		if err != nil {
			log.Printf("Warning: failed to insert domain status %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d domain statuses", count)
	return nil
}

// importDomainRgpStatuses imports domain RGP status data from CSV
func (svc *CSVToSQLiteService) importDomainRgpStatuses(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-domainRgpStatus.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping domain RGP statuses: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO domain_rgp_statuses (domain_name, rgp_status) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	count := 0
	isFirst := true
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if isFirst {
			isFirst = false
			continue // Skip header
		}
		if len(record) < 2 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1])
		if err != nil {
			log.Printf("Warning: failed to insert domain RGP status %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d domain RGP statuses", count)
	return nil
}

// importHosts imports host data from CSV
func (svc *CSVToSQLiteService) importHosts(tx *sql.Tx, heartbeat HeartbeatFunc) error {
	csvFile := svc.baseFilename + "-hosts.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping hosts: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO hosts (name, roid, clid, crrr, crdate, uprr, \"update\") VALUES (?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 7 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1], record[2], record[3], record[4], record[5], record[6])
		if err != nil {
			log.Printf("Warning: failed to insert host %s: %v", record[0], err)
			continue
		}
		count++

		if count%5000 == 0 && heartbeat != nil {
			heartbeat("importing hosts", count)
		}
	}

	log.Printf("✅ Imported %d hosts", count)
	return nil
}

// importHostAddresses imports host address data
func (svc *CSVToSQLiteService) importHostAddresses(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-hostAddresses.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping host addresses: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO host_addresses (host_name, ip_address, ip_version) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 3 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1], record[2])
		if err != nil {
			log.Printf("Warning: failed to insert host address %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d host addresses", count)
	return nil
}

// importHostStatuses imports host status data
func (svc *CSVToSQLiteService) importHostStatuses(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-hostStatuses.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping host statuses: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO host_statuses (host_name, status) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 2 {
			continue
		}

		_, err = stmt.Exec(strings.ToLower(record[0]), record[1])
		if err != nil {
			log.Printf("Warning: failed to insert host status %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d host statuses", count)
	return nil
}

// Helper methods

// importContacts imports contacts from CSV (linked contacts only)
func (svc *CSVToSQLiteService) importContacts(tx *sql.Tx, heartbeat HeartbeatFunc) error {
	csvFile := svc.baseFilename + "-contacts.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping contacts: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare(`INSERT INTO contacts (id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if len(record) < 10 {
			continue
		}
		if _, err = stmt.Exec(record[0], record[1], record[2], record[3], record[4], record[5], record[6], record[7], record[8], record[9]); err != nil {
			log.Printf("Warning: failed to insert contact %s: %v", record[0], err)
			continue
		}
		count++

		if count%5000 == 0 && heartbeat != nil {
			heartbeat("importing contacts", count)
		}
	}

	log.Printf("✅ Imported %d contacts", count)
	return nil
}

// importContactStatuses imports contact statuses from CSV
func (svc *CSVToSQLiteService) importContactStatuses(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-contactStatuses.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping contact statuses: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare(`INSERT INTO contact_statuses (contact_id, status) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if len(record) < 2 {
			continue
		}
		if _, err = stmt.Exec(record[0], record[1]); err != nil {
			log.Printf("Warning: failed to insert contact status %s -> %s: %v", record[0], record[1], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d contact statuses", count)
	return nil
}

// importContactPostalInfo imports contact postal info from CSV
func (svc *CSVToSQLiteService) importContactPostalInfo(tx *sql.Tx) error {
	csvFile := svc.baseFilename + "-contactPostalInfo.csv"
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping contact postal info: %s not found", csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare(`INSERT INTO contact_postal_info (contact_id, type, name, org, street1, street2, street3, city, state_province, postal_code, country_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	count := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Expect either 11 fields (with contact_id) or skip if not present
		if len(record) < 11 {
			continue
		}
		if _, err = stmt.Exec(record[0], record[1], record[2], record[3], record[4], record[5], record[6], record[7], record[8], record[9], record[10]); err != nil {
			log.Printf("Warning: failed to insert contact postal info for %s: %v", record[0], err)
			continue
		}
		count++
	}

	log.Printf("✅ Imported %d contact postal info records", count)
	return nil
}

func (svc *CSVToSQLiteService) fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func (svc *CSVToSQLiteService) readCSV(filename string) ([][]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	return reader.ReadAll()
}

// importRegistrarMapping imports registrar mapping from JSON if present
// Expected filename: baseFilename + "-registrarMapping.json" (or "-registrar-map.json" as a fallback)
func (svc *CSVToSQLiteService) importRegistrarMapping(tx *sql.Tx) error {
	// Determine candidate file names
	primary := svc.baseFilename + "-registrarMapping.json"
	fallback := svc.baseFilename + "-registrar-map.json"
	analysis := svc.baseFilename + "-analysis.json"

	var filePath string
	if svc.fileExists(primary) {
		filePath = primary
	} else if svc.fileExists(fallback) {
		filePath = fallback
	} else if svc.fileExists(analysis) {
		// Fallback: extract registrarMapping from the analysis JSON envelope
		f, err := os.Open(analysis)
		if err != nil {
			return err
		}
		defer f.Close()

		// Minimal envelope to avoid depending on full struct
		var envelope struct {
			RegistrarMapping entities.RegistrarMapping `json:"registrarMapping"`
		}
		dec := json.NewDecoder(f)
		if err := dec.Decode(&envelope); err != nil {
			return fmt.Errorf("failed to parse analysis json for registrar mapping: %w", err)
		}

		if len(envelope.RegistrarMapping) == 0 {
			// Nothing to import
			return nil
		}

		// Prepare insert
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, name, gurid, registrar_clid) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		inserted := 0
		for escrowID, info := range envelope.RegistrarMapping {
			gurid := int(info.GurID)
			mapped := string(info.RegistrarClID)
			if _, err = stmt.Exec(escrowID, info.Name, gurid, mapped); err != nil {
				log.Printf("Warning: failed to insert registrar mapping %s -> %s: %v", escrowID, mapped, err)
				continue
			}
			inserted++
		}

		log.Printf("✅ Imported %d registrar mapping rows (from analysis.json)", inserted)
		return nil
	} else {
		// Not present; it's optional
		return nil
	}

	// Open and decode JSON
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var mapping entities.RegistrarMapping
	if err := dec.Decode(&mapping); err != nil {
		return fmt.Errorf("failed to parse registrar mapping json (%s): %w", filepath.Base(filePath), err)
	}

	// Prepare insert
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, name, gurid, registrar_clid) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	inserted := 0
	for escrowID, info := range mapping {
		gurid := int(info.GurID)
		mapped := string(info.RegistrarClID)
		if _, err = stmt.Exec(escrowID, info.Name, gurid, mapped); err != nil {
			log.Printf("Warning: failed to insert registrar mapping %s -> %s: %v", escrowID, mapped, err)
			continue
		}
		inserted++
	}

	log.Printf("✅ Imported %d registrar mapping rows", inserted)
	return nil
}

// Query helper methods for easy lookups

type DomainInfo struct {
	Name        string
	Registrant  string
	ClID        string
	CreatedDate string
	ExpiryDate  string
	Nameservers []string
	Statuses    []string
	RgpStatuses []string
}

type HostInfo struct {
	Name        string
	ClID        string
	CreatedDate string
	IPAddresses []string
	Statuses    []string
}

// GetDomainInfo retrieves complete domain information including nameservers and statuses
func (svc *CSVToSQLiteService) GetDomainInfo(domainName string) (*DomainInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic domain info
	var info DomainInfo
	err := svc.db.QueryRow(`
		SELECT name, registrant, clid, crdate, exdate
		FROM domains WHERE name = ?
	`, domainName).Scan(&info.Name, &info.Registrant, &info.ClID, &info.CreatedDate, &info.ExpiryDate)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Domain not found
		}
		return nil, err
	}

	// Get nameservers
	rows, err := svc.db.Query("SELECT nameserver FROM domain_nameservers WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			continue
		}
		info.Nameservers = append(info.Nameservers, ns)
	}

	// Get statuses
	rows, err = svc.db.Query("SELECT status FROM domain_statuses WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			continue
		}
		info.Statuses = append(info.Statuses, status)
	}

	// Get RGP statuses
	rows, err = svc.db.Query("SELECT rgp_status FROM domain_rgp_statuses WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rgpStatus string
		if err := rows.Scan(&rgpStatus); err != nil {
			continue
		}
		info.RgpStatuses = append(info.RgpStatuses, rgpStatus)
	}

	return &info, nil
}

// GetHostInfo retrieves complete host information including IP addresses and statuses
func (svc *CSVToSQLiteService) GetHostInfo(hostName string) (*HostInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic host info
	var info HostInfo
	err := svc.db.QueryRow(`
		SELECT name, clid, crdate
		FROM hosts WHERE name = ?
	`, hostName).Scan(&info.Name, &info.ClID, &info.CreatedDate)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Host not found
		}
		return nil, err
	}

	// Get IP addresses
	rows, err := svc.db.Query("SELECT ip_address FROM host_addresses WHERE host_name = ?", hostName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		info.IPAddresses = append(info.IPAddresses, ip)
	}

	// Get statuses
	rows, err = svc.db.Query("SELECT status FROM host_statuses WHERE host_name = ?", hostName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			continue
		}
		info.Statuses = append(info.Statuses, status)
	}

	return &info, nil
}

// FindDomainsByNameserver finds all domains using a specific nameserver
func (svc *CSVToSQLiteService) FindDomainsByNameserver(nameserver string) ([]string, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := svc.db.Query("SELECT domain_name FROM domain_nameservers WHERE nameserver = ?", nameserver)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			continue
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// RegistrarInfo holds basic registrar information
type RegistrarInfo struct {
	ID         string
	Name       string
	GurID      int
	Status     string
	Voice      string
	Fax        string
	Email      string
	URL        string
	CrDate     string
	Update     string
	PostalInfo []RegistrarPostalInfo
}

// RegistrarPostalInfo holds registrar postal address information
type RegistrarPostalInfo struct {
	Type          string
	Street1       string
	Street2       string
	Street3       string
	City          string
	StateProvince string
	PostalCode    string
	CountryCode   string
}

// GetRegistrarInfo retrieves complete registrar information including postal info
func (svc *CSVToSQLiteService) GetRegistrarInfo(registrarID string) (*RegistrarInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic registrar info
	var info RegistrarInfo
	err := svc.db.QueryRow(`
		SELECT id, name, gurid, status, voice, fax, email, url, crdate, "update"
		FROM registrars WHERE id = ?
	`, registrarID).Scan(&info.ID, &info.Name, &info.GurID, &info.Status, &info.Voice, &info.Fax, &info.Email, &info.URL, &info.CrDate, &info.Update)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Registrar not found
		}
		return nil, err
	}

	// Get postal info
	rows, err := svc.db.Query("SELECT type, street1, street2, street3, city, state_province, postal_code, country_code FROM registrar_postal_info WHERE registrar_id = ?", registrarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var postal RegistrarPostalInfo
		if err := rows.Scan(&postal.Type, &postal.Street1, &postal.Street2, &postal.Street3, &postal.City, &postal.StateProvince, &postal.PostalCode, &postal.CountryCode); err != nil {
			continue
		}
		info.PostalInfo = append(info.PostalInfo, postal)
	}

	return &info, nil
}

// FindRegistrarsByName finds registrars matching a name pattern
func (svc *CSVToSQLiteService) FindRegistrarsByName(namePattern string) ([]RegistrarInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := svc.db.Query("SELECT id, name, gurid, status, email FROM registrars WHERE name LIKE ?", "%"+namePattern+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrars []RegistrarInfo
	for rows.Next() {
		var reg RegistrarInfo
		if err := rows.Scan(&reg.ID, &reg.Name, &reg.GurID, &reg.Status, &reg.Email); err != nil {
			continue
		}
		registrars = append(registrars, reg)
	}

	return registrars, nil
}

// OpenDatabase opens an existing SQLite database for querying
func OpenEscrowDatabase(dbPath string) (*CSVToSQLiteService, error) {
	svc := &CSVToSQLiteService{}
	var err error
	svc.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return svc, nil
}

// GetDatabaseStats returns basic statistics about the database
func (svc *CSVToSQLiteService) GetDatabaseStats() (map[string]int, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := make(map[string]int)
	tables := []string{"domains", "hosts", "domain_nameservers", "host_addresses", "domain_statuses", "domain_rgp_statuses", "host_statuses", "registrars", "registrar_postal_info"}

	for _, table := range tables {
		var count int
		err := svc.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			return nil, err
		}
		stats[table] = count
	}

	return stats, nil
}

// importRegistrars imports registrar data from CSV
func (svc *CSVToSQLiteService) importRegistrars(tx *sql.Tx) error {
	fileName := svc.baseFilename + "-registrars.csv"
	if !svc.fileExists(fileName) {
		log.Printf("⚠️  Registrars file not found: %s", fileName)
		return nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO registrars (id, name, gurid, status, voice, fax, email, url, crdate, \"update\") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	records, err := reader.ReadAll() // Still using ReadAll for registrars as they are small
	if err != nil {
		return err
	}

	for _, record := range records {
		if len(record) >= 10 {
			gurid := 0
			if record[2] != "" {
				if val, err := strconv.Atoi(record[2]); err == nil {
					gurid = val
				}
			}
			_, err := stmt.Exec(record[0], record[1], gurid, record[3], record[4], record[5], record[6], record[7], record[8], record[9])
			if err != nil {
				log.Printf("Error inserting registrar %s: %v", record[0], err)
			}
		}
	}

	log.Printf("✅ Imported %d registrars", len(records))
	return nil
}

// importRegistrarPostalInfo imports registrar postal info data from CSV
func (svc *CSVToSQLiteService) importRegistrarPostalInfo(tx *sql.Tx) error {
	fileName := svc.baseFilename + "-registrarPostalInfo.csv"
	if !svc.fileExists(fileName) {
		log.Printf("⚠️  Registrar postal info file not found: %s", fileName)
		return nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO registrar_postal_info (registrar_id, type, street1, street2, street3, city, state_province, postal_code, country_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	records, err := reader.ReadAll() // Small file
	if err != nil {
		return err
	}

	for _, record := range records {
		if len(record) >= 9 {
			_, err := stmt.Exec(record[0], record[1], record[2], record[3], record[4], record[5], record[6], record[7], record[8])
			if err != nil {
				log.Printf("Error inserting registrar postal info for %s: %v", record[0], err)
			}
		}
	}

	log.Printf("✅ Imported %d registrar postal info records", len(records))
	return nil
}

// importNNDNs imports NNDN data from CSV
func (svc *CSVToSQLiteService) importNNDNs(tx *sql.Tx) error {
	fileName := svc.baseFilename + "-nndns.csv"
	if !svc.fileExists(fileName) {
		log.Printf("⚠️  NNDNs file not found: %s", fileName)
		return nil
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	stmt, err := tx.Prepare("INSERT INTO nndns (aname, uname, idntableid, originalname, namestate, crdate) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	records, err := reader.ReadAll()
	if err != nil {
		return err
	}

	for _, record := range records {
		if len(record) >= 6 {
			_, err := stmt.Exec(record[0], record[1], record[2], record[3], record[4], record[5])
			if err != nil {
				log.Printf("Error inserting NNDN %s: %v", record[0], err)
			}
		}
	}

	log.Printf("✅ Imported %d NNDNs", len(records))
	return nil
}

// Close closes the database connection
func (svc *CSVToSQLiteService) Close() error {
	if svc.db != nil {
		return svc.db.Close()
	}
	return nil
}
