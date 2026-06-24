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

// EscrowRunItem represents a single escrow import run in the list response
// swagger:model EscrowRunItem
type EscrowRunItem struct {
	TLD         string            `json:"tld"`
	RunPrefix   string            `json:"runPrefix"`
	Date        string            `json:"date"`
	WorkflowID  string            `json:"workflowId"`
	SummaryKey  string            `json:"summaryKey,omitempty"`
	HasSummary  bool              `json:"hasSummary"`
	StagedDbKey string            `json:"stagedDbKey,omitempty"`
	StagedDbURL string            `json:"stagedDbUrl,omitempty"`
	Artifacts   map[string]string `json:"artifacts,omitempty"`
	URL         string            `json:"url"`
	// Direct, clickable links to key artifacts (when MINIO_PUBLIC_ENDPOINT is set and bucket is public)
	SummaryURL           string `json:"summaryUrl,omitempty"`
	RunReportURL         string `json:"runReportUrl,omitempty"`
	AnalysisURL          string `json:"analysisUrl,omitempty"`
	RegistrarMappingURL  string `json:"registrarMappingUrl,omitempty"`
	RegistrarMappingJSON string `json:"registrarMappingJsonUrl,omitempty"`
	SQLiteDbURL          string `json:"sqliteDbUrl,omitempty"`
	ImportEventsURL      string `json:"importEventsUrl,omitempty"`
	WorkflowStatus       string `json:"workflowStatus,omitempty"`
}

// EscrowImportListResponse is the envelope returned by ListImports
// swagger:model EscrowImportListResponse
type EscrowImportListResponse struct {
	Items []EscrowRunItem `json:"items"`
	Count int             `json:"count"`
}

func NewEscrowController(e *gin.Engine, handler gin.HandlerFunc) *EscrowController {
	controller := &EscrowController{}
	grp := e.Group("/escrow", handler)
	{
		grp.POST("/uploads/presign", controller.Presign)
		grp.POST("/uploads", controller.Upload)
		grp.POST("/uploads/multipart/init", controller.InitMultipartUpload)
		grp.POST("/uploads/multipart/presign-part", controller.PresignUploadPart)
		grp.POST("/uploads/multipart/complete", controller.CompleteMultipartUpload)
		grp.POST("/uploads/multipart/abort", controller.AbortMultipartUpload)
		grp.POST("/imports", controller.StartImport)
		grp.POST("/ingest", controller.StartIngestion)
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

	url, err := s3c.PresignPut(ctx.Request.Context(), key, 15*time.Minute)
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

	// Attempt to upload to S3/MinIO and return the object key so workflow can use it directly
	// Use path format similar to presign: escrow/YYYYMMDD/<unix>/<filename>
	s3c, s3err := storage.NewS3ClientFromEnv()
	if s3err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("s3 client error: %v", s3err)})
		return
	}
	day := time.Now().UTC().Format("20060102")
	s3Key := fmt.Sprintf("escrow/%s/%d/%s", day, time.Now().Unix(), safeName)
	if upErr := s3c.UploadFile(ctx.Request.Context(), s3Key, destPath, "application/octet-stream"); upErr != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("s3 upload failed: %v", upErr)})
		return
	}
	// Remove local temp file after successful upload
	_ = os.Remove(destPath)
	ctx.JSON(http.StatusCreated, uploadResponse{ObjectKey: s3Key, Size: size, Checksum: checksum})
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

	cfg := temporal.NewClientConfigFromEnv(temporal.QueueData)

	cli, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	wfID := fmt.Sprintf("escrow-import-%s-%s", req.TLD, time.Now().Format("20060102-150405"))

	we, err := cli.ExecuteWorkflow(ctx.Request.Context(), client.StartWorkflowOptions{
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

	temporalUIBase := os.Getenv("TEMPORAL_UI_URL")
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

// StartIngestion triggers the EscrowIngestionWorkflow for a specific staged DB
// Deprecated: This endpoint is retired. Please use the unified Escrow Import workflow (/escrow/imports or via the workflows launch API) and confirm ingestion via the ConfirmEscrowImport signal.
func (c *EscrowController) StartIngestion(ctx *gin.Context) {
	ctx.JSON(http.StatusBadRequest, gin.H{"error": "This endpoint is retired. Please use the unified Escrow Import workflow (/escrow/imports or via the workflows launch API) and confirm ingestion via the ConfirmEscrowImport signal."})
}



// ListImports returns recent escrow import runs for a given TLD by scanning S3/MinIO prefixes
func (c *EscrowController) ListImports(ctx *gin.Context) {
	// ... (Parsing params and S3 listing unchanged)
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

	// List objects under escrow/<tld>/ recursively
	prefix := fmt.Sprintf("escrow/%s/", tld)
	keys, err := s3c.ListObjectKeys(ctx.Request.Context(), prefix, true, 5000)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Initialize Temporal client for checking workflow status
	cfg := temporal.NewClientConfigFromEnv("")
	cli, err := temporal.GetTemporalClient(cfg)
	var hasCli bool
	if err == nil {
		hasCli = true
		defer cli.Close()
	}

	runs := make([]EscrowRunItem, 0, 32)
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
		if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
			base = "http://" + base
		}
		return strings.TrimRight(base, "/") + "/" + bucket + "/" + strings.TrimLeft(key, "/")
	}

	type rawRun struct {
		rp   string
		date string
		wf   string
	}
	var allRuns []rawRun
	keyMap := make(map[string]bool, len(keys))

	// Collect unique runs and build a fast lookup map for keys
	for _, k := range keys {
		keyMap[k] = true
		rp, date, wf, ok := parseRunPrefix(k)
		if !ok || seen[rp] {
			continue
		}
		seen[rp] = true
		allRuns = append(allRuns, rawRun{rp: rp, date: date, wf: wf})
	}

	// Sort newest first by date if possible (YYYYMMDD lexically sorts)
	sort.Slice(allRuns, func(i, j int) bool {
		if allRuns[i].date == allRuns[j].date {
			return allRuns[i].wf > allRuns[j].wf
		}
		return allRuns[i].date > allRuns[j].date
	})

	// Apply limit
	if len(allRuns) > limit {
		allRuns = allRuns[:limit]
	}

	// Iterate limited keys to build items
	for _, rr := range allRuns {
		rp := rr.rp
		date := rr.date
		wf := rr.wf

		// Build Temporal UI link
		ns := strings.TrimSpace(os.Getenv("TEMPORAL_NAMESPACE"))
		if ns == "" {
			ns = "default"
		}
		ui := strings.TrimSpace(os.Getenv("TEMPORAL_UI_URL"))
		if ui == "" {
			ui = "http://localhost:8081"
		}
		url := ui + "/namespaces/" + ns + "/workflows/" + wf

		sumKey := rp + "/summary.json"
		runReportKey := rp + "/run-report.json"
		hasSummary := keyMap[sumKey]
		hasRunReport := keyMap[runReportKey]

		// Discover key artifacts under this run prefix
		var analysisKey, regCsvKey, regJsonKey string
		var sqliteDbKey, stagedDbKey, importEventsKey string

		for _, kk := range keys {
			if strings.HasPrefix(kk, rp+"/") {
				lower := strings.ToLower(kk)
				if strings.HasSuffix(lower, "-analysis.json") {
					analysisKey = kk
				} else if strings.HasSuffix(lower, "-registrarmapping.csv") {
					regCsvKey = kk
				} else if strings.HasSuffix(lower, "-registrarmapping.json") || strings.HasSuffix(lower, "-registrar-map.json") {
					regJsonKey = kk
				} else if strings.HasSuffix(lower, ".db") {
					if strings.Contains(filepath.Base(lower), "staged-") {
						stagedDbKey = kk
					} else if sqliteDbKey == "" {
						sqliteDbKey = kk
					}
				} else if strings.HasSuffix(lower, "/import-events.json") {
					importEventsKey = kk
				}
			}
		}

		var wfStatus string
		if hasCli {
			desc, err := cli.DescribeWorkflowExecution(ctx.Request.Context(), wf, "")
			if err == nil {
				wfStatus = normalizeStatus(desc.GetWorkflowExecutionInfo().GetStatus().String())
			}
		}

		item := EscrowRunItem{
			TLD:            tld,
			RunPrefix:      rp,
			Date:           date,
			WorkflowID:     wf,
			WorkflowStatus: wfStatus,
			SummaryKey: func() string {
				if hasSummary {
					return sumKey
				}
				return ""
			}(),
			HasSummary:  hasSummary,
			URL:         url,
			StagedDbKey: stagedDbKey,
		}
		if hasSummary {
			item.SummaryURL = joinURL(pub, bucket, sumKey)
		}
		if hasRunReport {
			item.RunReportURL = joinURL(pub, bucket, runReportKey)
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
		if sqliteDbKey != "" {
			item.SQLiteDbURL = joinURL(pub, bucket, sqliteDbKey)
		}
		if stagedDbKey != "" {
			item.StagedDbURL = joinURL(pub, bucket, stagedDbKey)
		}
		if importEventsKey != "" {
			item.ImportEventsURL = joinURL(pub, bucket, importEventsKey)
		}

		runs = append(runs, item)
	}

	ctx.JSON(http.StatusOK, EscrowImportListResponse{Items: runs, Count: len(runs)})
}

// =============================================================================
// Multipart Upload — browser-direct large file uploads (S3-compatible)
// =============================================================================

const multipartPartSize = 100 * 1024 * 1024 // 100MB per part

type initMultipartRequest struct {
	WorkflowType string `json:"workflowType" binding:"required"`
	TLD          string `json:"tld" binding:"required"`
	Filename     string `json:"filename" binding:"required"`
}

type initMultipartResponse struct {
	UploadID string `json:"uploadId"`
	Key      string `json:"key"`
	PartSize int    `json:"partSize"`
}

type presignPartRequest struct {
	Key        string `json:"key" binding:"required"`
	UploadID   string `json:"uploadId" binding:"required"`
	PartNumber int    `json:"partNumber" binding:"required"`
}

type presignPartResponse struct {
	URL string `json:"url"`
}

type completePartInfo struct {
	PartNumber int    `json:"partNumber"`
	ETag       string `json:"etag"`
}

type completeMultipartRequest struct {
	Key      string             `json:"key" binding:"required"`
	UploadID string             `json:"uploadId" binding:"required"`
	Parts    []completePartInfo `json:"parts" binding:"required"`
}

type completeMultipartResponse struct {
	Key string `json:"key"`
}

type abortMultipartRequest struct {
	Key      string `json:"key" binding:"required"`
	UploadID string `json:"uploadId" binding:"required"`
}

// InitMultipartUpload starts a new S3 multipart upload and returns the upload ID.
// POST /escrow/uploads/multipart/init
func (c *EscrowController) InitMultipartUpload(ctx *gin.Context) {
	var req initMultipartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "tld, workflowType, and filename are required"})
		return
	}

	// Sanitize filename
	safe := strings.ReplaceAll(filepath.Base(req.Filename), " ", "_")

	// Build key: escrow/uploads/{workflowType}/{tld}/{uuid}/{filename}
	uuid := fmt.Sprintf("%d", time.Now().UnixNano())
	key := fmt.Sprintf("escrow/uploads/%s/%s/%s/%s", req.WorkflowType, req.TLD, uuid, safe)

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create storage client: " + err.Error()})
		return
	}

	uploadID, err := s3c.InitMultipartUpload(ctx.Request.Context(), key)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to init multipart upload: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, initMultipartResponse{
		UploadID: uploadID,
		Key:      key,
		PartSize: multipartPartSize,
	})
}

// PresignUploadPart returns a presigned PUT URL for a single part.
// POST /escrow/uploads/multipart/presign-part
func (c *EscrowController) PresignUploadPart(ctx *gin.Context) {
	var req presignPartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "key, uploadId, and partNumber are required"})
		return
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create storage client: " + err.Error()})
		return
	}

	url, err := s3c.PresignUploadPart(ctx.Request.Context(), req.Key, req.UploadID, req.PartNumber, 30*time.Minute)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to presign part: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, presignPartResponse{URL: url})
}

// CompleteMultipartUpload finalizes a multipart upload by assembling all parts.
// POST /escrow/uploads/multipart/complete
func (c *EscrowController) CompleteMultipartUpload(ctx *gin.Context) {
	var req completeMultipartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "key, uploadId, and parts are required"})
		return
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create storage client: " + err.Error()})
		return
	}

	parts := make([]storage.MultipartCompletePart, len(req.Parts))
	for i, p := range req.Parts {
		parts[i] = storage.MultipartCompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}

	if err := s3c.CompleteMultipartUpload(ctx.Request.Context(), req.Key, req.UploadID, parts); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to complete multipart upload: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, completeMultipartResponse{Key: req.Key})
}

// AbortMultipartUpload cancels an in-progress multipart upload and cleans up parts.
// POST /escrow/uploads/multipart/abort
func (c *EscrowController) AbortMultipartUpload(ctx *gin.Context) {
	var req abortMultipartRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "key and uploadId are required"})
		return
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create storage client: " + err.Error()})
		return
	}

	if err := s3c.AbortMultipartUpload(ctx.Request.Context(), req.Key, req.UploadID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to abort multipart upload: " + err.Error()})
		return
	}

	ctx.Status(http.StatusNoContent)
}
