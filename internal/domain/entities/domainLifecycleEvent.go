package entities

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrEmptyTldName         = errors.New("TldName cannot be empty")
	ErrEmptyTransactionType = errors.New("TransactionType cannot be empty")
	ErrEmptyClientID        = errors.New("ClientID cannot be empty")
	ErrEmptyDomainName      = errors.New("DomainName cannot be empty")
)

// DomainLifeCycleEvent struct defines an event that is generated each time a domain is registered, renewed, transferred or deleted
// Its consumers are the billing and reporting systems as well as make lifecycle events visible to users
type DomainLifeCycleEvent struct {
	ClientID        string          // ClientID is the unique identifier of the client Registrar.ClID
	ResellerID      string          // ResellerID is the unique identifier of the reseller if applicable
	TldName         string          // TldName is the top level domain name (e.g. COM, NET, ORG)
	DomainName      string          // DomainName is the domain name (e.g. example.net)
	DomainRoID      string          // DomainRoID is the unique identifier of the domain Registrar Object ID
	DomainYears     int             // DomainYears is the number of years the transaction is for
	TimeStamp       time.Time       // TimeStamp is the time the transaction took place
	TransactionType TransactionType // TransactionType is the type of transaction (e.g. REGISTRATION, RENEWAL, TRANSFER, DELETE)
	TraceID         string          // TraceID is the unique identifier allowing tracing events across services (e.g. traceID set by activity or client, event gets processed by billing application, billing appliction logs can contain trace_id)
	CorrelationID   string          // CorrelationID is the identifier allowing to group events together in a business context (e.g. auto-renew-workflow-kdjsflkwr238fnelwkknk34ln5)
	SKU             string          // SKU is the Stock Keeping Unit of the transaction (e.g. COM-REGISTRATION-1)
	Quote           Quote           // The quote for the transaction retrieved at the time of the transaction
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
	d.SKU = fmt.Sprintf("%s-%s-%dY", strings.ToUpper(d.TldName), strings.ToUpper(d.TransactionType.String()), d.DomainYears)
	return nil
}
