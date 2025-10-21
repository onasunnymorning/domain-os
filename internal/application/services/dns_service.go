package services

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
)

// DNSService provides DNS management operations
type DNSService struct {
	batchPublisher *dnsevents.BatchPublisher
}

// NewDNSService creates a new DNS service
func NewDNSService(batchPublisher *dnsevents.BatchPublisher) *DNSService {
	return &DNSService{
		batchPublisher: batchPublisher,
	}
}

// DNSQueueStatsResponse represents the response for queue stats
type DNSQueueStatsResponse struct {
	Stats          []dnsevents.QueueStats `json:"stats"`
	TotalPending   int64                  `json:"total_pending"`
	TotalPublished int64                  `json:"total_published"`
	TotalErrors    int64                  `json:"total_errors"`
}

// DNSQueueItemResponse represents a queued DNS change
type DNSQueueItemResponse struct {
	ID              int64      `json:"id"`
	ZoneName        string     `json:"zone_name"`
	ChangeType      string     `json:"change_type"`
	RecordType      string     `json:"record_type"`
	RecordName      string     `json:"record_name"`
	RecordData      string     `json:"record_data"`
	TTL             uint32     `json:"ttl"`
	SourceOperation string     `json:"source_operation"`
	DomainName      string     `json:"domain_name"`
	QueuedAt        time.Time  `json:"queued_at"`
	ErrorCount      *int       `json:"error_count,omitempty"`
	LastError       *string    `json:"last_error,omitempty"`
	LastErrorAt     *time.Time `json:"last_error_at,omitempty"`
}

// DNSPendingChangesResponse represents the response for pending changes
type DNSPendingChangesResponse struct {
	Changes []DNSQueueItemResponse `json:"changes"`
	Total   int64                  `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
}

// DNSErroredChangesResponse represents the response for errored changes
type DNSErroredChangesResponse struct {
	Errors []DNSQueueItemResponse `json:"errors"`
	Total  int64                  `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

// DNSHealthResponse represents the DNS system health status
type DNSHealthResponse struct {
	Status  string                 `json:"status"` // healthy, degraded, unhealthy
	Checks  map[string]bool        `json:"checks"`
	Metrics map[string]interface{} `json:"metrics"`
	Issues  []string               `json:"issues"`
}

// GetQueueStats returns statistics about the DNS queue
func (s *DNSService) GetQueueStats(ctx context.Context) (*DNSQueueStatsResponse, error) {
	if s.batchPublisher == nil {
		return nil, fmt.Errorf("DNS batch publisher not configured")
	}

	stats, err := s.batchPublisher.GetQueueStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	// Calculate totals
	var totalPending, totalPublished, totalErrors int64
	for _, stat := range stats {
		totalPending += stat.PendingCount
		totalPublished += stat.PublishedCount
		totalErrors += stat.ErrorCount
	}

	return &DNSQueueStatsResponse{
		Stats:          stats,
		TotalPending:   totalPending,
		TotalPublished: totalPublished,
		TotalErrors:    totalErrors,
	}, nil
}

// GetQueueStatsForZone returns statistics for a specific zone
func (s *DNSService) GetQueueStatsForZone(ctx context.Context, zone string) (*dnsevents.QueueStats, error) {
	if s.batchPublisher == nil {
		return nil, fmt.Errorf("DNS batch publisher not configured")
	}

	stats, err := s.batchPublisher.GetQueueStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	// Find the specific zone
	for _, stat := range stats {
		if stat.ZoneName == zone {
			return &stat, nil
		}
	}

	// Return empty stats if zone not found
	return &dnsevents.QueueStats{
		ZoneName:       zone,
		PendingCount:   0,
		PublishedCount: 0,
		ErrorCount:     0,
	}, nil
}

// GetPendingChanges returns pending DNS changes with filtering
func (s *DNSService) GetPendingChanges(ctx context.Context, zone, domain, changeType, recordType string, limit, offset int) (*DNSPendingChangesResponse, error) {
	if s.batchPublisher == nil {
		return nil, fmt.Errorf("DNS batch publisher not configured")
	}

	// Build query with filters
	query := `
		SELECT 
			id, zone_name, change_type, record_type, record_name, 
			record_data, ttl, source_operation, domain_name, queued_at
		FROM dns_change_queue
		WHERE published_at IS NULL
	`
	args := []interface{}{}

	if zone != "" {
		query += " AND zone_name = ?"
		args = append(args, zone)
	}
	if domain != "" {
		query += " AND domain_name = ?"
		args = append(args, domain)
	}
	if changeType != "" {
		query += " AND change_type = ?"
		args = append(args, changeType)
	}
	if recordType != "" {
		query += " AND record_type = ?"
		args = append(args, recordType)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int64
	err := s.batchPublisher.GetDB().WithContext(ctx).Raw(countQuery, args...).Scan(&total).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count pending changes: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY queued_at ASC, id ASC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var changes []DNSQueueItemResponse
	err = s.batchPublisher.GetDB().WithContext(ctx).Raw(query, args...).Scan(&changes).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get pending changes: %w", err)
	}

	return &DNSPendingChangesResponse{
		Changes: changes,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// GetErroredChanges returns DNS changes that have errors
func (s *DNSService) GetErroredChanges(ctx context.Context, zone string, limit, offset int) (*DNSErroredChangesResponse, error) {
	if s.batchPublisher == nil {
		return nil, fmt.Errorf("DNS batch publisher not configured")
	}

	query := `
		SELECT 
			id, zone_name, change_type, record_type, record_name, 
			record_data, ttl, source_operation, domain_name, queued_at,
			error_count, last_error, last_error_at
		FROM dns_change_queue
		WHERE published_at IS NULL AND error_count > 0
	`
	args := []interface{}{}

	if zone != "" {
		query += " AND zone_name = ?"
		args = append(args, zone)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM (" + query + ") AS filtered"
	var total int64
	err := s.batchPublisher.GetDB().WithContext(ctx).Raw(countQuery, args...).Scan(&total).Error
	if err != nil {
		return nil, fmt.Errorf("failed to count errored changes: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY last_error_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	var errors []DNSQueueItemResponse
	err = s.batchPublisher.GetDB().WithContext(ctx).Raw(query, args...).Scan(&errors).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get errored changes: %w", err)
	}

	return &DNSErroredChangesResponse{
		Errors: errors,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

// GetHealth returns the health status of the DNS system
func (s *DNSService) GetHealth(ctx context.Context) (*DNSHealthResponse, error) {
	if s.batchPublisher == nil {
		return &DNSHealthResponse{
			Status: "unavailable",
			Checks: map[string]bool{
				"publisher_configured": false,
			},
			Metrics: map[string]interface{}{},
			Issues:  []string{"DNS batch publisher not configured"},
		}, nil
	}

	checks := make(map[string]bool)
	metrics := make(map[string]interface{})
	issues := []string{}

	// Check if worker is running (we'll assume it is if publisher exists)
	checks["worker_running"] = true

	// Check database connection
	db := s.batchPublisher.GetDB()
	sqlDB, err := db.DB()
	if err != nil {
		checks["database_connected"] = false
		issues = append(issues, fmt.Sprintf("Database error: %v", err))
	} else {
		err = sqlDB.Ping()
		checks["database_connected"] = err == nil
		if err != nil {
			issues = append(issues, fmt.Sprintf("Database ping failed: %v", err))
		}
	}

	// Get queue metrics
	stats, err := s.batchPublisher.GetQueueStats(ctx)
	if err != nil {
		checks["queue_accessible"] = false
		issues = append(issues, fmt.Sprintf("Failed to get queue stats: %v", err))
	} else {
		checks["queue_accessible"] = true

		var totalPending, totalErrors int64
		var oldestPendingSeconds float64

		for _, stat := range stats {
			totalPending += stat.PendingCount
			totalErrors += stat.ErrorCount

			if stat.OldestPending != nil {
				age := time.Since(*stat.OldestPending).Seconds()
				if age > oldestPendingSeconds {
					oldestPendingSeconds = age
				}
			}
		}

		metrics["pending_count"] = totalPending
		metrics["error_count"] = totalErrors
		metrics["oldest_pending_seconds"] = oldestPendingSeconds

		// Check if queue is stuck (oldest item > 5 minutes)
		checks["queue_not_stuck"] = oldestPendingSeconds < 300
		if oldestPendingSeconds >= 300 {
			issues = append(issues, fmt.Sprintf("Queue appears stuck: oldest item is %.0f seconds old", oldestPendingSeconds))
		}

		// Check error rate (errors should be < 5% of total)
		var errorRatePercent float64
		if totalPending+totalErrors > 0 {
			errorRatePercent = float64(totalErrors) / float64(totalPending+totalErrors) * 100
		}
		metrics["error_rate_percent"] = errorRatePercent
		checks["error_rate_ok"] = errorRatePercent < 5.0
		if errorRatePercent >= 5.0 {
			issues = append(issues, fmt.Sprintf("High error rate: %.1f%%", errorRatePercent))
		}
	}

	// Determine overall status
	status := "healthy"
	if len(issues) > 0 {
		status = "degraded"
	}
	if !checks["database_connected"] || !checks["worker_running"] {
		status = "unhealthy"
	}

	return &DNSHealthResponse{
		Status:  status,
		Checks:  checks,
		Metrics: metrics,
		Issues:  issues,
	}, nil
}
