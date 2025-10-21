package postgres

// DNSZoneSchemaMigration creates tables for DNS zone management
// NO TRIGGERS - all events are published from application code
const DNSZoneSchemaMigration = `
-- ========================================
-- DNS Zone Serial Tracking
-- ========================================
CREATE TABLE IF NOT EXISTS dns_zone_serials (
    zone_name VARCHAR(255) PRIMARY KEY,
    serial BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    last_notify_at TIMESTAMPTZ,
    notify_count INTEGER DEFAULT 0,
    
    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Initialize serials for existing DNS-enabled TLDs (if tlds table exists)
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'tlds') THEN
        INSERT INTO dns_zone_serials (zone_name, serial)
        SELECT name, EXTRACT(EPOCH FROM NOW())::BIGINT
        FROM tlds 
        WHERE enable_dns = true
        ON CONFLICT (zone_name) DO NOTHING;
    END IF;
END $$;

-- ========================================
-- DNS Zone Journal (IXFR Support)
-- ========================================
CREATE TABLE IF NOT EXISTS dns_zone_journal (
    id BIGSERIAL,
    zone_name VARCHAR(255) NOT NULL,
    serial BIGINT NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- DNS Change Details
    change_type VARCHAR(10) NOT NULL, -- 'ADD' or 'DELETE'
    record_type VARCHAR(10) NOT NULL, -- 'NS', 'A', 'AAAA', etc.
    record_name VARCHAR(255) NOT NULL,
    record_data TEXT NOT NULL,
    ttl INTEGER NOT NULL DEFAULT 3600,
    
    -- Traceability (for debugging)
    source_operation VARCHAR(50), -- 'CreateDomain', 'AddHost', etc.
    domain_name VARCHAR(255),      -- Which domain triggered this
    
    PRIMARY KEY (zone_name, timestamp, id)
);

-- Indexes for efficient IXFR queries
CREATE INDEX IF NOT EXISTS idx_journal_zone_serial 
    ON dns_zone_journal(zone_name, serial);
CREATE INDEX IF NOT EXISTS idx_journal_timestamp 
    ON dns_zone_journal(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_journal_domain 
    ON dns_zone_journal(domain_name) 
    WHERE domain_name IS NOT NULL;

-- ========================================
-- Helper Functions (Application Callable)
-- ========================================

-- Function to get next serial for a zone
CREATE OR REPLACE FUNCTION get_next_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_current_serial BIGINT;
    v_new_serial BIGINT;
    v_today BIGINT;
    v_sequence INT;
BEGIN
    -- Lock and get current serial
    SELECT serial INTO v_current_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name
    FOR UPDATE;
    
    -- If no serial exists, initialize with today
    IF v_current_serial IS NULL THEN
        v_new_serial := (TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100) + 1;
        
        INSERT INTO dns_zone_serials (zone_name, serial)
        VALUES (p_zone_name, v_new_serial)
        ON CONFLICT (zone_name) DO UPDATE
        SET serial = v_new_serial;
        
        RETURN v_new_serial;
    END IF;
    
    -- Calculate new serial (YYYYMMDDnn format)
    v_today := TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100;
    
    IF v_current_serial >= v_today AND v_current_serial < v_today + 100 THEN
        -- Same day, increment sequence
        v_new_serial := v_current_serial + 1;
        
        -- Check for overflow (max 99 per day)
        IF v_new_serial >= v_today + 100 THEN
            -- Fallback to Unix timestamp
            v_new_serial := EXTRACT(EPOCH FROM NOW())::BIGINT;
        END IF;
    ELSE
        -- New day, start at 01
        v_new_serial := v_today + 1;
    END IF;
    
    -- Update serial
    UPDATE dns_zone_serials
    SET serial = v_new_serial,
        updated_at = NOW()
    WHERE zone_name = p_zone_name;
    
    RETURN v_new_serial;
END;
$$ LANGUAGE plpgsql;

-- Function to get current serial (without incrementing)
CREATE OR REPLACE FUNCTION get_current_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_serial BIGINT;
BEGIN
    SELECT serial INTO v_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name;
    
    RETURN COALESCE(v_serial, 0);
END;
$$ LANGUAGE plpgsql;

-- ========================================
-- Journal Cleanup (Manual or Cron)
-- ========================================

-- Keep only last N serials per zone
CREATE OR REPLACE FUNCTION cleanup_dns_journal(p_keep_count INT DEFAULT 100)
RETURNS TABLE(zone_name VARCHAR, deleted_count BIGINT) AS $$
BEGIN
    RETURN QUERY
    WITH deleted AS (
        DELETE FROM dns_zone_journal
        WHERE id IN (
            SELECT id FROM (
                SELECT id, zone_name,
                       ROW_NUMBER() OVER (
                           PARTITION BY zone_name 
                           ORDER BY serial DESC
                       ) as rn
                FROM dns_zone_journal
            ) t
            WHERE rn > p_keep_count
        )
        RETURNING zone_name
    )
    SELECT d.zone_name, COUNT(*)::BIGINT as deleted_count
    FROM deleted d
    GROUP BY d.zone_name;
END;
$$ LANGUAGE plpgsql;

-- ========================================
-- Utility Views
-- ========================================

-- View to see latest changes per zone
CREATE OR REPLACE VIEW dns_zone_latest_changes AS
SELECT 
    zone_name,
    serial,
    COUNT(*) as change_count,
    MAX(timestamp) as latest_change,
    ARRAY_AGG(DISTINCT change_type) as change_types,
    ARRAY_AGG(DISTINCT record_type) as record_types
FROM dns_zone_journal
GROUP BY zone_name, serial
ORDER BY zone_name, serial DESC;

-- View to see current zone serials
CREATE OR REPLACE VIEW dns_zone_status AS
SELECT 
    zs.zone_name,
    zs.serial as current_serial,
    zs.updated_at as last_updated,
    zs.last_notify_at,
    zs.notify_count,
    COALESCE(j.recent_changes, 0) as changes_in_last_hour
FROM dns_zone_serials zs
LEFT JOIN (
    SELECT zone_name, COUNT(*) as recent_changes
    FROM dns_zone_journal
    WHERE timestamp > NOW() - INTERVAL '1 hour'
    GROUP BY zone_name
) j ON j.zone_name = zs.zone_name
ORDER BY zs.zone_name;

COMMENT ON TABLE dns_zone_serials IS 'Tracks current serial number for each DNS zone';
COMMENT ON TABLE dns_zone_journal IS 'Change log for DNS zone updates (enables IXFR)';
COMMENT ON FUNCTION get_next_serial IS 'Get and increment serial for a zone (call within transaction)';
COMMENT ON FUNCTION get_current_serial IS 'Get current serial without incrementing';
COMMENT ON FUNCTION cleanup_dns_journal IS 'Remove old journal entries, keeping last N serials per zone';
`
