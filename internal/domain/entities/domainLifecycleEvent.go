package entities

import (
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrEmptyTldName         = errors.New("TldName cannot be empty")
	ErrEmptyTransactionType = errors.New("TransactionType cannot be empty")
	ErrEmptyClientID        = errors.New("ClientID cannot be empty")
	ErrEmptyDomainName      = errors.New("DomainName cannot be empty")
)

// DomainLifeCycleEvent struct defines a billing event that is generated each time a domain is registered, renewed, transferred or deleted
type DomainLifeCycleEvent struct {
	ClientID        string          // ClientID is the unique identifier of the client Registrar.ClID
	ResellerID      string          // ResellerID is the unique identifier of the reseller if applicable
	TldName         string          // TldName is the top level domain name (e.g. COM, NET, ORG)
	DomainName      string          // DomainName is the domain name (e.g. example.net)
	DomainYears     int             // DomainYears is the number of years the transaction is for
	TransactionType TransactionType // TransactionType is the type of transaction (e.g. REGISTRATION, RENEWAL, TRANSFER, DELETE)
	SKU             string          // SKU is the Stock Keeping Unit of the transaction (e.g. COM-REGISTRATION-1)
	Quote           Quote           // Quote is the quote of the transaction
	TraceID         string          // TraceID is the unique identifier of the transaction
	CorrelationID   string          // CorrelationID is a link to an upstream event if applicable
	TimeStamp       time.Time       // TimeStamp is the time the transaction took place
}

// NewDomainLifeCycleEvent creates a new DomainLifeCycleEvent with the given parameters
func NewDomainLifeCycleEvent(clientID, resellerID, tldName, domainName string, domainYears int, transactionType TransactionType) (*DomainLifeCycleEvent, error) {
	if clientID == "" {
		return nil, ErrEmptyClientID
	}
	if domainName == "" {
		return nil, ErrEmptyDomainName
	}
	dle := &DomainLifeCycleEvent{
		ClientID:        clientID,
		ResellerID:      resellerID,
		TldName:         tldName,
		DomainName:      domainName,
		DomainYears:     domainYears,
		TransactionType: transactionType,
		TimeStamp:       time.Now().UTC(),
	}
	err := dle.GenerateSKU()
	if err != nil {
		return nil, err
	}
	return dle, nil
}

// generateSKU generates and sets the DomainLifeCycleEvent.SKU based on the TLD, TransactionType and DomainYears (e.g. COM-REGISTRATION-1)
func (d *DomainLifeCycleEvent) GenerateSKU() error {
	if d.TldName == "" {
		return ErrEmptyTldName
	}
	if d.TransactionType == "" {
		return ErrEmptyTransactionType
	}
	d.SKU = strings.ToUpper(d.TldName) + "-" + strings.ToUpper(d.TransactionType.String()) + "-" + strings.ToUpper(strconv.Itoa(d.DomainYears))
	return nil
}
