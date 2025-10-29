package rest

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
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

func NewWorkflowController(e *gin.Engine, handler gin.HandlerFunc) *WorkflowController {
	controller := &WorkflowController{}
	grp := e.Group("/workflows", handler)
	{
		grp.POST("/registrars/sync", controller.StartRegistrarSync)
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

	cfg := temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: os.Getenv("TMPIO_QUEUE"),
	}

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	we, err := cli.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "sync-registrars-" + time.Now().Format("20060102-150405"),
		TaskQueue: cfg.WorkerQueue,
	}, workflows.SyncRegistrarsWorkflow, req.BatchSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Construct a Temporal UI link if available
	temporalUIBase := os.Getenv("TMPIO_UI_URL")
	if temporalUIBase == "" {
		// Fallback to local default where docker-compose maps Temporal UI to host 8081
		temporalUIBase = "http://localhost:8081"
	}
	workflowLink := temporalUIBase + "/namespaces/" + cfg.Namespace + "/workflows/" + we.GetID() + "/" + we.GetRunID()

	ctx.JSON(http.StatusAccepted, startWorkflowResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "started",
		URL:        workflowLink,
	})
}
