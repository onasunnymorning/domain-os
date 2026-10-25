# Walkthrough - Refactored Escrow Import Pipeline

I have refactored the sequential `EscrowImportWorkflow` into a modular, observable 5-stage pipeline as requested.

## Architecture

The workflow now consists of 5 distinct stages, each passing state via S3 Assets.

```mermaid
flowchart TB
    Start((Start)) --> A[ParseAndExtractAssets]
    A -->|AssetKeys| B[BuildStagingDatabase]
    B -->|Ryde.db Key| C[ResolveRegistrars]
    C -->|Ryde.db Key| D[ApplyRegistrarMappings]
    D -->|Staged.db Key| E[Ingest* / LinkDomainHosts / AccreditRegistrars]
    E --> End((Done))
```

## Implementation Details

### 1. ParseAndExtractAssets (Activity)
*   **Input**: XML Object Key
*   **Output**: List of CSV Asset Keys
*   **Action**: Streams XML, generates CSVs for each entity type, uploads to S3. Replaces `StreamingAnalysis`.

### 2. BuildStagingDatabase (Activity)
*   **Input**: Asset Keys
*   **Output**: `ryde.db` Key
*   **Action**: Ingests CSV assets into a monolithic SQLite database (`ryde.db`).

### 3. ResolveRegistrars (Activity)
*   **Input**: `ryde.db` Key, Overrides
*   **Output**: `ryde.db` Key (Updated)
*   **Action**: 
    1. Downloads `ryde.db`.
    2. Queries `registrars` table.
    3. Resolves mappings via API or Overrides.
    4. Persists mappings to `registrar_mapping` table in `ryde.db`.
    5. Uploads updated DB.

### 4. ApplyRegistrarMappings (Activity)
*   **Input**: `ryde.db` Key
*   **Output**: `staged.db` Key
*   **Action**:
    1. Creates a new `staged.db` by copying `ryde.db`.
    2. Performs SQL `UPDATE` operations to swap internal IDs (ClID, CrRr, UpRr) with mapped Registrar IDs using `registrar_mapping`.
    3. Uploads `staged.db`.

### 5. Ingestion (Activities)
*   **Activities**: `IngestContacts`, `IngestHosts`, `IngestDomains`, `IngestNNDNs`, `LinkDomainHosts`, `AccreditRegistrars`.
*   **Input**: `staged.db` Key
*   **Output**: Import Counts
*   **Action**:
    1. Downloads `staged.db`.
    2. Instantiates `DirectDBImporter`.
    3. Streams data from `staged.db` straight into Postgres using `DirectDBImporter`.
    4. Uses "Identity Mapping" since IDs are already swapped in `staged.db`.

Direct database ingestion is the only import path. The earlier variant that created each record through the admin API (`ImportFromSQLite` and the `escrow import` CLI command) has been removed.

## Verification
*   **Build**: `go build` passed for workflows and activities.
*   **Linting**: Fixed unused variables and import issues.

The code is ready for deployment and execution.

## UI Alignment

The frontend has been updated to reflect the new 5-stage pipeline:

1.  **Pipeline Stages**: The "Runs" table now displays a "Stage 1/5" to "Stage 5/5" badge, inferred from the existence of artifacts.
2.  **New Artifact Links**:
    *   **Staged DB**: Link to the intermediate `staged.db` (Stage 4).
    *   **Events**: Link to `import-events.json` (Stage 5 result).
3.  **Backend Support**: `ListImports` exposes the new artifacts. (`import-events.json` was written by the removed admin-API import and now only exists on older runs.)

### Verification
- **Backend Build**: Passed `go build -v ./internal/...`
- **Frontend Check**: Passed `npx tsc --noEmit` on modified files.
- **Manual Check**: User can refresh the Escrow page to see the new columns and links.
