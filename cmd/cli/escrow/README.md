# Escrow CLI Tool

The Escrow CLI tool provides comprehensive functionality for analyzing, importing, and managing Registry Data Escrow (RDE) deposit files in XML format.

## Features

- **XML Analysis**: Parse and analyze RDE escrow deposit files
- **Streaming Analysis**: High-performance single-pass XML processing for large files (8GB+)
- **CSV Export**: Export analyzed data to CSV files for further processing
- **SQLite Database**: Convert CSV files to optimized SQLite database for fast queries
- **Registrar Mapping**: Map registrar IDs to target systems
- **Data Validation**: Comprehensive validation of escrow data integrity

## Commands

### Analyze Command

Analyze an RDE escrow deposit file and export data to CSV files.

```bash
escrow analyze [options] <escrow-file.xml>
```

**Options:**
- `--map-registrars, --map, -m`: Try to map registrar IDs to the target system (default: false)
- `--streaming`: Use optimized single-pass streaming analysis (recommended for large files) (default: false)

**Example:**
```bash
# Basic analysis
escrow analyze co_2025-09-29_full_S1_R0.xml

# Streaming analysis with registrar mapping (recommended for large files)
escrow analyze --streaming --map-registrars co_2025-09-29_full_S1_R0.xml
```

**Generated CSV Files:**
- `{base}-domains.csv` - Domain records
- `{base}-domainStatuses.csv` - Domain status information
- `{base}-domainNameservers.csv` - Domain nameserver relationships
- `{base}-domainTransfers.csv` - Domain transfer data
- `{base}-domainDnssec.csv` - DNSSEC information
- `{base}-domainRgpStatuses.csv` - RGP (Redemption Grace Period) statuses
- `{base}-contacts.csv` - Contact records
- `{base}-contactStatuses.csv` - Contact status information
- `{base}-contactPostalInfo.csv` - Contact postal addresses
- `{base}-hosts.csv` - Host records
- `{base}-hostStatuses.csv` - Host status information
- `{base}-hostAddresses.csv` - Host IP addresses
- `{base}-registrars.csv` - Registrar information
- `{base}-registrarPostalInfo.csv` - Registrar postal addresses
- `{base}-nndn.csv` - NN DN records
- `{base}-uniqueContactID.csv` - Unique contact ID mappings

### CSV-to-SQLite Command

Convert escrow CSV files to an optimized SQLite database for fast queries.

```bash
escrow csv-to-sqlite [options] <base-filename>
```

**Options:**
- `--output, -o`: Output SQLite database file (default: `[base-filename].db`)

**Example:**
```bash
escrow csv-to-sqlite co_2025-09-29_full_S1_R0
```

This will read all CSV files with the base name `co_2025-09-29_full_S1_R0` and create `co_2025-09-29_full_S1_R0.db`.

**Database Schema:**
- `domains` - Core domain data
- `domain_nameservers` - Domain-to-nameserver relationships
- `domain_statuses` - Domain status information
- `domain_rgp_statuses` - RGP status information
- `hosts` - Host records
- `host_addresses` - Host IP addresses
- `host_statuses` - Host status information
- `contacts` - Contact records
- `contact_statuses` - Contact status information
- `contact_postal_info` - Contact postal addresses
- `registrars` - Registrar information
- `registrar_postal_info` - Registrar postal addresses

**Indexes:** Optimized indexes are created for fast lookups on common query patterns.

### Import Command

Import an RDE escrow deposit file into the system.

```bash
escrow import <escrow-file.xml>
```

### Generate Command

Export all relevant data from the database and create an XML escrow deposit file.

```bash
escrow generate [options]
```

### Version Command

Print the version of the escrow tool.

```bash
escrow version
```

## Performance

The tool includes two analysis modes:

1. **Standard Mode**: Loads entire XML into memory (suitable for smaller files)
2. **Streaming Mode**: Single-pass XML processing (recommended for large files >1GB)

**Performance Comparison:**
- **File Size**: 8.8GB XML file
- **Standard Mode**: ~15+ minutes processing time
- **Streaming Mode**: ~3 minutes processing time

## Usage Examples

### Complete Workflow

```bash
# 1. Analyze escrow file with streaming (fast for large files)
escrow analyze --streaming --map-registrars co_2025-09-29_full_S1_R0.xml

# 2. Convert CSV files to SQLite database for fast queries
escrow csv-to-sqlite co_2025-09-29_full_S1_R0

# 3. Query the database (example using sqlite3 CLI)
sqlite3 co_2025-09-29_full_S1_R0.db "SELECT name, registrant FROM domains WHERE name LIKE '%.com' LIMIT 10;"
```

### Large File Processing

For escrow files larger than 1GB, always use streaming mode:

```bash
escrow analyze --streaming co_2025-09-29_full_S1_R0.xml
```

### Database Queries

Once converted to SQLite, you can perform fast queries:

```sql
-- Find domains by registrar
SELECT d.name, r.name as registrar_name
FROM domains d
JOIN registrars r ON d.clid = r.id
WHERE r.name LIKE '%example%';

-- Count domains by status
SELECT status, COUNT(*) as count
FROM domain_statuses
GROUP BY status
ORDER BY count DESC;

-- Find hosts with multiple IP addresses
SELECT h.name, COUNT(ha.ip_address) as ip_count
FROM hosts h
JOIN host_addresses ha ON h.name = ha.host_name
GROUP BY h.name
HAVING ip_count > 1;
```

## Dependencies

- Uses `modernc.org/sqlite` - Pure Go SQLite driver (no CGO required)
- Compatible with standard SQLite features
- Cross-platform support (Linux, macOS, Windows)

## Error Handling

The tool includes comprehensive error handling and validation:
- CSV file integrity checks
- Database transaction safety
- Memory-efficient streaming processing
- Detailed error messages and logging
