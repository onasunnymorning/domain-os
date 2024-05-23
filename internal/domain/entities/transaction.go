package entities

import "errors"

const (
	TransactionTypeRegistration = "registration"
	TransactionTypeRenewal      = "renewal"
	TransactionTypeTransfer     = "transfer"
	TransactionTypeRestore      = "restore"
)

var (
	ValidTransactionTypes = []string{
		TransactionTypeRegistration,
		TransactionTypeRenewal,
		TransactionTypeTransfer,
		TransactionTypeRestore,
	}
	ErrInvalidTransactionType = errors.New("invalid transaction type, only registration, renewal, transfer and restore are allowed")
)
