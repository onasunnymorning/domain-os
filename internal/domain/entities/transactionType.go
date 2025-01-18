package entities

import (
	"fmt"
)

// TransactionType represents the type of transaction that is being requested
type TransactionType string

// String returns the string representation of the TransactionType
func (t TransactionType) String() string {
	return string(t)
}

// Transaction types
const (
	TransactionTypeRegistration = TransactionType("registration")
	TransactionTypeRenewal      = TransactionType("renewal")
	TransactionTypeAutoRenewal  = TransactionType("auto_renewal")
	TransactionTypeTransfer     = TransactionType("transfer")
	TransactionTypeRestore      = TransactionType("restore")
	TransactionTypeDelete       = TransactionType("delete")
	TransactionTypeAdminDelete  = TransactionType("admin-delete")
	TransactionTypeAdminCreate  = TransactionType("admin-create")
	TransactionTypeInfo         = TransactionType("info")
	TransactionTypeExpiry       = TransactionType("expiry")
	TransactionTypePurge        = TransactionType("purge")
)

var (

	// ValidTransactionTypes is a list of valid transaction types supported by the system
	ValidTransactionTypes = []TransactionType{
		TransactionTypeRegistration,
		TransactionTypeRenewal,
		TransactionTypeTransfer,
		TransactionTypeRestore,
		TransactionTypeDelete,
		TransactionTypeInfo,
		TransactionTypeAutoRenewal,
		TransactionTypeExpiry,
		TransactionTypePurge,
		TransactionTypeAdminDelete,
		TransactionTypeAdminCreate,
	}

	// ValidTransactionTypesForQuote is a list of valid transaction types supported in quotes
	ValidTransactionTypesForQuote = []TransactionType{
		TransactionTypeRegistration,
		TransactionTypeRenewal,
		TransactionTypeTransfer,
		TransactionTypeRestore,
		TransactionTypeAutoRenewal,
	}

	ErrInvalidTransactionTypeForQuote = fmt.Errorf("invalid transaction type, only %v are valid types for requesting quotes", ValidTransactionTypesForQuote)
	ErrInvalidTransactionType         = fmt.Errorf("invalid transaction type, only %v are valid", ValidTransactionTypes)
)
