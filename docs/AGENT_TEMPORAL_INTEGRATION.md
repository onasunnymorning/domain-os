# Agent + Temporal Workflows Integration

## 🎯 Executive Summary

**Temporal workflows are an EXCELLENT fit for advanced agent integration.** Your existing Temporal infrastructure (workflows for registrar sync, domain expiry, FX updates, etc.) provides a powerful foundation for the agent to orchestrate complex, long-running processes without re-inventing state machines.

**Key Insight:** Start with simple agent functions (Phase 1), then progressively leverage Temporal's heartbeats, signals, and queries to create **human-in-the-loop** workflows where agents and automated processes collaborate seamlessly.

---

## 📋 Table of Contents

1. [Why Temporal + Agent is Powerful](#why-temporal--agent-is-powerful)
2. [Integration Patterns](#integration-patterns)
3. [Evolution Path](#evolution-path)
4. [Use Case Examples](#use-case-examples)
5. [Implementation Guide](#implementation-guide)
6. [Best Practices](#best-practices)
7. [Migration Strategy](#migration-strategy)

---

## 🚀 Why Temporal + Agent is Powerful

### Current State Analysis

**You Already Have:**
```
internal/application/workflows/
├── syncRegistrarsWorkflow.go    # IANA/ICANN sync
├── expiryLoop.go                # Domain expiration handling
├── purgeLoop.go                 # Cleanup processes
├── restoreWorkflow.go           # Domain restoration
└── updateFX.go                  # Currency exchange updates
```

**Temporal Strengths:**
- ✅ **Durable state** - Workflow state persists across failures
- ✅ **Long-running** - Can run for days/weeks/months
- ✅ **Retry logic** - Automatic retry with backoff
- ✅ **Versioning** - Safe workflow updates
- ✅ **Signals** - External events can trigger workflow steps
- ✅ **Queries** - Check workflow status without interrupting
- ✅ **Heartbeats** - Activity progress tracking

### Agent + Temporal Synergy

| Feature | Agent Alone | Temporal Alone | **Agent + Temporal** |
|---------|-------------|----------------|---------------------|
| Natural language UI | ✅ | ❌ | ✅ |
| Durable state | ⚠️ (Redis) | ✅ | ✅ |
| Long-running (days+) | ❌ | ✅ | ✅ |
| Human-in-the-loop | ✅ | ⚠️ | ✅✅ |
| Automated steps | ⚠️ | ✅ | ✅ |
| Progress tracking | ⚠️ | ✅ (heartbeats) | ✅ |
| Approval gates | ✅ (chat) | ⚠️ (signals) | ✅✅ |
| Error recovery | ⚠️ | ✅ | ✅ |
| Audit trail | ⚠️ | ✅ | ✅ |

**Winner:** **Agent + Temporal** - Best of both worlds!

---

## 🎨 Integration Patterns

### Pattern 1: Agent Triggers Workflows (Simple)

**Use Case:** User asks agent to perform complex operation

```
User: "Setup a new TLD called .shop with sunrise phase"
  ↓
Agent: Analyzes intent, gathers info
  ↓
Agent: Starts Temporal workflow
  ↓
Workflow: Executes steps (create RO → TLD → Phase → Prices)
  ↓
Agent: Reports progress to user
```

**Implementation:**
```go
// Agent function
func (a *Agent) setupNewTLD(params SetupTLDParams) (string, error) {
    // Start Temporal workflow
    workflowOptions := client.StartWorkflowOptions{
        ID:        fmt.Sprintf("setup-tld-%s", params.Name),
        TaskQueue: "tld-setup",
    }
    
    we, err := a.temporalClient.ExecuteWorkflow(
        context.Background(),
        workflowOptions,
        workflows.SetupNewTLDWorkflow,
        params,
    )
    
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf(
        "Started TLD setup workflow. ID: %s. I'll monitor progress.",
        we.GetID(),
    ), nil
}
```

**Benefits:**
- ✅ Agent focuses on UX
- ✅ Workflow handles complexity
- ✅ Durable execution
- ✅ No state management in agent

---

### Pattern 2: Workflow Signals Agent (Human-in-the-Loop)

**Use Case:** Workflow needs human decision/approval

```
Workflow: Create TLD → ⏸️ Wait for pricing approval
  ↓ (signal via queue)
Agent: "The TLD .shop is ready. Proposed pricing: $10 base, $500 premium."
  ↓
User: "That premium is too high, make it $300"
  ↓
Agent: Sends signal to workflow
  ↓
Workflow: ▶️ Resume with approved pricing → Complete setup
```

**Implementation:**
```go
// Workflow with approval gate
func SetupNewTLDWorkflow(ctx workflow.Context, params SetupTLDParams) error {
    // ... create RO, TLD, Phase ...
    
    // Calculate default pricing
    pricing := calculateDefaultPricing(params)
    
    // Request approval (blocks here)
    var approvedPricing PricingApproval
    signalChan := workflow.GetSignalChannel(ctx, "pricing_approval")
    
    // Notify agent to request approval
    workflow.ExecuteActivity(ctx, 
        activities.NotifyAgentForApproval,
        "pricing_approval_needed",
        pricing,
    )
    
    // Wait for signal (with timeout)
    selector := workflow.NewSelector(ctx)
    selector.AddReceive(signalChan, func(c workflow.ReceiveChannel, more bool) {
        c.Receive(ctx, &approvedPricing)
    })
    selector.Select(ctx) // Blocks until signal received
    
    // Continue with approved pricing
    workflow.ExecuteActivity(ctx, 
        activities.CreatePricing,
        approvedPricing,
    )
    
    return nil
}
```

**Agent handling:**
```go
// Agent receives approval request via queue/webhook
func (a *Agent) handleApprovalRequest(req ApprovalRequest) {
    // Format message for user
    message := fmt.Sprintf(
        "The TLD .%s is ready. Proposed pricing:\n"+
        "- Base: $%d\n"+
        "- Premium: $%d\n"+
        "Do you approve?",
        req.TLD, req.Pricing.Base, req.Pricing.Premium,
    )
    
    // Send to user via chat
    a.sendMessage(req.UserID, message)
    
    // Store pending approval
    a.pendingApprovals[req.WorkflowID] = req
}

// Agent function for approval
func (a *Agent) approvePricing(workflowID string, pricing PricingApproval) error {
    // Send signal to workflow
    err := a.temporalClient.SignalWorkflow(
        context.Background(),
        workflowID,
        "",
        "pricing_approval",
        pricing,
    )
    
    return err
}
```

**Benefits:**
- ✅ Workflow pauses, not the user
- ✅ Conversation continues naturally
- ✅ State preserved in Temporal
- ✅ Agent just facilitates communication

---

### Pattern 3: Agent Monitors Workflow Progress (Heartbeats)

**Use Case:** Long-running workflow with progress updates

```
User: "Import 10,000 domains from CSV"
  ↓
Agent: Starts workflow
  ↓
Workflow: Processing... (heartbeat every 100 domains)
  ↓
Agent: Queries heartbeat, reports to user
  ↓
Agent: "Imported 2,500/10,000 domains (25% complete)"
```

**Implementation:**
```go
// Workflow activity with heartbeats
func ImportDomainsActivity(ctx context.Context, csvPath string) error {
    domains := readCSV(csvPath)
    total := len(domains)
    
    for i, domain := range domains {
        // Record heartbeat every 100 domains
        if i % 100 == 0 {
            activity.RecordHeartbeat(ctx, map[string]interface{}{
                "processed": i,
                "total": total,
                "percentage": float64(i) / float64(total) * 100,
            })
        }
        
        // Process domain
        createDomain(domain)
    }
    
    return nil
}
```

**Agent monitoring:**
```go
// Agent function to check progress
func (a *Agent) checkWorkflowProgress(workflowID string) (string, error) {
    // Query workflow for current state
    resp, err := a.temporalClient.QueryWorkflow(
        context.Background(),
        workflowID,
        "",
        "getProgress",
    )
    
    if err != nil {
        return "", err
    }
    
    var progress ProgressInfo
    resp.Get(&progress)
    
    return fmt.Sprintf(
        "Import progress: %d/%d domains (%.1f%% complete)",
        progress.Processed,
        progress.Total,
        progress.Percentage,
    ), nil
}

// Proactive updates (optional)
func (a *Agent) monitorWorkflow(workflowID string, userID string) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        progress, _ := a.checkWorkflowProgress(workflowID)
        
        // Send update to user
        a.sendMessage(userID, progress)
        
        // Stop when workflow completes
        desc, _ := a.temporalClient.DescribeWorkflowExecution(
            context.Background(),
            workflowID,
            "",
        )
        
        if desc.WorkflowExecutionInfo.Status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
            a.sendMessage(userID, "Import completed!")
            break
        }
    }
}
```

**Benefits:**
- ✅ User gets real-time updates
- ✅ No polling from frontend
- ✅ Agent provides natural language progress
- ✅ Workflow doesn't care about UI

---

### Pattern 4: Multi-Step Conversational Workflows

**Use Case:** Complex workflow where each step needs confirmation

```
User: "I want to launch a new TLD"
Agent: "Great! What's the TLD name?"
User: ".shop"
Agent: "Got it. What type? (generic/geographic/sponsored)"
User: "generic"
Agent: [Starts workflow with step 1 complete]
Workflow: Signal "Step 1 complete" → Wait for step 2
Agent: "Should I create a sunrise phase? (recommended)"
User: "Yes, 30 days"
Agent: [Signals workflow with step 2 data]
Workflow: Continue → Complete
```

**Implementation:**
```go
// Conversational workflow
func ConversationalTLDSetupWorkflow(ctx workflow.Context) error {
    var setup TLDSetup
    
    // Step 1: Get basic info
    step1Chan := workflow.GetSignalChannel(ctx, "basic_info")
    step1Chan.Receive(ctx, &setup.BasicInfo)
    
    // Notify agent: ask about phases
    workflow.ExecuteActivity(ctx, 
        activities.NotifyAgent,
        "ask_about_phases",
        setup,
    )
    
    // Step 2: Get phase config
    step2Chan := workflow.GetSignalChannel(ctx, "phase_config")
    step2Chan.Receive(ctx, &setup.PhaseConfig)
    
    // Notify agent: ask about pricing
    workflow.ExecuteActivity(ctx,
        activities.NotifyAgent,
        "ask_about_pricing",
        setup,
    )
    
    // Step 3: Get pricing
    step3Chan := workflow.GetSignalChannel(ctx, "pricing")
    step3Chan.Receive(ctx, &setup.Pricing)
    
    // Execute setup with all collected data
    workflow.ExecuteActivity(ctx,
        activities.CreateCompleteTLD,
        setup,
    )
    
    return nil
}
```

**Benefits:**
- ✅ Natural conversation flow
- ✅ State persisted across chat sessions
- ✅ User can pause and resume
- ✅ Agent focused on conversation, not state

---

## 🗺️ Evolution Path

### Phase 1: Simple Agent (Week 1-4)
**No Temporal integration yet**

```
Agent Functions:
- create_registry_operator
- create_tld
- create_phase
- list_tlds
```

**Why wait?**
- Prove agent value first
- Learn user patterns
- Keep initial complexity low

---

### Phase 2: Agent Triggers Workflows (Week 5-8)
**Agent starts workflows, monitors completion**

```go
// New agent functions
- start_tld_setup_workflow(name, type, phases)
- start_bulk_import_workflow(csv_url)
- start_registrar_sync_workflow()
- check_workflow_status(workflow_id)
```

**Value:**
- ✅ Leverage existing workflows
- ✅ Agent provides natural language wrapper
- ✅ Durable execution

**Example:**
```
User: "Sync our registrars with ICANN"
Agent: Calls start_registrar_sync_workflow()
       "Started registrar sync (ID: sync-abc123)"
User: "What's the status?"
Agent: Calls check_workflow_status("sync-abc123")
       "Sync in progress: 247/500 registrars updated"
```

---

### Phase 3: Workflow Signals (Week 9-12)
**Workflows request human decisions**

```go
// Workflows send signals to agent
workflows.SetupNewTLDWorkflow:
  - Signal: "approval_needed" (pricing)
  - Wait for: "approval_granted" signal
  
workflows.BulkDomainTransferWorkflow:
  - Signal: "conflict_detected" (duplicate domains)
  - Wait for: "resolution" signal
```

**Value:**
- ✅ Human-in-the-loop automation
- ✅ No blocking users
- ✅ Intelligent decision points

**Example:**
```
[Workflow detects pricing needs approval]
Agent: "The calculated pricing for .shop is $15 base, $600 premium.
        This is 20% higher than .store. Approve or adjust?"
User: "Lower premium to $500"
Agent: Signals workflow with adjusted pricing
[Workflow continues with approval]
```

---

### Phase 4: Advanced Orchestration (Week 13+)
**Full agent + workflow collaboration**

```go
// Complex scenarios
- Multi-TLD setup with dependencies
- Phased domain migrations
- Conditional approval chains
- Dynamic workflow modification
```

**Value:**
- ✅ Handle complex real-world scenarios
- ✅ Agent + automation working together
- ✅ Maximum flexibility

**Example:**
```
User: "Launch .shop and .shopping as a bundle with shared pricing"
Agent: Analyzes requirements
       Starts custom workflow with 2 parallel TLD setups
       Coordinates shared pricing approval
       Reports unified progress
       "Bundle setup complete: .shop and .shopping are live"
```

---

## 💡 Use Case Examples

### Use Case 1: TLD Launch with Approval Gates

**Scenario:** Launch new TLD with multiple approval points

**Flow:**
```
1. User: "Launch .coffee TLD"
   ↓
2. Agent: Gathers info via conversation
   ↓
3. Agent: Starts TLDLaunchWorkflow
   ↓
4. Workflow: Creates RO, TLD, calculates pricing
   ↓ [APPROVAL GATE 1]
5. Workflow → Agent: "Approve pricing?"
6. Agent → User: "Pricing: $12 base, $400 premium. Approve?"
7. User → Agent: "Approved"
8. Agent → Workflow: Signal approval
   ↓
9. Workflow: Creates phases, sets up DNS
   ↓ [APPROVAL GATE 2]
10. Workflow → Agent: "Ready to go live?"
11. Agent → User: "All configured. Launch now?"
12. User → Agent: "Yes, go live"
13. Agent → Workflow: Signal launch
   ↓
14. Workflow: Activates TLD, notifies registrars
   ↓
15. Agent: "🎉 .coffee is now live!"
```

**Temporal Workflow:**
```go
func TLDLaunchWorkflow(ctx workflow.Context, params LaunchParams) error {
    // Step 1: Create resources
    var tld TLD
    workflow.ExecuteActivity(ctx, activities.CreateTLDResources, params).Get(ctx, &tld)
    
    // Step 2: Wait for pricing approval
    var pricingApproval PricingApproval
    workflow.GetSignalChannel(ctx, "pricing_approval").Receive(ctx, &pricingApproval)
    workflow.ExecuteActivity(ctx, activities.SetupPricing, pricingApproval)
    
    // Step 3: Configure infrastructure
    workflow.ExecuteActivity(ctx, activities.SetupDNS, tld)
    workflow.ExecuteActivity(ctx, activities.CreatePhases, params.Phases)
    
    // Step 4: Wait for launch approval
    var launchApproval LaunchApproval
    workflow.GetSignalChannel(ctx, "launch_approval").Receive(ctx, &launchApproval)
    
    // Step 5: Go live
    workflow.ExecuteActivity(ctx, activities.ActivateTLD, tld)
    workflow.ExecuteActivity(ctx, activities.NotifyRegistrars, tld)
    
    return nil
}
```

**Agent Functions:**
```go
func setupTLDLaunch(params LaunchParams) {
    // Start workflow
    workflowID := startWorkflow("TLDLaunchWorkflow", params)
    
    // Monitor for approval requests
    monitorWorkflowSignals(workflowID)
}
```

---

### Use Case 2: Bulk Domain Import with Progress

**Scenario:** Import 50,000 domains from CSV

**Flow:**
```
User: "Import domains from this CSV" [uploads file]
  ↓
Agent: Validates CSV, starts ImportWorkflow
  ↓
Workflow: Processes in batches of 1000
  ↓ [Heartbeat every batch]
Agent: Queries progress every 30 seconds
Agent → User: "Progress: 15,000/50,000 (30%)"
  ↓
[2 hours later]
Agent → User: "Import complete! 49,847 succeeded, 153 failed"
User: "Why did some fail?"
Agent: Queries workflow history
Agent → User: "Failed domains had invalid DNS configs. 
               View error report: [link]"
```

**Temporal Workflow:**
```go
func BulkImportDomainsWorkflow(ctx workflow.Context, csvPath string) (*ImportResult, error) {
    domains := readCSV(csvPath)
    batches := chunkDomains(domains, 1000)
    
    results := &ImportResult{}
    
    for i, batch := range batches {
        var batchResult BatchResult
        
        // Process batch with heartbeat
        err := workflow.ExecuteActivity(ctx,
            activities.ProcessDomainBatch,
            batch,
        ).Get(ctx, &batchResult)
        
        // Aggregate results
        results.Succeeded += batchResult.Succeeded
        results.Failed += batchResult.Failed
        results.Errors = append(results.Errors, batchResult.Errors...)
        
        // Record progress
        workflow.UpsertSearchAttributes(ctx, map[string]interface{}{
            "Progress": float64(i+1) / float64(len(batches)) * 100,
        })
    }
    
    return results, nil
}

// Activity with heartbeat
func ProcessDomainBatch(ctx context.Context, batch []Domain) (BatchResult, error) {
    result := BatchResult{}
    
    for i, domain := range batch {
        // Heartbeat every 100 domains
        if i % 100 == 0 {
            activity.RecordHeartbeat(ctx, i)
        }
        
        if err := createDomain(domain); err != nil {
            result.Failed++
            result.Errors = append(result.Errors, DomainError{
                Domain: domain.Name,
                Error: err.Error(),
            })
        } else {
            result.Succeeded++
        }
    }
    
    return result, nil
}
```

---

### Use Case 3: Registrar Sync with Error Recovery

**Scenario:** Sync registrars with IANA/ICANN (existing workflow)

**Enhanced with Agent:**
```
User: "Sync our registrar list"
  ↓
Agent: Starts SyncRegistrarsWorkflow (existing!)
  ↓
[IANA API is down]
Workflow: Retries with backoff (Temporal handles this)
  ↓
Agent: Detects retry, notifies user
Agent → User: "IANA API is slow, retrying (attempt 2/3)"
  ↓
[Sync succeeds on retry 2]
Agent → User: "Sync complete! Updated 247 registrars, added 3 new"
```

**Agent Enhancement to Existing Workflow:**
```go
// Existing workflow (no changes needed!)
func SyncRegistrarsWorkflow(ctx workflow.Context, batchsize int) error {
    // ... existing code ...
}

// New agent wrapper
func (a *Agent) syncRegistrars() (string, error) {
    we, err := a.temporalClient.ExecuteWorkflow(
        context.Background(),
        client.StartWorkflowOptions{
            ID: fmt.Sprintf("sync-registrars-%s", time.Now().Format("20060102")),
            TaskQueue: "registrar-sync",
        },
        workflows.SyncRegistrarsWorkflow,
        100, // batchsize
    )
    
    if err != nil {
        return "", err
    }
    
    // Monitor in background
    go a.monitorSyncProgress(we.GetID())
    
    return fmt.Sprintf(
        "Started registrar sync. I'll notify you of progress.",
    ), nil
}
```

**Zero workflow changes, instant agent integration!** 🎉

---

## 📐 Implementation Guide

### Step 1: Expose Workflow Client to Agent

```go
// internal/agent/service/agent_service.go
type AgentService struct {
    adminClient    *client.AdminAPIClient
    temporalClient client.Client  // Add this
    llmClient      *openai.Client
}

func NewAgentService(
    adminClient *client.AdminAPIClient,
    temporalClient client.Client,
    llmClient *openai.Client,
) *AgentService {
    return &AgentService{
        adminClient:    adminClient,
        temporalClient: temporalClient,
        llmClient:      llmClient,
    }
}
```

### Step 2: Create Workflow-Trigger Functions

```go
// internal/agent/functions/workflow_functions.go
package functions

func (f *Functions) StartTLDSetupWorkflow(params TLDSetupParams) (string, error) {
    we, err := f.temporalClient.ExecuteWorkflow(
        context.Background(),
        client.StartWorkflowOptions{
            ID:        fmt.Sprintf("tld-setup-%s", params.Name),
            TaskQueue: "tld-setup",
        },
        workflows.SetupNewTLDWorkflow,
        params,
    )
    
    if err != nil {
        return "", fmt.Errorf("failed to start workflow: %w", err)
    }
    
    return fmt.Sprintf(
        "Started TLD setup workflow for .%s (ID: %s). "+
        "This will take about 5 minutes. I'll notify you when complete.",
        params.Name,
        we.GetID(),
    ), nil
}
```

### Step 3: Register Functions with LLM

```go
// Add to function definitions
{
    Type: "function",
    Function: openai.FunctionDefinition{
        Name: "start_tld_setup_workflow",
        Description: "Start a comprehensive TLD setup workflow including " +
            "RO creation, TLD creation, phase setup, and pricing configuration. "+
            "Use for complex TLD launches.",
        Parameters: jsonschema.Definition{
            Type: jsonschema.Object,
            Properties: map[string]jsonschema.Definition{
                "name": {
                    Type:        jsonschema.String,
                    Description: "TLD name without dot (e.g., 'shop')",
                },
                "type": {
                    Type:        jsonschema.String,
                    Description: "TLD type: generic, geographic, or sponsored",
                    Enum:        []string{"generic", "geographic", "sponsored"},
                },
                // ... more parameters
            },
            Required: []string{"name", "type"},
        },
    },
}
```

### Step 4: Monitor Workflow Progress

```go
// internal/agent/functions/workflow_monitor.go
func (f *Functions) MonitorWorkflowProgress(workflowID string) error {
    // Subscribe to workflow updates
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        desc, err := f.temporalClient.DescribeWorkflowExecution(
            context.Background(),
            workflowID,
            "",
        )
        
        if err != nil {
            return err
        }
        
        // Check if completed
        if desc.WorkflowExecutionInfo.Status != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
            f.notifyUser(workflowID, "Workflow completed!")
            break
        }
        
        // Query for progress (if workflow supports it)
        var progress WorkflowProgress
        resp, _ := f.temporalClient.QueryWorkflow(
            context.Background(),
            workflowID,
            "",
            "getProgress",
        )
        resp.Get(&progress)
        
        // Send update
        f.notifyUser(workflowID, fmt.Sprintf(
            "Progress: %d%% complete",
            progress.Percentage,
        ))
    }
    
    return nil
}
```

### Step 5: Handle Workflow Signals (Approval Gates)

```go
// Workflow requests approval
func SetupTLDWithApprovalWorkflow(ctx workflow.Context, params TLDParams) error {
    // ... setup steps ...
    
    // Request approval
    workflow.ExecuteActivity(ctx,
        activities.RequestAgentApproval,
        ApprovalRequest{
            WorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
            Type:       "pricing_approval",
            Data:       pricing,
        },
    )
    
    // Wait for signal
    var approval PricingApproval
    signalChan := workflow.GetSignalChannel(ctx, "pricing_approval")
    signalChan.Receive(ctx, &approval)
    
    // Continue with approval
    // ...
}

// Activity notifies agent
func RequestAgentApproval(ctx context.Context, req ApprovalRequest) error {
    // Send to agent via queue/webhook
    return publishToAgentQueue(req)
}

// Agent receives and processes
func (a *Agent) handleApprovalRequest(req ApprovalRequest) {
    message := formatApprovalMessage(req)
    a.sendToUser(req.UserID, message)
    
    // Store pending approval
    a.pendingApprovals[req.WorkflowID] = req
}

// Agent function to approve
func (a *Agent) approvePricing(workflowID string, approval PricingApproval) error {
    return a.temporalClient.SignalWorkflow(
        context.Background(),
        workflowID,
        "",
        "pricing_approval",
        approval,
    )
}
```

---

## 🎯 Best Practices

### 1. Workflow Design for Agent Integration

**DO:**
- ✅ Use signals for all approval gates
- ✅ Implement progress queries
- ✅ Emit heartbeats from long activities
- ✅ Return detailed error messages
- ✅ Use search attributes for filtering

**DON'T:**
- ❌ Put UI logic in workflows
- ❌ Make workflows depend on agent
- ❌ Block without timeouts
- ❌ Forget error handling
- ❌ Skip activity retries

### 2. Agent Function Design

**DO:**
- ✅ Provide natural language feedback
- ✅ Monitor workflow status proactively
- ✅ Handle workflow errors gracefully
- ✅ Support workflow cancellation
- ✅ Store workflow IDs in conversation context

**DON'T:**
- ❌ Duplicate workflow logic in agent
- ❌ Poll excessively
- ❌ Ignore workflow failures
- ❌ Forget to clean up monitoring

### 3. Communication Patterns

**Agent → Workflow:**
- Use `ExecuteWorkflow` to start
- Use `SignalWorkflow` for approvals/updates
- Use `CancelWorkflow` for cancellations

**Workflow → Agent:**
- Use activity to publish to queue/webhook
- Agent polls queue for requests
- Or use webhook for immediate notifications

### 4. State Management

**Where State Lives:**
- ✅ **Temporal:** Workflow state, execution history
- ✅ **Agent (Redis):** Conversation context, pending approvals
- ✅ **Backend:** Business data (DB)

**Don't duplicate state!**

---

## 🚀 Migration Strategy

### From Current State → Agent + Temporal

**Current Workflows (Keep as-is):**
```
✅ syncRegistrarsWorkflow     - Already great!
✅ expiryLoop                 - No changes needed
✅ purgeLoop                  - Perfect as-is
✅ updateFX                   - Works well
✅ restoreWorkflow            - Good to go
```

**Phase 1: Add Agent Wrappers (Week 5-6)**
```go
// Zero workflow changes, just add agent functions

func (a *Agent) syncRegistrars() {
    startWorkflow("SyncRegistrarsWorkflow")
}

func (a *Agent) checkRegistrarSyncStatus(workflowID string) {
    queryWorkflow(workflowID, "getProgress")
}
```

**Phase 2: Enhance with Progress Monitoring (Week 7-8)**
```go
// Add progress queries to existing workflows

func SyncRegistrarsWorkflow(ctx workflow.Context, batchsize int) error {
    // Existing code...
    
    // NEW: Register query handler
    err := workflow.SetQueryHandler(ctx, "getProgress", func() (ProgressInfo, error) {
        return ProgressInfo{
            Processed: processedCount,
            Total:     totalCount,
            Status:    currentStatus,
        }, nil
    })
    
    // Continue existing logic...
}
```

**Phase 3: Add Approval Gates (Week 9-10)**
```go
// Create new workflows with approval gates

func SetupNewTLDWorkflow(ctx workflow.Context, params TLDParams) error {
    // New workflow with agent collaboration
    
    // Step 1: Auto-create RO and TLD
    workflow.ExecuteActivity(ctx, activities.CreateTLD, params)
    
    // Step 2: Request pricing approval
    workflow.ExecuteActivity(ctx, activities.NotifyAgent, "approval_needed")
    
    var approval PricingApproval
    workflow.GetSignalChannel(ctx, "approval").Receive(ctx, &approval)
    
    // Step 3: Complete with approval
    workflow.ExecuteActivity(ctx, activities.Finalize, approval)
    
    return nil
}
```

**Phase 4: Advanced Orchestration (Week 11+)**
```go
// Complex multi-workflow coordination

func LaunchTLDBundleWorkflow(ctx workflow.Context, bundle TLDBundle) error {
    // Parallel TLD setups
    childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
        WorkflowID: "tld-1",
    })
    
    future1 := workflow.ExecuteChildWorkflow(childCtx, SetupNewTLDWorkflow, bundle.TLD1)
    future2 := workflow.ExecuteChildWorkflow(childCtx, SetupNewTLDWorkflow, bundle.TLD2)
    
    // Wait for both
    future1.Get(ctx, nil)
    future2.Get(ctx, nil)
    
    // Coordinated pricing approval for bundle
    // ...
}
```

---

## 📊 Success Metrics

### Technical Metrics
- ✅ Workflow success rate: >95%
- ✅ Agent-triggered workflows: Track adoption
- ✅ Approval response time: <2 minutes avg
- ✅ Long-running workflow stability: 0 crashes

### User Metrics
- ✅ Complex task completion: 80%+ with agent vs 40% without
- ✅ Time to complete TLD setup: 50% reduction
- ✅ User satisfaction: "I prefer agent-assisted workflows"
- ✅ Error recovery: 90%+ successful with agent guidance

### Business Metrics
- ✅ Reduced support tickets for complex workflows
- ✅ Faster TLD launches (days → hours)
- ✅ Fewer manual errors in multi-step processes
- ✅ Increased self-service adoption

---

## 🎓 Key Takeaways

### 1. **Perfect Fit for Domain-OS**
You already have Temporal! Adding agent integration is a natural evolution, not a rewrite.

### 2. **Start Simple, Grow Complex**
- Week 1-4: Agent with basic functions
- Week 5-6: Agent triggers existing workflows
- Week 7-8: Add progress monitoring
- Week 9+: Human-in-the-loop workflows

### 3. **Don't Re-invent State Machines**
Temporal handles:
- ✅ State persistence
- ✅ Retries
- ✅ Long-running processes
- ✅ Workflow versioning

Agent focuses on:
- ✅ Natural language UX
- ✅ User communication
- ✅ Intent understanding

### 4. **Leverage Signals & Heartbeats**
These are PERFECT for agent integration:
- **Signals** = Approval gates
- **Heartbeats** = Progress updates
- **Queries** = Status checks

### 5. **Zero Workflow Changes for Phase 1**
Your existing workflows (`syncRegistrarsWorkflow`, etc.) can be agent-accessible immediately with just wrapper functions!

---

## 🚀 Quick Start

### Try This Today (15 minutes)

```go
// 1. Add agent function to trigger existing workflow
func (a *Agent) syncRegistrars() (string, error) {
    we, err := a.temporalClient.ExecuteWorkflow(
        context.Background(),
        client.StartWorkflowOptions{
            ID: "sync-" + time.Now().Format("20060102-150405"),
            TaskQueue: "registrar-sync",
        },
        workflows.SyncRegistrarsWorkflow,
        100, // batchsize
    )
    
    if err != nil {
        return "", err
    }
    
    return fmt.Sprintf("Started registrar sync: %s", we.GetID()), nil
}

// 2. Register with LLM
functions := []openai.Tool{
    {
        Type: "function",
        Function: openai.FunctionDefinition{
            Name: "sync_registrars",
            Description: "Sync our registrar list with IANA and ICANN",
        },
    },
}

// 3. Test it!
// User: "Sync our registrar list"
// Agent: Calls sync_registrars()
// Agent: "Started registrar sync: sync-20250110-143022"
```

**That's it!** You've integrated your first workflow with the agent. 🎉

---

## 📚 Resources

- [Temporal Documentation](https://docs.temporal.io/)
- [Signals & Queries](https://docs.temporal.io/dev-guide/go/features#signals)
- [Heartbeats](https://docs.temporal.io/dev-guide/go/features#heartbeats)
- [Child Workflows](https://docs.temporal.io/dev-guide/go/features#child-workflows)
- Your existing workflows: `internal/application/workflows/`

---

## ✅ Conclusion

**Agent + Temporal is the PERFECT combination for Domain-OS:**

1. **Temporal** handles complex orchestration, state, retries
2. **Agent** provides natural language interface, gathers user intent
3. **Together** they create seamless human-in-the-loop workflows

**Start simple** (Week 5-6): Wrap existing workflows  
**Grow naturally** (Week 7+): Add progress, approvals, coordination

**No need to re-invent workflow state machines** - Temporal already did it! 🚀

---

*Document version: 1.0*  
*Last updated: January 10, 2025*  
*Recommendation: Integrate Temporal starting Phase 2 (Week 5)*
