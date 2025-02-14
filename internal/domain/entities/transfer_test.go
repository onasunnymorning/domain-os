package entities

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewDomainTransfer(t *testing.T) {
	transferGracePolicyDays := 5
	domainTransfer := NewDomainTransfer(transferGracePolicyDays)

	if domainTransfer.ID == uuid.Nil {
		t.Errorf("expected a valid UUID, got %v", domainTransfer.ID)
	}

	if domainTransfer.Status != TransferStatusPending {
		t.Errorf("expected status %v, got %v", TransferStatusPending, domainTransfer.Status)
	}

	if domainTransfer.CreatedAt.IsZero() {
		t.Errorf("expected a valid RequestedAt time, got %v", domainTransfer.CreatedAt)
	}

	expectedExpiresAt := domainTransfer.CreatedAt.AddDate(0, 0, transferGracePolicyDays)
	if !domainTransfer.ExpiryDate.Equal(expectedExpiresAt) {
		t.Errorf("expected ExpiresAt %v, got %v", expectedExpiresAt, domainTransfer.ExpiryDate)
	}
}

func TestDomainTransfer_ApproveDeny(t *testing.T) {
	testcases := []struct {
		name           string
		transfer       DomainTransfer
		transferStatus TransferStatus
		expectedError  error
	}{
		{
			name: "Approve transfer",
			transfer: DomainTransfer{
				Status: TransferStatusPending,
			},
			transferStatus: TransferStatusApproved,
			expectedError:  nil,
		},
		{
			name: "Deny transfer",
			transfer: DomainTransfer{
				Status: TransferStatusPending,
			},
			transferStatus: TransferStatusDenied,
			expectedError:  nil,
		},
		{
			name: "Transfer already approved",
			transfer: DomainTransfer{
				Status: TransferStatusApproved,
			},
			transferStatus: TransferStatusApproved,
			expectedError:  nil,
		},
		{
			name: "Transfer already denied",
			transfer: DomainTransfer{
				Status: TransferStatusDenied,
			},
			transferStatus: TransferStatusDenied,
			expectedError:  nil,
		},
		{
			name: "Transfer already complete (approved)",
			transfer: DomainTransfer{
				Status: TransferStatusApproved,
			},
			transferStatus: TransferStatusDenied,
			expectedError:  ErrTransferComplete,
		},
		{
			name: "Transfer already complete (denied)",
			transfer: DomainTransfer{
				Status: TransferStatusDenied,
			},
			transferStatus: TransferStatusApproved,
			expectedError:  ErrTransferComplete,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			switch tc.transferStatus {
			case TransferStatusApproved:
				err := tc.transfer.approve("correlationID", "reason")
				if err != tc.expectedError {
					t.Errorf("expected error %v, got %v", tc.expectedError, err)
				}
			case TransferStatusDenied:
				err := tc.transfer.deny("correlationID", "reason")
				if err != tc.expectedError {
					t.Errorf("expected error %v, got %v", tc.expectedError, err)
				}
			}
		})
	}

}
func TestDomainTransfer_Finalize(t *testing.T) {
	testcases := []struct {
		name          string
		transfer      DomainTransfer
		command       FinalizeTransferCommand
		expectedError error
	}{
		{
			name: "Finalize approve transfer",
			transfer: DomainTransfer{
				Status: TransferStatusPending,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusApproved,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: nil,
		},
		{
			name: "Finalize deny transfer",
			transfer: DomainTransfer{
				Status: TransferStatusPending,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusDenied,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: nil,
		},
		{
			name: "Finalize transfer with invalid status",
			transfer: DomainTransfer{
				Status: TransferStatusPending,
			},
			command: FinalizeTransferCommand{
				Status:        "invalid",
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: ErrInvalidTransferStatus,
		},
		{
			name: "Finalize transfer already approved",
			transfer: DomainTransfer{
				Status: TransferStatusApproved,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusApproved,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: nil,
		},
		{
			name: "Finalize transfer already denied",
			transfer: DomainTransfer{
				Status: TransferStatusDenied,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusDenied,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: nil,
		},
		{
			name: "Finalize transfer already complete (approved)",
			transfer: DomainTransfer{
				Status: TransferStatusApproved,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusDenied,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: ErrTransferComplete,
		},
		{
			name: "Finalize transfer already complete (denied)",
			transfer: DomainTransfer{
				Status: TransferStatusDenied,
			},
			command: FinalizeTransferCommand{
				Status:        TransferStatusApproved,
				CorrelationID: "correlationID",
				Reason:        "reason",
			},
			expectedError: ErrTransferComplete,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.transfer.Finalize(tc.command)
			if err != tc.expectedError {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}
		})
	}
}
