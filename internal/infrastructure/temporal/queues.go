package temporal

// Task queue names — the single source of truth for all producers and consumers.
const (
	// QueueObjectLifecycle handles domain lifecycle state-machine transitions:
	// expiry, purge, restore, and registrar synchronization.
	QueueObjectLifecycle = "object-lifecycle"

	// QueueDataPipeline handles external data ingestion and transformation:
	// escrow staging/ingestion, TLD cleanup, and FX rate updates.
	QueueDataPipeline = "data-pipeline"
)
