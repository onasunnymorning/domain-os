package postgres

// DNSQueueSchemaMigration creates the DNS change queue table for batching
// This allows DNS events to be queued in the database and processed in batches
const DNSQueueSchemaMigration = `
-- ========================================
-- DNS Change Queue (for time-based batching)
-- ========================================
CREATE TABLE IF NOT EXISTS dns_change_queue (
    id BIGSERIAL PRIMARY KEY,
    zone_name VARCHAR(255) NOT NULL,
    
    -- DNS Change Details
    change_type VARCHAR(10) NOT NULL,  -- 'ADD' or 'DELETE'
    record_type VARCHAR(10) NOT NULL,  -- 'NS', 'A', 'AAAA', etc.
    record_name VARCHAR(255) NOT NULL,
    record_data TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 3600,
    
    -- Traceability
    source_operation VARCHAR(50),
    domain_name VARCHAR(255),
    
    -- Queue Metadata
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    batch_id BIGINT,
    
    -- Error tracking
    error_count INTEGER DEFAULT 0,
    last_error TEXT,
    last_error_at TIMESTAMPTZ,
    
    -- Indexes for efficient queue processing
    CONSTRAINT check_change_type CHECK (change_type IN ('ADD', 'DELETE')),
    CONSTRAINT check_record_type CHECK (record_type IN ('NS', 'A', 'AAAA', 'CNAME', 'TXT', 'MX'))
);

-- Index for pending items (most common query)
CREATE INDEX IF NOT EXISTS idx_queue_pending 
    ON dns_change_queue(zone_name, queued_at) 
    WHERE published_at IS NULL;

-- Index for published items (for cleanup queries)
CREATE INDEX IF NOT EXISTS idx_queue_published 
    ON dns_change_queue(published_at) 
    WHERE published_at IS NOT NULL;

-- Index for error tracking
CREATE INDEX IF NOT EXISTS idx_queue_errors 
    ON dns_change_queue(error_count) 
    WHERE error_count > 0;

-- Index for batch processing
CREATE INDEX IF NOT EXISTS idx_queue_batch 
    ON dns_change_queue(batch_id) 
    WHERE batch_id IS NOT NULL;

-- ========================================
-- Queue Cleanup Function
-- ========================================

-- Delete published queue items older than retention period
CREATE OR REPLACE FUNCTION cleanup_dns_queue(p_retention_days INT DEFAULT 7)
RETURNS BIGINT AS $$
DECLARE
    v_deleted_count BIGINT;
BEGIN
    DELETE FROM dns_change_queue
    WHERE published_at IS NOT NULL
    AND published_at < NOW() - (p_retention_days || ' days')::INTERVAL;
    
    GET DIAGNOSTICS v_deleted_count = ROW_COUNT;
    
    RETURN v_deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ========================================
-- Queue Statistics View
-- ========================================

CREATE OR REPLACE VIEW dns_queue_stats AS
SELECT 
    zone_name,
    COUNT(*) FILTER (WHERE published_at IS NULL) as pending_count,
    COUNT(*) FILTER (WHERE published_at IS NOT NULL) as published_count,
    COUNT(*) FILTER (WHERE error_count > 0) as error_count,
    MIN(queued_at) FILTER (WHERE published_at IS NULL) as oldest_pending,
    MAX(queued_at) FILTER (WHERE published_at IS NULL) as newest_pending,
    AVG(EXTRACT(EPOCH FROM (published_at - queued_at))) FILTER (WHERE published_at IS NOT NULL) as avg_wait_seconds
FROM dns_change_queue
GROUP BY zone_name;
`
