---
name: data-pipeline-qa
description: Standard QA report format and validation pattern for data pipeline staged data. Use when implementing quality gates in any data pipeline that stages data before writing to a live system.
---

# Data Pipeline QA Standard

## Purpose

All data pipelines that stage data (SQLite, CSV, temp tables) before writing to a production database **MUST** include a QA validation step that produces a structured JSON report (`qa-report.json`). This report serves as the quality gate: the pipeline reads the `passed` field and halts if it is `false`.

This is a project-wide standard — not specific to any single pipeline. Whether you are importing escrow deposits, syncing zone files, or bulk-loading contact data, the same report schema and rules apply.

## QA Report Schema (v1.0)

### Top-level fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | `string` | Always `"1.0"`. Enables forward-compatible schema evolution. |
| `timestamp` | `string` (ISO 8601) | When the QA step ran. |
| `pipeline` | `string` | Pipeline identifier, e.g. `"escrow-staging"`, `"zone-import"`. |
| `context` | `object` | Pipeline-specific metadata. E.g. `{"tld": "example", "workflowId": "abc-123"}`. |
| `sourceKey` | `string` | Identifier of the data source being validated (e.g. S3 key, file path). |
| `passed` | `bool` | **The gate signal.** `false` if ANY check with `severity: "error"` failed. |
| `summary` | `object` | Entity counts for at-a-glance verification. Keys are entity names, values are counts. |
| `checks` | `array` | Array of Check objects (see below). |

### Check object

| Field | Type | Description |
|-------|------|-------------|
| `rule` | `string` | Stable, machine-readable ID in `snake_case`. Used for alerting and filtering. Must never be renamed once published. |
| `description` | `string` | Human-readable explanation of what this check validates. |
| `severity` | `string` | One of `"error"`, `"warning"`, `"info"`. |
| `passed` | `bool` | Did this specific check pass? |
| `affectedCount` | `int` | Number of affected records. Always present; `0` when passed. |
| `message` | `string` | One-line human summary of the result. |
| `detail` | `object` (optional) | Structured data for the check (e.g. count comparisons, thresholds). |
| `sampledItems` | `array` (optional) | Up to **50** example failing records for debugging. MUST be capped. |

## Rules

1. **Every data pipeline with a staging step MUST produce a `qa-report.json`** before writing to the live system.
2. **The `passed` field is the gate signal** — pipelines MUST check this field and halt on `false`.
3. **Severity levels are meaningful:**
   - `error` — blocks the pipeline; data integrity is at risk.
   - `warning` — surfaces for operator review; does not block.
   - `info` — observational; useful for dashboarding and trend analysis.
4. **Rule IDs must be stable** — once a rule ID is published, it must not be renamed. New checks get new IDs.
5. **`sampledItems` MUST be capped at 50** — the report is for triage, not replay. Full data lives in the staged source.
6. **QA checks should be pure reads** — no side effects, no writes. The QA step must be safe to re-run at any time.
7. **QA should run locally** — operate on the staged data (SQLite, files) without network dependencies where possible.
8. **Reports are uploaded as pipeline artifacts** — alongside the staged data in the run's artifact directory.

## Minimum Checks for Any Pipeline

Every pipeline MUST implement at least:

1. **Entity count validation** (`entity_count_mismatch`) — staged counts match the source/parse phase.
2. **Referential integrity** (`referential_integrity`) — foreign key references resolve within the staged data.
3. **Required field completeness** (`required_field_null`) — no NULLs in columns marked as required.
4. **Identity mapping validation** (`unmapped_identity`) — if IDs are remapped during staging, verify all mappings resolved.

Pipelines may add domain-specific checks beyond these four.

## Go Implementation Pattern

### Structs

```go
type QAReport struct {
    Version   string            `json:"version"`
    Timestamp time.Time         `json:"timestamp"`
    Pipeline  string            `json:"pipeline"`
    Context   map[string]string `json:"context"`
    SourceKey string            `json:"sourceKey"`
    Passed    bool              `json:"passed"`
    Summary   map[string]int64  `json:"summary"`
    Checks    []QACheck         `json:"checks"`
}

type QACheck struct {
    Rule          string      `json:"rule"`
    Description   string      `json:"description"`
    Severity      string      `json:"severity"` // "error", "warning", "info"
    Passed        bool        `json:"passed"`
    AffectedCount int         `json:"affectedCount"`
    Message       string      `json:"message"`
    Detail        interface{} `json:"detail,omitempty"`
    SampledItems  interface{} `json:"sampledItems,omitempty"`
}
```

### Helper

```go
// NewQAReport creates a report with Passed defaulting to true.
// It flips to false when an error-severity check fails.
func NewQAReport(pipeline, sourceKey string, ctx map[string]string) *QAReport {
    return &QAReport{
        Version:   "1.0",
        Timestamp: time.Now().UTC(),
        Pipeline:  pipeline,
        Context:   ctx,
        SourceKey: sourceKey,
        Passed:    true,
        Summary:   make(map[string]int64),
    }
}

// AddCheck appends a check and flips the gate on error-severity failure.
func (r *QAReport) AddCheck(check QACheck) {
    r.Checks = append(r.Checks, check)
    if !check.Passed && check.Severity == "error" {
        r.Passed = false
    }
}
```

Initialize with `NewQAReport`, call `AddCheck` for each validation, then serialize and inspect `Passed` before proceeding.

## Example: Full Report

```json
{
  "version": "1.0",
  "timestamp": "2026-06-23T12:00:00Z",
  "pipeline": "escrow-staging",
  "context": {
    "tld": "example",
    "workflowId": "wf-abc-123"
  },
  "sourceKey": "s3://escrow-inbox/example/2026-06-23/deposit.csv",
  "passed": true,
  "summary": {
    "domains": 14523,
    "contacts": 8710,
    "hosts": 2041
  },
  "checks": [
    {
      "rule": "entity_count_mismatch",
      "description": "Staged domain count matches the parsed source count",
      "severity": "error",
      "passed": true,
      "affectedCount": 0,
      "message": "Domain count matches: 14523 staged, 14523 parsed"
    },
    {
      "rule": "referential_integrity",
      "description": "All domain registrant IDs resolve to a staged contact",
      "severity": "error",
      "passed": true,
      "affectedCount": 0,
      "message": "All 14523 registrant references resolve"
    },
    {
      "rule": "required_field_null",
      "description": "No NULLs in required domain columns (name, registrantId, crDate)",
      "severity": "error",
      "passed": true,
      "affectedCount": 0,
      "message": "All required fields populated across 14523 domains"
    },
    {
      "rule": "expiry_date_in_past",
      "description": "Domains with expiry dates before today",
      "severity": "warning",
      "passed": false,
      "affectedCount": 37,
      "message": "37 domains have expiry dates in the past",
      "detail": {
        "threshold": "2026-06-23",
        "oldestExpiry": "2024-11-02"
      },
      "sampledItems": [
        {"domain": "stale1.example", "exDate": "2024-11-02"},
        {"domain": "stale2.example", "exDate": "2025-01-15"}
      ]
    },
    {
      "rule": "unmapped_identity",
      "description": "All contact IDs from source mapped to internal IDs",
      "severity": "error",
      "passed": true,
      "affectedCount": 0,
      "message": "All 8710 contact ID mappings resolved"
    }
  ]
}
```

Note that `passed` is `true` at the top level because the only failing check (`expiry_date_in_past`) has severity `"warning"`, which does not block the pipeline.

## References

This pattern draws inspiration from:

- **[Great Expectations](https://greatexpectations.io/)** — expectation suites and validation results
- **[dbt](https://www.getdbt.com/)** — test result contracts and severity levels
- **[SARIF](https://sarifweb.azurewebsites.net/)** — Static Analysis Results Interchange Format (structured findings with rule IDs and severity)
