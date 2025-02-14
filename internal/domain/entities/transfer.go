package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// TransferStatus represents the current stage of a domain transfer.
type TransferStatus string

const (

	// TransferStatusPending means the transfer is pending approval from the losing registrar.
	TransferStatusPending TransferStatus = "pending"

	// TransferStatusApproved means the losing registrar has approved or the transfer
	// auto-approved after the policy’s waiting period.
	TransferStatusApproved TransferStatus = "approved"

	// TransferStatusDenied means the transfer request was explicitly rejected.
	TransferStatusDenied TransferStatus = "denied"
)

var (
	// ErrTransferComplete is returned when trying to update a transfer that is already approved or denied.
	ErrTransferComplete = errors.New("transfer is already approved or denied and cannot be updated")

	// ErrInvalidTransferStatus is returned when trying to update a transfer with an invalid status.
	ErrInvalidTransferStatus = errors.New("invalid transfer status")
)

// DomainTransfer holds information about a domain's transfer event.
type DomainTransfer struct {
	// An internal ID
	ID uuid.UUID

	// The RoID of the domain being transferred.
	DomainRoiD int64

	// The name of domain being transferred.
	DomainName DomainName

	// The registrar that is attempting to take over management of the domain.
	GainingRegistrar Registrar

	// The current or losing registrar of the domain.
	LosingRegistrar Registrar

	// TransferStatus indicates whether the transfer is pending, approved, denied or failed.
	Status TransferStatus

	// Time the transfer was requested (when <transfer> command was sent to the registry).
	RequestedAt time.Time

	// If the transfer is pending, this could be the deadline by which the losing registrar
	// must respond (configured in PhasePolicy.TransferGP).
	ExpiresAt time.Time

	// Time the transfer was last updated.
	UpdatedAt time.Time

	// Optional field to store a reason for denial or failure.
	Reason string

	// Correlation ID of the transfer request.
	CorrelationID string

	// Optional: Additional metadata or logs you might want to store.
	Notes string
}

// NewDomainTransfer creates a new DomainTransfer object with default values.
// The transferGracePolicyDays parameter is used to calculate the ExpiresAt field and
// should be set to the value of the TransferGP field in the PhasePolicy in which the transfer is being processed.
func NewDomainTransfer(transferGracePolicyDays int) DomainTransfer {
	return DomainTransfer{
		ID:          uuid.New(),
		Status:      TransferStatusPending,
		RequestedAt: time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().AddDate(0, 0, transferGracePolicyDays),
		UpdatedAt:   time.Now().UTC(),
	}
}

// FinalizeTransferCommand is a command that can be used to approve or deny a domain transfer.
type FinalizeTransferCommand struct {
	// CorrelationID is an optional field that can be used to store the ID of the request that approved or denied the transfer.
	CorrelationID string

	// Reason is an optional field that can be used to store a reason for denial
	Reason string

	// Status is the new status of the transfer.
	Status TransferStatus
}

// Finalize is a method that can be used to approve or deny a domain transfer.
// It will return an error if the transfer cannot be finalized.
func (t *DomainTransfer) Finalize(cmd FinalizeTransferCommand) error {
	switch cmd.Status {
	case TransferStatusApproved:
		return t.approve(cmd.CorrelationID, cmd.Reason)
	case TransferStatusDenied:
		return t.deny(cmd.CorrelationID, cmd.Reason)
	default:
		return ErrInvalidTransferStatus
	}
}

// approve marks the transfer as approved and sets the status to TransferStatusApproved.
// CorrelationID is an optional field that can be used to store the ID of the request that approved the transfer.
// If the transfer is already denied, it will return an error. If the transfer is already approved, it will be idempotent.
func (t *DomainTransfer) approve(correlationID, reason string) error {
	if t.isComplete() {
		if t.Status == TransferStatusApproved {
			return nil // already approved - idempotent
		}
		return ErrTransferComplete
	}
	t.Status = TransferStatusApproved
	t.CorrelationID = correlationID
	t.Reason = reason
	t.UpdatedAt = time.Now().UTC()

	return nil
}

// deny marks the transfer as denied and sets the status to TransferStatusDenied and sets the denial reason.
// CorrelationID is an optional field that can be used to store the ID of the request that denied the transfer.
// If the transfer is already approved, it will return an error. If the transfer is already denied, it will be idempotent.
func (t *DomainTransfer) deny(correlationID, reason string) error {
	if t.isComplete() {
		if t.Status == TransferStatusDenied {
			return nil // already denied - idempotent
		}
		return ErrTransferComplete
	}
	t.Status = TransferStatusDenied
	t.Reason = reason
	t.CorrelationID = correlationID
	t.UpdatedAt = time.Now().UTC()

	return nil
}

// isComplete returns true if the transfer is approved or denied.
// a completed transfer should not be updated.
func (t *DomainTransfer) isComplete() bool {
	return t.Status == TransferStatusApproved || t.Status == TransferStatusDenied
}
