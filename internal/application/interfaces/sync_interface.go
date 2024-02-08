package interfaces

// SyncService is a service for synchronizing data from external sources and storing it in the database
// SyncService defines the SyncService interface
type SyncService interface {
	RefreshSpec5Labels() error
	RefreshIANARegistrars() error
}
