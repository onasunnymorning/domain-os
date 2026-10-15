package services

// SetBatchSize makes the cursor-paged phases commit every n rows, so a test can
// cross batch boundaries with a handful of staged rows.
func (s *DirectDBImporter) SetBatchSize(n int) { s.batchSize = n }
