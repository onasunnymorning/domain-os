package rest

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

type WorkflowController struct{}

type startRegistrarSyncRequest struct {
	BatchSize int    `json:"batchSize"`
	Mode      string `json:"mode"` // reserved for future use ("init"|"sync")
}

type startWorkflowResponse struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	URL        string `json:"url"`
}

type workflowRegistryResponse struct {
	Items []workflows.WorkflowMeta `json:"items"`
	Count int                      `json:"count"`
}

type launchWorkflowRequest struct {
	WorkflowType string                 `json:"workflowType" binding:"required"`
	Params       map[string]interface{} `json:"params"`
}

type launchWorkflowResponse struct {
	WorkflowID string                 `json:"workflowId"`
	RunID      string                 `json:"runId"`
	Status     string                 `json:"status"`
	URL        string                 `json:"url"`
	Steps      []workflows.WorkflowStep `json:"steps"`
}

type activeWorkflowItem struct {
	WorkflowID   string `json:"workflowId"`
	RunID        string `json:"runId"`
	WorkflowType string `json:"workflowType"`
	Status       string `json:"status"`
	StartTime    string `json:"startTime"`
	URL          string `json:"url"`
}

type activeWorkflowsResponse struct {
	Items []activeWorkflowItem `json:"items"`
	Count int                  `json:"count"`
}

type workflowStatusResponse struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	StartTime  string `json:"startTime,omitempty"`
	CloseTime  string `json:"closeTime,omitempty"`
	URL        string `json:"url"`
}

type signalWorkflowRequest struct {
	SignalName string      `json:"signalName" binding:"required"`
	Payload    interface{} `json:"payload"`
}

// buildTemporalUILink constructs a clickable Temporal UI link for a workflow execution.
func buildTemporalUILink(namespace, workflowID, runID string) string {
	base := os.Getenv("TEMPORAL_UI_URL")
	base = strings.Trim(base, "\"'")
	if base == "" {
		base = "http://localhost:8081"
	}
	link := base + "/namespaces/" + namespace + "/workflows/" + workflowID
	if runID != "" {
		link += "/" + runID
	}
	return link
}

// normalizeStatus converts Temporal's protobuf status strings
// (e.g., "WORKFLOW_EXECUTION_STATUS_RUNNING") to our short form ("RUNNING").
func normalizeStatus(raw string) string {
	const prefix = "WORKFLOW_EXECUTION_STATUS_"
	if strings.HasPrefix(raw, prefix) {
		return raw[len(prefix):]
	}
	return strings.ToUpper(raw)
}

func NewWorkflowController(e *gin.Engine, handler gin.HandlerFunc) *WorkflowController {
	controller := &WorkflowController{}
	grp := e.Group("/workflows", handler)
	{
		grp.POST("/registrars/sync", controller.StartRegistrarSync)
		grp.POST("/tlds/:tldName/cleanup", controller.StartTLDCleanup)
		grp.POST("/tlds/:tldName/cleanup/confirm", controller.ConfirmTLDCleanup)
		grp.GET("/registry", controller.GetRegistry)
		grp.POST("/launch", controller.LaunchWorkflow)
		grp.GET("/active", controller.ListActiveWorkflows)
		grp.GET("/:workflowId/status", controller.GetWorkflowStatus)
		grp.GET("/:workflowId/result", controller.GetWorkflowResult)
		grp.POST("/:workflowId/signal", controller.SignalWorkflow)
		grp.GET("/storage/download", controller.GetDownloadURL)
	}
	return controller
}

// StartRegistrarSync starts the SyncRegistrarsWorkflow in Temporal
// @Summary Start registrar sync workflow
// @Description Starts the Temporal workflow that bootstraps or syncs registrars depending on system state
// @Tags Workflows
// @Accept json
// @Produce json
// @Param request body startRegistrarSyncRequest false "Workflow options"
// @Success 202 {object} startWorkflowResponse
// @Failure 500 {object} map[string]string
// @Router /workflows/registrars/sync [post]
func (c *WorkflowController) StartRegistrarSync(ctx *gin.Context) {
	var req startRegistrarSyncRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// Default when body is empty
		req = startRegistrarSyncRequest{BatchSize: 100}
	}
	if req.BatchSize <= 0 {
		req.BatchSize = 100
	}

	cfg := temporal.NewClientConfigFromEnv(temporal.QueueLifecycle)

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{
		ID:        "sync-registrars-" + time.Now().Format("20060102-150405"),
		TaskQueue: cfg.WorkerQueue,
	}, workflows.SyncRegistrarsWorkflow, req.BatchSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, startWorkflowResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "started",
		URL:        buildTemporalUILink(cfg.Namespace, we.GetID(), we.GetRunID()),
	})
}

type startTLDCleanupRequest struct {
	KeepTLDAndPhases bool `json:"keepTLDAndPhases"`
}

// StartTLDCleanup starts the TLDCleanupWorkflow in Temporal
// @Summary Start TLD cleanup workflow
// @Description Triggers a background Temporal workflow to safely delete a TLD and its assets
// @Tags Workflows
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param request body startTLDCleanupRequest true "Options"
// @Success 202 {object} startWorkflowResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /workflows/tlds/{tldName}/cleanup [post]
func (c *WorkflowController) StartTLDCleanup(ctx *gin.Context) {
	name := ctx.Param("tldName")

	var req startTLDCleanupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		// If empty body is sent, default should be false anyway
		req.KeepTLDAndPhases = false
	}

	cfg := temporal.NewClientConfigFromEnv(temporal.QueueData)

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	wfID := fmt.Sprintf("tld-cleanup-%s-%s", name, time.Now().Format("20060102-150405"))

	params := workflows.TLDCleanupParams{
		TLD:              name,
		KeepTLDAndPhases: req.KeepTLDAndPhases,
	}

	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{
		ID:        wfID,
		TaskQueue: cfg.WorkerQueue,
	}, workflows.TLDCleanupWorkflow, params)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start cleanup workflow: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, startWorkflowResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "started",
		URL:        buildTemporalUILink(cfg.Namespace, we.GetID(), we.GetRunID()),
	})
}

type confirmTLDCleanupRequest struct {
	WorkflowID string `json:"workflowId" binding:"required"`
	Confirm    bool   `json:"confirm"`
}

// ConfirmTLDCleanup sends the ConfirmTLDCleanup signal to a running TLDCleanupWorkflow
// @Summary Confirm or reject TLD cleanup
// @Description Sends the ConfirmTLDCleanup signal to a running TLD cleanup workflow
// @Tags Workflows
// @Accept json
// @Produce json
// @Param tldName path string true "TLD Name"
// @Param request body confirmTLDCleanupRequest true "Confirmation"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /workflows/tlds/{tldName}/cleanup/confirm [post]
func (c *WorkflowController) ConfirmTLDCleanup(ctx *gin.Context) {
	var req confirmTLDCleanupRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "workflowId is required"})
		return
	}

	cfg := temporal.NewClientConfigFromEnv("")

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	err = cli.SignalWorkflow(ctx.Request.Context(), req.WorkflowID, "", "ConfirmTLDCleanup", req.Confirm)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send signal: " + err.Error()})
		return
	}

	action := "confirmed"
	if !req.Confirm {
		action = "rejected"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":     action,
		"workflowId": req.WorkflowID,
	})
}

// GetRegistry returns the workflow type registry
// @Summary Get workflow registry
// @Description Returns metadata for all available workflow types including steps, tags, and configuration
// @Tags Workflows
// @Produce json
// @Success 200 {object} workflowRegistryResponse
// @Router /workflows/registry [get]
func (c *WorkflowController) GetRegistry(ctx *gin.Context) {
	items := workflows.GetWorkflowRegistry()
	ctx.JSON(http.StatusOK, workflowRegistryResponse{
		Items: items,
		Count: len(items),
	})
}

// LaunchWorkflow starts a workflow by type
// @Summary Launch a workflow
// @Description Starts a Temporal workflow of the specified type with the given parameters
// @Tags Workflows
// @Accept json
// @Produce json
// @Param request body launchWorkflowRequest true "Launch parameters"
// @Success 202 {object} launchWorkflowResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /workflows/launch [post]
func (c *WorkflowController) LaunchWorkflow(ctx *gin.Context) {
	var req launchWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "workflowType is required"})
		return
	}

	meta, ok := workflows.GetWorkflowMeta(req.WorkflowType)
	if !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unknown workflow type: %s", req.WorkflowType)})
		return
	}

	cfg := temporal.NewClientConfigFromEnv(meta.Queue)

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	ts := time.Now().Format("20060102-150405")
	var wfID string
	var workflow interface{}
	var args []interface{}

	switch req.WorkflowType {
	case "escrow-import":
		tld, _ := req.Params["tld"].(string)
		objectKey, _ := req.Params["objectKey"].(string)
		options, _ := req.Params["options"].(map[string]interface{})
		if tld == "" || objectKey == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld and objectKey are required for escrow-import"})
			return
		}
		wfID = fmt.Sprintf("escrow-import-%s-%s", tld, ts)
		workflow = workflows.EscrowImportWorkflow
		args = []interface{}{workflows.EscrowImportParams{TLD: tld, ObjectKey: objectKey, Options: options}}

	case "tld-cleanup":
		tld, _ := req.Params["tld"].(string)
		if tld == "" {
			tld, _ = req.Params["tldName"].(string)
		}
		if tld == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld is required for tld-cleanup"})
			return
		}
		keepTLDAndPhases, _ := req.Params["keepTLDAndPhases"].(bool)
		wfID = fmt.Sprintf("tld-cleanup-%s-%s", tld, ts)
		workflow = workflows.TLDCleanupWorkflow
		args = []interface{}{workflows.TLDCleanupParams{TLD: tld, KeepTLDAndPhases: keepTLDAndPhases}}

	case "sync-registrars":
		batchSize := 100
		if bs, ok := req.Params["batchSize"].(float64); ok && bs > 0 {
			batchSize = int(bs)
		}
		wfID = fmt.Sprintf("sync-registrars-%s", ts)
		workflow = workflows.SyncRegistrarsWorkflow
		args = []interface{}{batchSize}

	case "update-fx":
		wfID = fmt.Sprintf("update-fx-%s", ts)
		workflow = workflows.UpdateFX
		args = nil

	case "expiry-loop":
		var loopParams workflows.ExpiryLoopParams
		if req.Params != nil {
			if bs, ok := req.Params["batchSize"].(float64); ok && bs > 0 {
				loopParams.BatchSize = int(bs)
			}
			if cl, ok := req.Params["concurrencyLimit"].(float64); ok && cl > 0 {
				loopParams.ConcurrencyLimit = int(cl)
			}
			if dr, ok := req.Params["dryRun"].(bool); ok {
				loopParams.DryRun = dr
			}
			if rto, ok := req.Params["referenceTimeOverride"].(string); ok && rto != "" {
				if parsedTime, err := time.Parse(time.RFC3339, rto); err == nil {
					loopParams.ReferenceTimeOverride = &parsedTime
				}
			}
		}
		wfID = fmt.Sprintf("expiry-loop-%s", ts)
		workflow = workflows.ExpiryLoop
		args = []interface{}{loopParams}

	case "purge-loop":
		var loopParams workflows.PurgeLoopParams
		if req.Params != nil {
			if bs, ok := req.Params["batchSize"].(float64); ok && bs > 0 {
				loopParams.BatchSize = int(bs)
			}
			if cl, ok := req.Params["concurrencyLimit"].(float64); ok && cl > 0 {
				loopParams.ConcurrencyLimit = int(cl)
			}
			if dr, ok := req.Params["dryRun"].(bool); ok {
				loopParams.DryRun = dr
			}
			if rto, ok := req.Params["referenceTimeOverride"].(string); ok && rto != "" {
				if parsedTime, err := time.Parse(time.RFC3339, rto); err == nil {
					loopParams.ReferenceTimeOverride = &parsedTime
				}
			}
		}
		wfID = fmt.Sprintf("purge-loop-%s", ts)
		workflow = workflows.PurgeLoop
		args = []interface{}{loopParams}

	case "restore-workflow":
		wfID = fmt.Sprintf("restore-workflow-%s", ts)
		workflow = workflows.RestoreWorkflow
		args = nil

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported workflow type: %s", req.WorkflowType)})
		return
	}

	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{
		ID:        wfID,
		TaskQueue: cfg.WorkerQueue,
	}, workflow, args...)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start workflow: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusAccepted, launchWorkflowResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "started",
		URL:        buildTemporalUILink(cfg.Namespace, we.GetID(), we.GetRunID()),
		Steps:      meta.Steps,
	})
}

// ListActiveWorkflows lists currently running workflows
// @Summary List active workflows
// @Description Returns all currently running Temporal workflow executions
// @Tags Workflows
// @Produce json
// @Success 200 {object} activeWorkflowsResponse
// @Failure 500 {object} map[string]string
// @Router /workflows/active [get]
func (c *WorkflowController) ListActiveWorkflows(ctx *gin.Context) {
	cfg := temporal.NewClientConfigFromEnv("")

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	namespace := strings.TrimSpace(os.Getenv("TEMPORAL_NAMESPACE"))
	if namespace == "" {
		namespace = "default"
	}

	resp, err := cli.WorkflowService().ListOpenWorkflowExecutions(ctx.Request.Context(), &workflowservice.ListOpenWorkflowExecutionsRequest{
		Namespace:       namespace,
		MaximumPageSize: 100,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list workflows: " + err.Error()})
		return
	}

	items := make([]activeWorkflowItem, 0, len(resp.GetExecutions()))
	for _, exec := range resp.GetExecutions() {
		info := exec.GetExecution()
		wfType := deriveWorkflowType(info.GetWorkflowId())

		startTime := ""
		if exec.GetStartTime() != nil {
			startTime = exec.GetStartTime().AsTime().Format(time.RFC3339)
		}

		item := activeWorkflowItem{
			WorkflowID:   info.GetWorkflowId(),
			RunID:        info.GetRunId(),
			WorkflowType: wfType,
			Status:       normalizeStatus(exec.GetStatus().String()),
			StartTime:    startTime,
			URL:          buildTemporalUILink(namespace, info.GetWorkflowId(), info.GetRunId()),
		}

		items = append(items, item)
	}

	ctx.JSON(http.StatusOK, activeWorkflowsResponse{
		Items: items,
		Count: len(items),
	})
}

// GetWorkflowStatus describes a single workflow execution
// @Summary Get workflow status
// @Description Returns the current status and metadata of a specific workflow execution
// @Tags Workflows
// @Produce json
// @Param workflowId path string true "Workflow ID"
// @Success 200 {object} workflowStatusResponse
// @Failure 500 {object} map[string]string
// @Router /workflows/{workflowId}/status [get]
func (c *WorkflowController) GetWorkflowStatus(ctx *gin.Context) {
	workflowID := ctx.Param("workflowId")

	cfg := temporal.NewClientConfigFromEnv("")

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	namespace := strings.TrimSpace(os.Getenv("TEMPORAL_NAMESPACE"))
	if namespace == "" {
		namespace = "default"
	}

	resp, err := cli.DescribeWorkflowExecution(ctx.Request.Context(), workflowID, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to describe workflow: " + err.Error()})
		return
	}

	info := resp.GetWorkflowExecutionInfo()
	runID := info.GetExecution().GetRunId()

	startTime := ""
	if info.GetStartTime() != nil {
		startTime = info.GetStartTime().AsTime().Format(time.RFC3339)
	}
	closeTime := ""
	if info.GetCloseTime() != nil {
		closeTime = info.GetCloseTime().AsTime().Format(time.RFC3339)
	}

	ctx.JSON(http.StatusOK, workflowStatusResponse{
		WorkflowID: workflowID,
		RunID:      runID,
		Status:     normalizeStatus(info.GetStatus().String()),
		StartTime:  startTime,
		CloseTime:  closeTime,
		URL:        buildTemporalUILink(namespace, workflowID, runID),
	})
}

// SignalWorkflow sends a signal to a running workflow
// @Summary Signal a workflow
// @Description Sends a named signal with payload to a running workflow (used for human-in-the-loop confirmations)
// @Tags Workflows
// @Accept json
// @Produce json
// @Param workflowId path string true "Workflow ID"
// @Param request body signalWorkflowRequest true "Signal details"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /workflows/{workflowId}/signal [post]
func (c *WorkflowController) SignalWorkflow(ctx *gin.Context) {
	workflowID := ctx.Param("workflowId")

	var req signalWorkflowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "signalName is required"})
		return
	}

	cfg := temporal.NewClientConfigFromEnv("")

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	err = cli.SignalWorkflow(ctx.Request.Context(), workflowID, "", req.SignalName, req.Payload)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send signal: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status":     "signaled",
		"workflowId": workflowID,
		"signalName": req.SignalName,
	})
}

// deriveWorkflowType extracts the workflow type from a workflow ID by matching
// known prefixes from the registry.
func deriveWorkflowType(workflowID string) string {
	// Match against known workflow type keys from the registry
	for _, meta := range workflows.GetWorkflowRegistry() {
		if strings.HasPrefix(workflowID, meta.Key+"-") {
			return meta.Key
		}
	}
	return "unknown"
}

// GetWorkflowResult returns the result of a completed workflow execution
// @Summary Get workflow result
// @Description Returns the result of a completed workflow, or status if still running
// @Tags Workflows
// @Produce json
// @Param workflowId path string true "Workflow ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /workflows/{workflowId}/result [get]
func (c *WorkflowController) GetWorkflowResult(ctx *gin.Context) {
	workflowID := ctx.Param("workflowId")

	cfg := temporal.NewClientConfigFromEnv("")
	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to connect to workflow service: " + err.Error()})
		return
	}
	defer cli.Close()

	// Check if workflow is closed
	desc, err := cli.DescribeWorkflowExecution(ctx.Request.Context(), workflowID, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to describe workflow: " + err.Error()})
		return
	}

	status := normalizeStatus(desc.GetWorkflowExecutionInfo().GetStatus().String())
	if status == "RUNNING" {
		var queryResult interface{}
		queryResp, err := cli.QueryWorkflow(ctx.Request.Context(), workflowID, "", "state")
		if err == nil {
			if err = queryResp.Get(&queryResult); err == nil {
				ctx.JSON(http.StatusOK, gin.H{
					"status": "RUNNING",
					"state":  queryResult,
				})
				return
			}
		}

		queryResp, err = cli.QueryWorkflow(ctx.Request.Context(), workflowID, "", "progress")
		if err == nil {
			if err = queryResp.Get(&queryResult); err == nil {
				ctx.JSON(http.StatusOK, gin.H{
					"status": "RUNNING",
					"state":  queryResult,
				})
				return
			}
		}

		ctx.JSON(http.StatusOK, gin.H{"status": "RUNNING"})
		return
	}

	// Workflow is closed, get result
	run := cli.GetWorkflow(ctx.Request.Context(), workflowID, "")
	var result map[string]interface{}
	err = run.Get(ctx.Request.Context(), &result)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"status": status,
			"error":  err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"status": status,
		"result": result,
	})
}

// GetDownloadURL returns a presigned GET URL for downloading a file from S3 storage
// @Summary Get download URL
// @Description Returns a presigned GET URL for the specified storage key
// @Tags Workflows
// @Produce json
// @Param key query string true "Storage key"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /workflows/storage/download [get]
func (c *WorkflowController) GetDownloadURL(ctx *gin.Context) {
	key := ctx.Query("key")
	if key == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "key query parameter is required"})
		return
	}

	s3, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create storage client: " + err.Error()})
		return
	}

	url, err := s3.PresignGet(ctx.Request.Context(), key, 15*time.Minute)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate download URL: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

