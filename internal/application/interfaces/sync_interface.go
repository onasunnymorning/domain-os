package interfaces

import "context"

// SyncService is a service for synchronizing data from external sources and storing it in the database
// SyncService defines the SyncService interface
type SyncService interface {
	RefreshSpec5Labels(ctx context.Context) error
	RefreshIANARegistrars(ctx context.Context) error
	RefreshFXRates(ctx context.Context, baseCurrency string) error
}
