package rest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/client"
)

// EscrowController provides endpoints to upload escrow files and trigger imports
type EscrowController struct{}

type uploadResponse struct {
	ObjectKey string `json:"objectKey"`
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum"`
}

type presignResponse struct {
	ObjectKey string `json:"objectKey"`
	Url       string `json:"url"`
	Method    string `json:"method"`
	ExpiresIn int64  `json:"expiresIn"`
}

type startEscrowImportRequest struct {
	TLD       string                 `json:"tld"`
	ObjectKey string                 `json:"objectKey"`
	Options   map[string]interface{} `json:"options"`
}

type startEscrowImportResponse struct {
	WorkflowID string `json:"workflowId"`
	RunID      string `json:"runId"`
	Status     string `json:"status"`
	URL        string `json:"url"`
}

func NewEscrowController(e *gin.Engine, handler gin.HandlerFunc) *EscrowController {
	controller := &EscrowController{}
	grp := e.Group("/escrow", handler)
	{
		grp.POST("/uploads/presign", controller.Presign)
		grp.POST("/uploads", controller.Upload)
		grp.POST("/imports", controller.StartImport)
		grp.GET("/imports", controller.ListImports)
	}
	return controller
}

// Presign returns a presigned PUT URL for direct browser upload to S3/MinIO
func (c *EscrowController) Presign(ctx *gin.Context) {
	// Expect ?filename= and optional ?tld=
	filename := strings.TrimSpace(ctx.Query("filename"))
	if filename == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "filename query parameter is required"})
		return
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Construct a key: escrow/YYYYMMDD/<timestamp>/<sanitized-filename>
	ts := time.Now().UTC().Format("20060102")
	safe := strings.ReplaceAll(filename, " ", "_")
	key := fmt.Sprintf("escrow/%s/%d/%s", ts, time.Now().Unix(), safe)

	url, err := s3c.PresignPut(ctx, key, 15*time.Minute)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, presignResponse{ObjectKey: key, Url: url, Method: "PUT", ExpiresIn: 900})
}

// Upload streams a multipart file to local storage (dev fallback until S3 is wired)
func (c *EscrowController) Upload(ctx *gin.Context) {
	// Ensure low memory usage for large files; remaining goes to disk temp files
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, 1<<40) // 1TB upper guard
	// Get the uploaded file
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file form field 'file' is required"})
		return
	}
	defer file.Close()

	// Determine destination directory
	baseDir := strings.TrimSpace(os.Getenv("ESCROW_UPLOAD_DIR"))
	if baseDir == "" {
		baseDir = "/tmp/escrow-uploads"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create upload dir: %v", err)})
		return
	}

	// Create destination filename
	ts := time.Now().UTC().Format("20060102-150405")
	safeName := strings.ReplaceAll(header.Filename, " ", "_")
	destName := fmt.Sprintf("%s-%s", ts, safeName)
	destPath := filepath.Join(baseDir, destName)

	// Stream copy with checksum
	dst, err := os.Create(destPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create dest file: %v", err)})
		return
	}
	defer dst.Close()

	hasher := sha256.New()
	size, err := io.Copy(dst, io.TeeReader(file, hasher))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to write file: %v", err)})
		return
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// ObjectKey is the absolute path for now (to be switched to S3 key later)
	ctx.JSON(http.StatusCreated, uploadResponse{
		ObjectKey: destPath,
		Size:      size,
		Checksum:  checksum,
	})
}

// StartImport triggers the Temporal EscrowImportWorkflow
func (c *EscrowController) StartImport(ctx *gin.Context) {
	var req startEscrowImportRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.TLD) == "" || strings.TrimSpace(req.ObjectKey) == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld and objectKey are required"})
		return
	}

	cfg := temporal.TemporalClientconfig{
		HostPort:    os.Getenv("TMPIO_HOST_PORT"),
		Namespace:   os.Getenv("TMPIO_NAME_SPACE"),
		ClientKey:   os.Getenv("TMPIO_KEY"),
		ClientCert:  os.Getenv("TMPIO_CERT"),
		WorkerQueue: getEscrowQueue(),
	}

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	wfID := fmt.Sprintf("escrow-import-%s-%s", req.TLD, time.Now().Format("20060102-150405"))

	we, err := cli.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        wfID,
		TaskQueue: cfg.WorkerQueue,
	}, workflows.EscrowImportWorkflow, workflows.EscrowImportParams{
		TLD:       req.TLD,
		ObjectKey: req.ObjectKey,
		Options:   req.Options,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	temporalUIBase := os.Getenv("TMPIO_UI_URL")
	// Be defensive: strip accidental surrounding quotes from env values
	temporalUIBase = strings.Trim(temporalUIBase, "\"'")
	if temporalUIBase == "" {
		temporalUIBase = "http://localhost:8081"
	}
	link := temporalUIBase + "/namespaces/" + cfg.Namespace + "/workflows/" + we.GetID() + "/" + we.GetRunID()

	ctx.JSON(http.StatusAccepted, startEscrowImportResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "started",
		URL:        link,
	})
}

func getEscrowQueue() string {
	q := strings.TrimSpace(os.Getenv("ESCROW_QUEUE"))
	if q == "" {
		return "escrow-import"
	}
	return q
}

// ListImports returns recent escrow import runs for a given TLD by scanning S3/MinIO prefixes
// GET /escrow/imports?tld=<tld>&limit=20
func (c *EscrowController) ListImports(ctx *gin.Context) {
	tld := strings.TrimSpace(ctx.Query("tld"))
	if tld == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld query parameter is required"})
		return
	}
	limit := 20
	if v := strings.TrimSpace(ctx.Query("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// List objects under escrow/<tld>/ recursively and derive run prefixes: escrow/<tld>/<yyyyMMdd>/<workflowId>
	prefix := fmt.Sprintf("escrow/%s/", tld)
	keys, err := s3c.ListObjectKeys(ctx, prefix, true, 5000)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type runItem struct {
		TLD        string            `json:"tld"`
		RunPrefix  string            `json:"runPrefix"`
		Date       string            `json:"date"`
		WorkflowID string            `json:"workflowId"`
		SummaryKey string            `json:"summaryKey,omitempty"`
		HasSummary bool              `json:"hasSummary"`
		Artifacts  map[string]string `json:"artifacts,omitempty"`
		URL        string            `json:"url"`
		// Direct, clickable links to key artifacts (when MINIO_PUBLIC_ENDPOINT is set and bucket is public)
		SummaryURL            string `json:"summaryUrl,omitempty"`
		AnalysisURL           string `json:"analysisUrl,omitempty"`
		RegistrarMappingURL   string `json:"registrarMappingUrl,omitempty"`
		RegistrarMappingJSON  string `json:"registrarMappingJsonUrl,omitempty"`
	}
	runs := make([]runItem, 0, 32)
	seen := map[string]bool{}

	// Helper to extract run prefix
	parseRunPrefix := func(key string) (string, string, string, bool) {
		parts := strings.Split(key, "/")
		if len(parts) < 4 {
			return "", "", "", false
		}
		if parts[0] != "escrow" || parts[1] != tld {
			return "", "", "", false
		}
		date := parts[2]
		wf := parts[3]
		rp := strings.Join(parts[:4], "/")
		return rp, date, wf, true
	}

	// Build helpers for public URLs
	bucket := strings.TrimSpace(os.Getenv("ESCROW_BUCKET"))
	if bucket == "" {
		bucket = "escrow"
	}
	pub := strings.Trim(strings.TrimSpace(os.Getenv("MINIO_PUBLIC_ENDPOINT")), "\"'")
	if pub == "" {
		pub = "http://localhost:9000"
	}
	joinURL := func(base, bucket, key string) string {
		// Expect base like http://localhost:9000
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			base = "http://" + base
		}
		// Avoid duplicate slashes
		return strings.TrimRight(base, "/") + "/" + bucket + "/" + strings.TrimLeft(key, "/")
	}

	// Iterate keys to collect unique runs
	for _, k := range keys {
		rp, date, wf, ok := parseRunPrefix(k)
		if !ok {
			continue
		}
		if seen[rp] {
			continue
		}
		seen[rp] = true

		// Build Temporal UI link
		ns := strings.TrimSpace(os.Getenv("TMPIO_NAME_SPACE"))
		if ns == "" {
			ns = "default"
		}
		ui := strings.TrimSpace(os.Getenv("TMPIO_UI_URL"))
		if ui == "" {
			ui = "http://localhost:8081"
		}
		url := ui + "/namespaces/" + ns + "/workflows/" + wf

		sumKey := rp + "/summary.json"
		hasSummary, _ := s3c.Exists(ctx, sumKey)

		// Discover key artifacts under this run prefix
		var analysisKey, regCsvKey, regJsonKey string
		for _, kk := range keys {
			if strings.HasPrefix(kk, rp+"/") {
				lower := strings.ToLower(kk)
				if strings.HasSuffix(lower, "-analysis.json") {
					analysisKey = kk
				} else if strings.HasSuffix(lower, "-registrarmapping.csv") {
					regCsvKey = kk
				} else if strings.HasSuffix(lower, "-registrarmapping.json") || strings.HasSuffix(lower, "-registrar-map.json") {
					regJsonKey = kk
				}
			}
		}

		item := runItem{
			TLD:        tld,
			RunPrefix:  rp,
			Date:       date,
			WorkflowID: wf,
			SummaryKey: func() string {
				if hasSummary {
					return sumKey
				}
				return ""
			}(),
			HasSummary: hasSummary,
			URL:        url,
		}
		// Attach public URLs if we can build them
		if hasSummary {
			item.SummaryURL = joinURL(pub, bucket, sumKey)
		}
		if analysisKey != "" {
			item.AnalysisURL = joinURL(pub, bucket, analysisKey)
		}
		if regCsvKey != "" {
			item.RegistrarMappingURL = joinURL(pub, bucket, regCsvKey)
		}
		if regJsonKey != "" {
			item.RegistrarMappingJSON = joinURL(pub, bucket, regJsonKey)
		}

		runs = append(runs, item)
		if len(runs) >= limit {
			break
		}
	}

	// Sort newest first by date if possible (YYYYMMDD lexically sorts)
	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Date == runs[j].Date {
			return runs[i].WorkflowID > runs[j].WorkflowID
		}
		return runs[i].Date > runs[j].Date
	})

	ctx.JSON(http.StatusOK, gin.H{"items": runs, "count": len(runs)})
}
