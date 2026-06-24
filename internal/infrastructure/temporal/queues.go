package temporal

// Task queue names — the single source of truth for all producers and consumers.
const (
	// QueueLifecycle handles domain lifecycle state-machine transitions:
	// expiry, purge, restore, and registrar synchronization.
	QueueLifecycle = "object-lifecycle"

	// QueueData handles external data ingestion and transformation:
	// escrow staging/ingestion, TLD cleanup, and FX rate updates.
	QueueData = "data-pipeline"
)
