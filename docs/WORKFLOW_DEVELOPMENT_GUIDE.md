# Temporal Workflow Development Guide — Target State

> **Audience**: Developers (human and agentic) working on domain-os Temporal workflows.
> **Scope**: Covers the full workflow lifecycle — creation through retirement — plus code organisation, security, testing, and operational conventions.

---

## 1. Current State Snapshot

### Inventory

| Queue | Env Var | Workflows | Purpose |
|---|---|---|---|
| `object-lifecycle` | `TEMPORAL_LIFECYCLE_QUEUE` | ExpiryLoop, PurgeLoop, RestoreWorkflow, SyncRegistrarsWorkflow | Domain lifecycle operations |
| `data-pipeline` | `TEMPORAL_DATA_QUEUE` | EscrowStagingWorkflow, EscrowIngestionWorkflow, TLDCleanupWorkflow, UpdateFX | Data import, escrow, and sync operations |

### Directory Layout

```
internal/
├── application/
│   ├── workflows/          # 8 workflow definitions
│   │   ├── expiryLoop.go
│   │   ├── purgeLoop.go
│   │   ├── restoreWorkflow.go
│   │   ├── syncRegistrarsWorkflow.go
│   │   ├── escrowImport.go         # EscrowStaging + EscrowIngestion
│   │   ├── tldCleanupWorkflow.go
│   │   ├── tldCleanupWorkflow_test.go
│   │   ├── updateFX.go
│   │   └── helpers.go
│   ├── activities/         # ~65 activity files
│   └── schedules/          # 5 schedule definitions
├── infrastructure/
│   └── temporal/
│       └── temporal.go     # Client factory (API key / mTLS / plain)
└── interface/
    └── rest/
        ├── workflow_controller.go   # REST triggers for workflows
        └── escrow_controller.go     # REST triggers for escrow
```

### What's Solid (Don't Change)

- **Signal-based human approval** (TLDCleanupWorkflow) — safe, timeout-protected
- **Child workflow pattern** (EscrowStaging → EscrowIngestion with `PARENT_CLOSE_POLICY_ABANDON`)
- **3-tier Temporal auth** (API key → mTLS → plain dial) in `temporal.go`
- **Auth0 JWT middleware** on all workflow-triggering REST endpoints
- **Separate task queues** for lifecycle, escrow, and sync — enables independent scaling
- **Doppler for secrets** — no `.env` files, no hardcoded credentials

---

## 2. Workflow Lifecycle

Every workflow in this repository should be managed through these lifecycle phases:

```
Create → Test → Deploy → Monitor → Deprecate → Retire → Archive
```

### 2.1 Create

#### File Naming Convention

| Type | Pattern | Example |
|---|---|---|
| Workflow | `{descriptiveName}Workflow.go` | `tldCleanupWorkflow.go` |
| Workflow test | `{descriptiveName}Workflow_test.go` | `tldCleanupWorkflow_test.go` |
| Activity | `{verbNoun}.go` | `backupTLDAssets.go` |
| Activity test | `{verbNoun}_test.go` | `backupTLDAssets_test.go` |
| Schedule | `{workflowName}Schedule.go` | `expirySchedule.go` |

#### Workflow Function Signature

Workflows are **plain Go functions** (not struct methods). Follow this pattern:

```go
// {WorkflowName} orchestrates {brief description}.
// Task queue: {queue-name}
// Trigger: {REST / Schedule / Child workflow / CLI}
func MyNewWorkflow(ctx workflow.Context, params MyWorkflowParams) (MyWorkflowResult, error) {
    // ...
}
```

> **IMPORTANT:** Use a **single struct** for inputs and a **single struct** for outputs. This makes the contract stable — you can add fields without breaking backward compatibility.

```go
// DO — stable contract
type EscrowStagingParams struct {
    TLD            string
    S3Key          string
    AutoIngest     bool  // added later without breaking existing histories
}

// DON'T — positional args are fragile
func EscrowStagingWorkflow(ctx workflow.Context, tld string, s3Key string) error { ... }
```

#### Workflow ID Convention

Use a consistent pattern across all workflows:

```
{domain}-{operation}-{qualifier}-{timestamp}
```

| Domain | Examples |
|---|---|
| `lifecycle` | `lifecycle-expiry-20260623T150000` |
| `lifecycle` | `lifecycle-sync-registrars-20260623T150000` |
| `escrow` | `escrow-staging-example.com-20260623T150000` |
| `escrow` | `escrow-ingest-example.com-20260623T150000` |
| `tld` | `tld-cleanup-example-20260623T150000` |
| `sync` | `sync-fx-20260623T150000` |

For scheduled workflows, Temporal manages the ID. For REST/CLI triggers, generate the ID at the call site:

```go
wfID := fmt.Sprintf("escrow-staging-%s-%s", tld, time.Now().Format("20060102T150405"))
```

#### Activity Organisation Rules

1. **One exported function per file** — keep activities small and focused.
2. **Activities do all I/O** — API calls, DB queries, file operations.
3. **Workflows do orchestration only** — no direct I/O, no non-deterministic operations.
4. **Activity retry policy** — set explicitly, don't rely on defaults:

```go
activityOpts := workflow.ActivityOptions{
    StartToCloseTimeout: 5 * time.Minute,
    RetryPolicy: &temporal.RetryPolicy{
        InitialInterval:    time.Second,
        BackoffCoefficient: 2.0,
        MaximumInterval:    time.Minute,
        MaximumAttempts:    3,
    },
}
ctx = workflow.WithActivityOptions(ctx, activityOpts)
```

### 2.2 Test

#### Required Test Coverage

Every workflow **must** have:

| Test Type | What It Covers | Where |
|---|---|---|
| **Workflow unit test** | Logic, branching, error paths | `workflows/{name}_test.go` |
| **Replay test** | Determinism validation | `workflows/{name}_replay_test.go` |
| **Activity unit tests** | Business logic per activity | `activities/{name}_test.go` |

#### Workflow Unit Tests

Use Temporal's `testsuite.WorkflowTestSuite` with `TestWorkflowEnvironment`:

```go
func (s *MyWorkflowSuite) TestHappyPath() {
    env := s.NewWorkflowEnvironment()
    env.RegisterWorkflow(MyWorkflow)
    env.RegisterActivity(myActivity)

    env.OnActivity(myActivity, mock.Anything, mock.Anything).Return("result", nil)

    env.ExecuteWorkflow(MyWorkflow, MyParams{Field: "value"})

    s.True(env.IsWorkflowCompleted())
    s.NoError(env.GetWorkflowError())
}
```

For **signal-based workflows** (like TLDCleanupWorkflow), test both the approval and rejection paths:

```go
// Simulate delayed signal approval
env.RegisterDelayedCallback(func() {
    env.SignalWorkflow("signal-channel-name", true)
}, time.Millisecond*100)
```

> **TIP:** Set `TEMPORAL_DEBUG=true` during development to prevent false-positive deadlock panics in tests.

#### Replay Tests

Replay tests are the **single most valuable safety net** for workflow code changes. They verify that new code can correctly replay existing workflow histories without non-determinism errors.

```go
func TestReplayMyWorkflow(t *testing.T) {
    replayer := worker.NewWorkflowReplayer()
    replayer.RegisterWorkflow(MyWorkflow)
    err := replayer.ReplayWorkflowHistoryFromJSONFile(nil, "testdata/my_workflow_history.json")
    require.NoError(t, err)
}
```

**How to capture histories for replay tests:**

```bash
# Via Temporal CLI
temporal workflow show \
  --workflow-id "escrow-staging-example.com-20260623T150000" \
  --namespace default \
  --output json > testdata/escrow_staging_history.json
```

**Rules:**
- Store replay history files in `workflows/testdata/`.
- Capture at least one representative history per workflow type.
- Update histories when workflow logic changes intentionally.
- Run replay tests in CI on every PR.

#### Static Analysis

Use the `workflowcheck` tool to detect determinism violations at compile time:

```bash
go install go.temporal.io/sdk/contrib/tools/workflowcheck@latest
workflowcheck ./internal/application/workflows/...
```

### 2.3 Version

> **NOTE:** domain-os currently has **zero** uses of `workflow.GetVersion()` or Worker Versioning. This section describes the target state.

#### When Versioning is Required

You **must** version workflow code when:
- Changing the sequence of activity calls
- Adding, removing, or reordering activities
- Changing the arguments passed to an activity
- Changing timer durations
- Adding or removing child workflow calls

You **do not** need to version when:
- Changing activity *implementation* (activity code is not part of replay determinism)
- Changing activity retry policies (applied at dispatch, not replay)
- Adding logging or metrics inside workflows (as long as they use `workflow.GetLogger()`)

#### Patching Strategy (Recommended for Now)

Use `workflow.GetVersion()` to branch between old and new logic:

```go
v := workflow.GetVersion(ctx, "add-validation-step", workflow.DefaultVersion, 1)
if v == 1 {
    // New path: validate before proceeding
    err = workflow.ExecuteActivity(ctx, ValidateEscrowSource, params).Get(ctx, &validationResult)
    if err != nil {
        return err
    }
}
// Original path continues here
err = workflow.ExecuteActivity(ctx, ProcessData, params).Get(ctx, &result)
```

**Patch cleanup lifecycle:**

| Phase | State | Action |
|---|---|---|
| Initial | No patch | Original code |
| Patched | `GetVersion` branch | Both old + new code paths |
| Deprecated | All old workflows completed | Remove the `DefaultVersion` branch |
| Clean | Retention period expired | Remove `GetVersion` call entirely |

> **WARNING:** Only remove old code paths after confirming that all workflows using them have completed AND the namespace retention period has expired. Use `temporal workflow list --query 'WorkflowType = "X" AND ExecutionStatus = "Running"'` to check.

#### Worker Versioning (Consider Later)

When deployment frequency increases or long-running entity workflows are added, adopt **Worker Versioning with Build IDs**:

```go
w := worker.New(c, taskQueue, worker.Options{
    BuildID:                 "v1.2.3-abc123",
    UseBuildIDForVersioning: true,
})
```

This routes workflow tasks to the correct worker version automatically. Enables true blue-green deployments.

### 2.4 Deploy

#### Current Process (Sufficient for Now)

1. PR with workflow changes
2. CI: lint + test + replay tests + `workflowcheck`
3. Merge → tagged release
4. Docker image → deploy

#### Target State (When Scale Requires It)

- Tag worker Docker images with Build ID
- Use Temporal Worker Versioning to route tasks to correct version
- Progressive rollout: 10% → 50% → 100% with pause durations
- Instant rollback by re-routing traffic to previous Build ID

### 2.5 Monitor

#### Current State

- Temporal UI (port 8081) for workflow visibility
- `"get-state"` query handler on TLDCleanupWorkflow

#### Target State

Every workflow that runs longer than a few seconds **should** expose a query handler:

```go
var state MyWorkflowState

// Register query handler early in the workflow
err := workflow.SetQueryHandler(ctx, "get-state", func() (MyWorkflowState, error) {
    return state, nil
})
if err != nil {
    return err
}

// Update state as workflow progresses
state.Phase = "validating"
state.ItemsProcessed = 42
```

**Metrics and tracing** (when ready):
- Add OpenTelemetry interceptor to the Temporal client for distributed tracing
- Export Temporal SDK metrics to Prometheus via the metrics handler
- Build Grafana dashboards for task queue backlogs and activity latencies

### 2.6 Deprecate

When a workflow should no longer be used:

1. **Stop new executions** — remove/comment the REST endpoint or schedule, add a deprecation notice to the controller:

```go
// Deprecated: Use NewImprovedWorkflow instead. Will be removed in v2.x.
func (c *WorkflowController) StartOldWorkflow(ctx *gin.Context) {
    ctx.JSON(http.StatusGone, gin.H{
        "error": "This workflow is deprecated. Use POST /workflows/new-improved instead.",
    })
}
```

2. **Drain running instances** — monitor with:

```bash
temporal workflow list \
  --query 'WorkflowType = "OldWorkflow" AND ExecutionStatus = "Running"' \
  --namespace <namespace>
```

3. **Keep worker registration** until all instances complete. Never remove a workflow from the worker while instances are still running — they'll hang forever.

4. **Remove code** only after all instances have completed AND the namespace retention period has expired.

### 2.7 Retire & Archive

- **Namespace retention** — configure per-environment. Closed workflow histories are automatically purged after the retention period.
- **Archival** (when compliance requires it) — configure the namespace to archive completed histories to S3:

```bash
temporal operator namespace update \
  --namespace <namespace> \
  --history-archival-state enabled \
  --history-uri "s3://<bucket>/temporal-archive"
```

- **Code archival** — retain deprecated workflow code in a `_deprecated/` subdirectory or tagged git release for audit purposes.

---

## 3. Code Organisation

### 3.1 Modular Worker Registration (Target)

The current unified worker registers everything in a single `main()` function (111 lines). As workflows grow, extract per-domain registration functions:

```go
// internal/application/workflows/register_lifecycle.go
package workflows

import "go.temporal.io/sdk/worker"

// RegisterLifecycleWorkflows registers all domain-lifecycle workflows
// and their activities on the given worker.
func RegisterLifecycleWorkflows(w worker.Worker, acts LifecycleActivities) {
    w.RegisterWorkflow(ExpiryLoop)
    w.RegisterWorkflow(PurgeLoop)
    w.RegisterWorkflow(RestoreWorkflow)
    w.RegisterWorkflow(SyncRegistrarsWorkflow)

    w.RegisterActivity(acts.CheckDomainCanAutoRenew)
    w.RegisterActivity(acts.GetExpiredDomainCount)
    // ... remaining activities
}
```

Then `main.go` becomes:

```go
lifecycleWorker := worker.New(client, lifecycleQueue, worker.Options{})
workflows.RegisterLifecycleWorkflows(lifecycleWorker, lifecycleActs)

escrowWorker := worker.New(client, escrowQueue, worker.Options{})
workflows.RegisterEscrowWorkflows(escrowWorker, escrowActs)

syncWorker := worker.New(client, syncQueue, worker.Options{})
workflows.RegisterSyncWorkflows(syncWorker, syncActs)
```

**Benefits:**
- PRs that add workflows only touch the relevant `register_*.go` file
- Compile-time safety — if an activity is missing, the registration function won't compile
- Easier code review — each domain is self-contained

### 3.2 Task Queue Strategy

The current 3-queue setup is appropriate. When adding new workflows, assign to queues based on:

| Queue | Use When |
|---|---|
| `object-lifecycle` | Domain CRUD, registrar management, anything that interacts with the core domain model |
| `data-pipeline` | Heavy data operations, imports, exports, cleanup, and periodic sync tasks (FX rates, external data pulls) |

> **IMPORTANT:** If you add a workflow that doesn't fit these categories (e.g., a reporting or analytics workflow), prefer adding a new task queue over overloading an existing one. Define the queue name as a constant, never hardcode strings in multiple places.

### 3.3 Temporal Client Configuration

All Temporal client configuration uses **environment variables managed by Doppler**:

| Env Var | Purpose | Default |
|---|---|---|
| `TEMPORAL_HOST_PORT` | Temporal server address | `temporal:7233` |
| `TEMPORAL_NAMESPACE` | Temporal namespace | `default` |
| `TEMPORAL_LIFECYCLE_QUEUE` | Lifecycle task queue | `object-lifecycle` |
| `TEMPORAL_DATA_QUEUE` | Data pipeline task queue | `data-pipeline` |
| `TEMPORAL_API_KEY` | API key auth (Temporal Cloud) | — |
| `TEMPORAL_CLIENT_CERT` | mTLS certificate (PEM) | — |
| `TEMPORAL_CLIENT_KEY` | mTLS private key (PEM) | — |
| `TEMPORAL_UI_URL` | Temporal UI base URL | `http://localhost:8081` |

**Rules:**
- Never read secrets directly — always via Doppler (`doppler run --`)
- When adding a new secret, use `doppler secrets set KEY=value`
- The `temporal.go` client factory handles auth tier selection automatically

---

## 4. Security & Compliance

### 4.1 Authentication & Authorisation

#### Current Model

| Layer | Mechanism |
|---|---|
| **REST API → Workflow trigger** | Auth0 JWT middleware (`authMiddleware`) |
| **Worker → Temporal Server** | API key / mTLS / plain (environment-dependent) |
| **Worker → domain-os API** | M2M credentials (`AUTH0_WORKER_CLIENT_ID/SECRET`) with fallback to `ADMIN_TOKEN` |

#### Per-Workflow Authorisation (Target)

Currently, all authenticated users can trigger any workflow. As the team grows, add application-level RBAC:

```go
// In the REST controller, before starting a workflow:
func (c *WorkflowController) StartTLDCleanup(ctx *gin.Context) {
    // Extract user claims from Auth0 JWT
    claims := auth.GetClaims(ctx)

    // Check permission — "destructive" workflows require elevated role
    if !claims.HasPermission("workflows:destructive") {
        ctx.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
        return
    }
    // ... proceed to start workflow
}
```

This is an **application-level concern** — Temporal itself doesn't enforce per-workflow RBAC (it operates at the namespace level).

### 4.2 Approval Attribution

When workflows use signal-based approval gates (like TLDCleanupWorkflow), **always include the approver's identity** in the signal payload. This creates an immutable audit record in Temporal's event history.

```go
// Signal payload — include who approved and when
type ApprovalSignal struct {
    Approved   bool   `json:"approved"`
    ApprovedBy string `json:"approvedBy"` // e.g. "g.prins@example.com"
    ApprovedAt string `json:"approvedAt"` // ISO 8601 timestamp
    Reason     string `json:"reason"`     // optional justification
}
```

The REST endpoint that sends the signal should extract the user identity from the Auth0 JWT and populate these fields server-side — never trust the client to self-report identity.

### 4.3 PII in Workflow Parameters

Workflow inputs, outputs, and activity parameters are stored **in plain text** in Temporal's event history. Be aware of what you're passing:

| Data Type | Current Risk | Guidance |
|---|---|---|
| Domain names | Low | Not PII — safe to include |
| TLD settings, pricing | Low | Business data — safe |
| Registrar email/contact | Medium | Pass identifiers (IDs) where possible, not raw contact data |
| User credentials | — | **Never** pass credentials as workflow parameters. Use context propagation (see portal auth pattern). |

**When compliance requires it**, implement a custom `DataConverter` with `PayloadCodec` for client-side encryption:

```go
client.Dial(client.Options{
    DataConverter: NewEncryptedDataConverter(defaultDataConverter, encryptionKey),
})
```

This encrypts all payloads before they reach Temporal. The server only processes opaque ciphertext.

### 4.4 Data Retention

| Environment | Recommended Retention | Rationale |
|---|---|---|
| Local/Dev | 24–72 hours | Minimise storage, fast iteration |
| OTE/Staging | 7 days | Enough for troubleshooting |
| Production | 30–90 days | Balance between audit needs and storage cost |

Configure via:

```bash
temporal operator namespace update \
  --namespace <namespace> \
  --retention 720h  # 30 days
```

For compliance requirements beyond retention, enable **archival** to S3 (see section 2.7).

---

## 5. Patterns Reference

### 5.1 Signal-Gate (Human-in-the-Loop Approval)

Use for any **destructive or irreversible** operation:

```go
func DestructiveWorkflow(ctx workflow.Context, params Params) error {
    var state WorkflowState
    workflow.SetQueryHandler(ctx, "get-state", func() (WorkflowState, error) {
        return state, nil
    })

    // Phase 1: Preview
    state.Phase = "previewing"
    var preview PreviewResult
    err := workflow.ExecuteActivity(ctx, GeneratePreview, params).Get(ctx, &preview)
    if err != nil {
        return err
    }
    state.Preview = preview

    // Phase 2: Wait for approval (with timeout)
    state.Phase = "awaiting-approval"
    approvalCh := workflow.GetSignalChannel(ctx, "approval")
    var approval ApprovalSignal

    selector := workflow.NewSelector(ctx)
    selector.AddReceive(approvalCh, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &approval)
    })

    timerCtx, cancelTimer := workflow.WithCancel(ctx)
    selector.AddFuture(workflow.NewTimer(timerCtx, 48*time.Hour), func(f workflow.Future) {
        approval = ApprovalSignal{Approved: false, Reason: "auto-rejected: 48h timeout"}
    })
    selector.Select(ctx)
    cancelTimer()

    if !approval.Approved {
        state.Phase = "rejected"
        return fmt.Errorf("workflow rejected: %s", approval.Reason)
    }

    // Phase 3: Execute
    state.Phase = "executing"
    state.ApprovedBy = approval.ApprovedBy
    // ... execute destructive operation

    state.Phase = "completed"
    return nil
}
```

### 5.2 Saga (Compensating Transactions)

Use for **multi-step resource creation** where partial failures need rollback:

```go
func CreateResourceWorkflow(ctx workflow.Context, params Params) error {
    var compensations []func(ctx workflow.Context) error

    // Step 1
    err := workflow.ExecuteActivity(ctx, Step1Create, params).Get(ctx, &result1)
    if err != nil {
        return err
    }
    compensations = append(compensations, func(ctx workflow.Context) error {
        return workflow.ExecuteActivity(ctx, Step1Delete, result1).Get(ctx, nil)
    })

    // Step 2
    err = workflow.ExecuteActivity(ctx, Step2Create, result1).Get(ctx, &result2)
    if err != nil {
        compensate(ctx, compensations) // Run compensations in reverse
        return err
    }

    return nil
}

func compensate(ctx workflow.Context, compensations []func(ctx workflow.Context) error) {
    // Use disconnected context so compensations run even if workflow is cancelled
    dctx, _ := workflow.NewDisconnectedContext(ctx)
    for i := len(compensations) - 1; i >= 0; i-- {
        if err := compensations[i](dctx); err != nil {
            workflow.GetLogger(ctx).Error("compensation failed", "error", err)
        }
    }
}
```

### 5.3 Child Workflow (Fire-and-Forget)

Use when a workflow should spawn an independent continuation:

```go
childOpts := workflow.ChildWorkflowOptions{
    WorkflowID:            fmt.Sprintf("ingest-%s", wfID),
    TaskQueue:             "escrow-import",
    ParentClosePolicy:     enums.PARENT_CLOSE_POLICY_ABANDON, // child survives parent
    WorkflowRunTimeout:    10 * time.Hour,
}
childCtx := workflow.WithChildOptions(ctx, childOpts)
workflow.ExecuteChildWorkflow(childCtx, IngestionWorkflow, ingestionParams)
// Don't .Get() — let it run independently
```

### 5.4 Scheduled Loops

Use for recurring operations. Define schedules in `internal/application/schedules/`:

```go
var MySchedule = client.ScheduleOptions{
    ID: "my-schedule",
    Spec: client.ScheduleSpec{
        Intervals: []client.ScheduleIntervalSpec{
            {Every: 1 * time.Hour, Offset: 15 * time.Minute},
        },
    },
    Action: &client.ScheduleWorkflowAction{
        Workflow: workflows.MyLoopWorkflow,
        Args:     []interface{}{},
        TaskQueue: "domain-lifecycle",
        // ... workflow options
    },
}
```

---

## 6. Improvement Backlog

Prioritised list of improvements. Items marked 🟢 are low-effort/high-impact and should be tackled first.

### 🟢 Priority 1 — Do Now

| # | Item | Effort | Impact |
|---|---|---|---|
| 1 | **Add replay tests** for all 8 workflows in CI | 1–2 days | Catches determinism violations before deployment |
| 2 | **Fix schedule file naming swap** — `purgeSchedule.go` and `updateFXSchedule.go` have swapped variable prefixes | 10 min | Removes confusing code smell |
| 3 | **Add `workflowcheck`** to CI pipeline | 30 min | Static analysis catches determinism issues at compile time |
| 4 | **Add query handlers** to workflows that lack them (ExpiryLoop, PurgeLoop, RestoreWorkflow, SyncRegistrarsWorkflow, UpdateFX) | 2–3 hours | Better observability for all workflows |

### 🟡 Priority 2 — Do Soon

| # | Item | Effort | Impact |
|---|---|---|---|
| 5 | **Extract modular worker registration** — per-domain `Register*()` functions | Half day | Cleaner PRs, easier code review, compile-time safety |
| 6 | **Standardise workflow IDs** across REST controllers and schedules | 1–2 hours | Consistent naming for searching and correlation |
| 7 | **Add approval attribution** to TLDCleanupWorkflow signal payload | 1–2 hours | Audit trail — who approved what |
| 8 | **Add workflow unit tests** for untested workflows (ExpiryLoop, PurgeLoop, RestoreWorkflow, SyncRegistrarsWorkflow, EscrowStaging/Ingestion, UpdateFX) | 2–3 days | Close the testing gap |

### 🔵 Priority 3 — When Needed

| # | Item | Effort | Trigger |
|---|---|---|---|
| 9 | **Adopt `GetVersion` patching** for workflow code changes | Per-change | When you need to change workflow logic with in-flight instances |
| 10 | **Configure namespace retention** per environment | 1 hour | When compliance or storage costs require it |
| 11 | **Add OpenTelemetry tracing** to Temporal client | Half day | When you need distributed tracing across services |
| 12 | **Implement per-workflow RBAC** in REST controllers | 1–2 days | When the operator team grows beyond trusted individuals |
| 13 | **Worker Versioning with Build IDs** | 1–2 days | When deployment frequency increases or long-running workflows are added |
| 14 | **Custom DataConverter** for PII encryption | 2–3 days | When external compliance audit requires encrypted payloads |
| 15 | **Temporal Nexus** for cross-namespace workflow communication | Investigation | When domain-os and registry-ops-tools workflows need to call each other |

---

## 7. Anti-Patterns to Avoid

| Anti-Pattern | Why | Do Instead |
|---|---|---|
| `MyWorkflow_v2` naming | Creates tech debt, doesn't integrate with Temporal versioning | Use `workflow.GetVersion()` or Worker Versioning |
| Direct I/O in workflows | Breaks determinism — file reads, HTTP calls, DB queries are non-deterministic | Move all I/O to activities |
| `time.Now()` in workflows | Non-deterministic — different value on replay | Use `workflow.Now(ctx)` |
| `fmt.Println` in workflows | Not replay-safe | Use `workflow.GetLogger(ctx)` |
| `go func()` in workflows | Goroutines break Temporal's execution model | Use `workflow.Go(ctx, func(ctx workflow.Context) { ... })` |
| `rand.Int()` in workflows | Non-deterministic | Use `workflow.SideEffect()` |
| Hardcoding task queue strings | Leads to drift between workers and callers | Define as constants in a shared package |
| Removing workflow code before draining | Running instances will hang forever | Drain first, then remove |
| Passing credentials in workflow params | Stored in plain text in event history | Use context propagation or activity-level auth |

---

## 8. Checklist — Adding a New Workflow

Use this checklist when adding a new workflow to domain-os:

- [ ] **Workflow definition** in `internal/application/workflows/{name}Workflow.go`
- [ ] **Single struct** for input params, single struct for output
- [ ] **Query handler** (`"get-state"`) registered for observability
- [ ] **Activity files** in `internal/application/activities/` (one function per file)
- [ ] **Explicit retry policies** on all activity options
- [ ] **Workflow unit test** in `workflows/{name}Workflow_test.go`
- [ ] **Activity unit tests** for each new activity
- [ ] **Replay test** in `workflows/{name}_replay_test.go`
- [ ] **Worker registration** in the appropriate `register_*.go` module
- [ ] **REST endpoint** (if user-triggered) in `interface/rest/workflow_controller.go`
  - [ ] Auth0 middleware applied
  - [ ] Workflow ID follows naming convention
  - [ ] Returns workflow ID, run ID, and Temporal UI link
- [ ] **Schedule definition** (if recurring) in `schedules/{name}Schedule.go`
- [ ] **Approval gate** (if destructive) with `ApprovedBy` attribution
- [ ] **Documentation** — update this guide or add a workflow-specific doc

---

## 9. Reference Links

| Topic | URL |
|---|---|
| Temporal Go SDK Docs | https://docs.temporal.io/develop/go |
| Versioning Guide | https://docs.temporal.io/develop/go/versioning |
| Testing Suite | https://docs.temporal.io/develop/go/testing-suite |
| Worker Deployments | https://docs.temporal.io/production-deployment/worker-deployments |
| Search Attributes | https://docs.temporal.io/visibility |
| Data Encryption | https://docs.temporal.io/production-deployment/data-encryption |
| Security (Self-Hosted) | https://docs.temporal.io/self-hosted-guide/security |
| Temporal Nexus | https://docs.temporal.io/nexus |
