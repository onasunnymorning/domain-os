package response

// BulkCreateRegistrarsResult reports the outcome of a registrar bulk create.
//
// The bulk insert skips rows that collide on a unique constraint (a name that is
// already taken, for instance) instead of failing the batch, so a 201 alone does
// not say what was created. Callers must read Created and Skipped.
type BulkCreateRegistrarsResult struct {
	// Created is the ClIDs that were inserted
	Created []string
	// Skipped is the ClIDs that were requested but not inserted
	Skipped []string
}
