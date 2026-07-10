export const BUCKET_STORAGE_DOC_MARKDOWN = `# Bucket Storage Strategy & Recommendations

This document outlines the architectural strategy for S3-compatible object storage (MinIO / Cloudflare R2 / AWS S3) in **Domain OS**. It addresses the tradeoffs between consolidated (single bucket) and segmented (multi-bucket) storage, environment variable naming conventions, and criteria for splitting buckets.

---

## 1. Core Architecture Decision: Segmented Storage (Split Buckets)

Domain OS utilizes S3-compatible object storage for diverse operational datasets. We recommend and implement **segmented storage** (distinct buckets for different concerns) rather than a single bucket using directory paths. 

While a consolidated bucket is slightly simpler to provision, the datasets managed by the registry have fundamentally incompatible security, access control, regulatory, and lifecycle profiles.

### Data Classification

| Data Type | Description | Privacy Level | Lifecycle Profile |
|:---|:---|:---|:---|
| **Registry Escrow (RDE/BRDA)** | Regulatory XML snapshots containing raw domain, contact, registrar, and host objects. | **PII (High)** | Expire after regulatory compliance period (e.g., 365 days) |
| **Event Archives** | Gzip-compressed JSONL files containing historical logs of all domain lifecycle mutations and events. | **Medium** | Keep indefinitely for auditing, telemetry, and forensic analysis |
| **Compliance Reports** | CSV sweeps of TLD label reports for ICANN compliance audits (e.g., Spec 5 sweeps). | **Low** | Retain for compliance window (e.g., 3 years) |

---

## 2. Environment Variable Naming Conventions

To keep configurations clean and cloud-agnostic, environment variables corresponding to storage buckets are prefixed with \`STORAGE_\` instead of cloud-provider-specific prefixes like \`S3_\`.

| Bucket Target | Environment Variable | Service Scope | Description |
|:---|:---|:---|:---|
| Registry Escrow Deposits | \`STORAGE_ESCROW_BUCKET\` | API, Worker | Stores sensitive XML escrow packages |
| Event Stream Archives | \`STORAGE_EVENT_LOGS_BUCKET\` | API, Worker | Stores compressed event stream logs |
| Compliance Sweeps / Reports | \`STORAGE_REPORTS_BUCKET\` | API | Stores CSV sweeps and report metadata |

---

## 3. Decision Matrix: Consolidated vs. Segmented

| Dimension | Consolidated (Folders in 1 Bucket) | Segmented (Multiple Buckets) | Winner |
|:---|:---|:---|:---|
| **IAM Access Control** | Complex prefix-based path policies (highly prone to configuration leaks). | Clean resource-based policies applied directly at the bucket level. | **Segmented** |
| **Lifecycle Policies** | Complex path-prefix rules in lifecycle configuration. | Simple whole-bucket rules (e.g., expire in 365 days). | **Segmented** |
| **Third-Party Access** | Challenging to delegate read-only directory paths securely. | Safe delegation of access for auditing/regulatory bodies. | **Segmented** |
| **Storage Tiering** | Must configure transition rules by prefix patterns. | Can set entire bucket to transition to cold storage classes. | **Segmented** |
| **Initial Provisioning** | Minimally faster (1 Infrastructure resource). | Standard (3-4 Infrastructure resources). | **Consolidated** |

---

## 4. Criteria for Splitting Buckets

When evaluating whether a new data type or feature should be placed in an existing bucket or provisioned with a new one, apply the following four criteria:

### A. The PII & Security Boundary
* **Rule**: If the data contains Personally Identifiable Information (PII) — such as names, physical addresses, emails, or phone numbers — it **must** be isolated from non-sensitive data.
* **Reasoning**: This keeps compliance audits (GDPR, SOC2, ICANN) as narrow as possible. Only the specific worker process requiring access needs permissions to the PII bucket.

### B. Third-Party & Public Access Boundary
* **Rule**: If a third party (e.g., an ICANN escrow agent pulling via SFTP, or public users downloading assets) needs direct read/write access to the storage, that data must live in its own bucket.
* **Reasoning**: Never expose a bucket containing internal operational data to external actors, even with folder path restrictions.

### C. Lifecycle and Retention Boundary
* **Rule**: Split data if it has different retention requirements.
* **Reasoning**: Putting short-lived regulatory files (like escrow deposits) and long-lived audit data (like event logs) in the same bucket makes it easy to accidentally purge critical logs or keep expensive transient files indefinitely.

### D. Storage Class & Cost Optimization
* **Rule**: Split high-volume, rarely-accessed data (write-once, read-never archives) from low-volume, frequently-accessed metadata.
* **Reasoning**: Event logs accumulate fast. Isolating them allows you to transition the entire bucket to cold storage (e.g., AWS Glacier Deep Archive or Cloudflare R2 Infrequent Access) without affecting operational tools.
`;
