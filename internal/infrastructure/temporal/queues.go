package temporal

// Task queue names — the single source of truth for all producers and consumers.
const (
	// Production queues — classified by workload profile.
	QueueFastOps    = "fast-ops"       // Low-latency, sub-minute activities
	QueueScheduled  = "scheduled"      // Periodic background work
	QueueHeavyBatch = "heavy-batch"    // Multi-hour, resource-intensive
	QueueLifecycle  = "lifecycle"      // Domain state-machine transitions

	// Deprecated — kept for in-flight workflow transition.
	// Remove once all in-flight workflows on these queues have completed.
	QueueData                = "data-pipeline"     // DEPRECATED: use QueueScheduled, QueueFastOps, or QueueHeavyBatch
	QueueLifecycleDeprecated = "object-lifecycle"  // DEPRECATED: use QueueLifecycle
)
