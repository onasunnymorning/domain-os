package storage

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EventArchiveReader reads archived domain events from S3.
// Events are stored as gzip-compressed JSONL files with day-partitioned keys:
// events/archive/{year}/{month}/{day}/events-{timestamp}-{count}.jsonl.gz
type EventArchiveReader struct {
	s3 *S3Client
}

// NewEventArchiveReader creates a new archive reader using the given S3 client.
func NewEventArchiveReader(s3 *S3Client) *EventArchiveReader {
	return &EventArchiveReader{s3: s3}
}

// SearchArchive reads archived events from S3 for the specified date range,
// applying the given filters in-memory. Results are returned in reverse
// chronological order, capped at filter.Limit.
//
// This function downloads and decompresses JSONL.gz files for each day in the
// range. At typical volumes (~500 events/file, ~30 files/month), a one-month
// query scans ~15K events in memory — acceptable for operator tooling.
func (r *EventArchiveReader) SearchArchive(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	// Determine date range to scan
	start, end := r.resolveDateRange(filter)

	// List all archive keys in the date range
	keys, err := r.listKeysForRange(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("SearchArchive: failed to list archive keys for range %s to %s: %w", start.Format("2006-01-02"), end.Format("2006-01-02"), err)
	}

	if len(keys) == 0 {
		return &entities.EventSearchResult{
			Events:     []entities.DomainEvent{},
			TotalCount: 0,
			Tier:       "warm",
		}, nil
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	// Process files in reverse order (newest first) to get most recent events first.
	// Stop once we have enough matches.
	var matched []entities.DomainEvent
	var totalMatched int64

	for i := len(keys) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		events, err := r.readJSONLGz(ctx, keys[i])
		if err != nil {
			return nil, fmt.Errorf("SearchArchive: failed to read archive file %s: %w", keys[i], err)
		}

		for j := len(events) - 1; j >= 0; j-- {
			evt := events[j]
			if r.matchesFilter(evt, filter) {
				totalMatched++
				if len(matched) < limit {
					matched = append(matched, evt)
				}
			}
		}
	}

	return &entities.EventSearchResult{
		Events:     matched,
		TotalCount: totalMatched,
		Tier:       "warm",
	}, nil
}

// resolveDateRange determines the start and end dates for scanning archive files.
// Falls back to scanning the last 365 days if no date range is specified.
func (r *EventArchiveReader) resolveDateRange(filter entities.EventSearchFilter) (time.Time, time.Time) {
	now := time.Now().UTC()

	end := now
	if filter.Before != nil {
		end = *filter.Before
	}

	start := now.AddDate(-1, 0, 0) // Default: scan last 1 year
	if filter.After != nil {
		start = *filter.After
	}

	return start, end
}

// listKeysForRange lists all archive keys between start and end dates.
// Keys are organized as: events/archive/{year}/{month}/{day}/
func (r *EventArchiveReader) listKeysForRange(ctx context.Context, start, end time.Time) ([]string, error) {
	var allKeys []string

	// Iterate day by day
	current := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	for !current.After(endDay) {
		prefix := fmt.Sprintf("events/archive/%d/%02d/%02d/", current.Year(), current.Month(), current.Day())
		keys, err := r.s3.ListObjectKeys(ctx, prefix, true, 0)
		if err != nil {
			return nil, fmt.Errorf("listKeysForRange(prefix=%s): %w", prefix, err)
		}
		allKeys = append(allKeys, keys...)
		current = current.AddDate(0, 0, 1)
	}

	return allKeys, nil
}

// readJSONLGz downloads and decompresses a gzip-compressed JSONL file from S3,
// returning the parsed events.
func (r *EventArchiveReader) readJSONLGz(ctx context.Context, key string) ([]entities.DomainEvent, error) {
	stream, err := r.s3.DownloadStream(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("readJSONLGz(key=%s): download failed: %w", key, err)
	}
	defer stream.Close()

	gz, err := gzip.NewReader(stream)
	if err != nil {
		return nil, fmt.Errorf("readJSONLGz(key=%s): gzip init failed: %w", key, err)
	}
	defer gz.Close()

	var events []entities.DomainEvent
	scanner := bufio.NewScanner(gz)
	// Increase buffer size for large event payloads (1MB max per line)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var evt entities.DomainEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			// Log but skip malformed lines — don't fail the whole query
			continue
		}
		events = append(events, evt)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("readJSONLGz(key=%s): scanner error at line %d: %w", key, lineNum, err)
	}

	return events, nil
}

// matchesFilter checks if an event matches all non-empty filter criteria.
func (r *EventArchiveReader) matchesFilter(evt entities.DomainEvent, filter entities.EventSearchFilter) bool {
	if filter.Subject != "" && evt.Subject != filter.Subject {
		return false
	}

	if filter.Type != "" {
		if strings.HasSuffix(filter.Type, "*") {
			prefix := strings.TrimSuffix(filter.Type, "*")
			if !strings.HasPrefix(evt.Type, prefix) {
				return false
			}
		} else if evt.Type != filter.Type {
			return false
		}
	}

	if filter.Source != "" && evt.Source != filter.Source {
		return false
	}

	if filter.Actor != "" && evt.Actor != filter.Actor {
		return false
	}

	if filter.RoID != "" && evt.RoID != filter.RoID {
		return false
	}

	if filter.After != nil && evt.Time.Before(*filter.After) {
		return false
	}

	if filter.Before != nil && !evt.Time.Before(*filter.Before) {
		return false
	}

	return true
}
