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
	TransactionTypeTransfer     = TransactionType("transfer")
	TransactionTypeRestore      = TransactionType("restore")
	TransactionTypeDelete       = TransactionType("delete")
	TransactionTypeInfo         = TransactionType("info")
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
	}

	// ValidTransactionTypesForQuote is a list of valid transaction types supported in quotes
	ValidTransactionTypesForQuote = []TransactionType{
		TransactionTypeRegistration,
		TransactionTypeRenewal,
		TransactionTypeTransfer,
		TransactionTypeRestore,
	}

	ErrInvalidTransactionTypeForQuote = fmt.Errorf("invalid transaction type, only %v are valid types for requesting quotes", ValidTransactionTypesForQuote)
	ErrInvalidTransactionType         = fmt.Errorf("invalid transaction type, only %v are valid", ValidTransactionTypes)
)
