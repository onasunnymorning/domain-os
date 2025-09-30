package accreditation
package accreditation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"go.uber.org/zap"
)

// HederaAccreditationReader reads accreditation data from Hedera smart contract
// and listens to consensus service events for real-time updates
type HederaAccreditationReader struct {
	client         *hedera.Client
	consensusTopicID hedera.TopicID
	contractID     hedera.ContractID
	
	// Account for queries (read-only operations)
	operatorAccountID hedera.AccountID
	operatorKey       hedera.PrivateKey
	
	logger *zap.Logger
	
	// Local cache for performance (in production, use Redis or similar)
	cache map[string]bool
}

// AccreditationStatus represents the result of an accreditation query
type AccreditationStatus struct {
	TLDName       string    `json:"tldName"`
	RegistrarClID string    `json:"registrarClID"`
	IsAccredited  bool      `json:"isAccredited"`
	LastUpdated   time.Time `json:"lastUpdated"`
	Source        string    `json:"source"` // "contract" or "cache"
}

// NewHederaAccreditationReader creates a new Hedera-based reader
func NewHederaAccreditationReader(
	network string,
	operatorAccountID hedera.AccountID,
	operatorKey hedera.PrivateKey,
	consensusTopicID hedera.TopicID,
	contractID hedera.ContractID,
	logger *zap.Logger,
) (*HederaAccreditationReader, error) {
	
	var client *hedera.Client
	
	switch network {
	case "testnet":
		client = hedera.ClientForTestnet()
	case "mainnet":
		client = hedera.ClientForMainnet()
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}
	
	client.SetOperator(operatorAccountID, operatorKey)
	
	return &HederaAccreditationReader{
		client:            client,
		consensusTopicID:  consensusTopicID,
		contractID:        contractID,
		operatorAccountID: operatorAccountID,
		operatorKey:       operatorKey,
		logger:            logger,
		cache:             make(map[string]bool),
	}, nil
}

// IsRegistrarAccreditedForTLD checks if a registrar is accredited for a specific TLD
// This replaces your current AccreditationService.IsRegistrarAccreditedForTLD method
func (r *HederaAccreditationReader) IsRegistrarAccreditedForTLD(ctx context.Context, tldName, registrarClID string) (bool, error) {
	cacheKey := fmt.Sprintf("%s:%s", tldName, registrarClID)
	
	// Check cache first (in production, implement TTL)
	if cachedResult, exists := r.cache[cacheKey]; exists {
		r.logger.Debug("Returning cached accreditation status",
			zap.String("tldName", tldName),
			zap.String("registrarClID", registrarClID),
			zap.Bool("isAccredited", cachedResult))
		return cachedResult, nil
	}
	
	// Query smart contract
	result, err := r.queryContractAccreditation(ctx, tldName, registrarClID)
	if err != nil {
		return false, fmt.Errorf("failed to query contract: %w", err)
	}
	
	// Update cache
	r.cache[cacheKey] = result
	
	r.logger.Debug("Queried accreditation from contract",
		zap.String("tldName", tldName),
		zap.String("registrarClID", registrarClID),
		zap.Bool("isAccredited", result))
	
	return result, nil
}

// queryContractAccreditation queries the smart contract for accreditation status
func (r *HederaAccreditationReader) queryContractAccreditation(ctx context.Context, tldName, registrarClID string) (bool, error) {
	// Create contract call query
	contractQuery := hedera.NewContractCallQuery().
		SetContractID(r.contractID).
		SetGas(50000). // Read operations use less gas
		SetFunction("isAccredited",
			hedera.NewContractFunctionParameters().
				AddString(tldName).
				AddString(registrarClID))
	
	// Execute query
	response, err := contractQuery.Execute(r.client)
	if err != nil {
		return false, fmt.Errorf("failed to execute contract query: %w", err)
	}
	
	// Parse response - expecting a boolean return value
	if len(response.AsBytes()) == 0 {
		return false, fmt.Errorf("empty response from contract")
	}
	
	// The response should be a 32-byte word with the boolean value
	// In Solidity, bool true is represented as 1, false as 0
	result := response.GetBool(0)
	
	return result, nil
}

// GetAccreditationStatus returns detailed accreditation status
func (r *HederaAccreditationReader) GetAccreditationStatus(ctx context.Context, tldName, registrarClID string) (*AccreditationStatus, error) {
	isAccredited, err := r.IsRegistrarAccreditedForTLD(ctx, tldName, registrarClID)
	if err != nil {
		return nil, err
	}
	
	cacheKey := fmt.Sprintf("%s:%s", tldName, registrarClID)
	source := "contract"
	if _, exists := r.cache[cacheKey]; exists {
		source = "cache"
	}
	
	return &AccreditationStatus{
		TLDName:       tldName,
		RegistrarClID: registrarClID,
		IsAccredited:  isAccredited,
		LastUpdated:   time.Now().UTC(),
		Source:        source,
	}, nil
}

// ListTLDRegistrars returns registrars accredited for a specific TLD
// Note: This is a simplified version. In production, you'd implement proper pagination
// and potentially use a separate indexing service for complex queries
func (r *HederaAccreditationReader) ListTLDRegistrars(ctx context.Context, pageSize int, pageCursor, tldName string) ([]*entities.Registrar, error) {
	// For now, this is a placeholder implementation
	// In a full implementation, you would:
	// 1. Listen to consensus service events to build an index
	// 2. Query that index for efficient lookups
	// 3. Implement proper pagination
	
	r.logger.Warn("ListTLDRegistrars not fully implemented - requires event indexing service")
	return []*entities.Registrar{}, nil
}

// ListRegistrarTLDs returns TLDs that a registrar is accredited for
func (r *HederaAccreditationReader) ListRegistrarTLDs(ctx context.Context, pageSize int, pageCursor, rarClID string) ([]*entities.TLD, error) {
	// Similar to ListTLDRegistrars, this requires an indexing service
	r.logger.Warn("ListRegistrarTLDs not fully implemented - requires event indexing service")
	return []*entities.TLD{}, nil
}

// StartConsensusListener starts listening to Hedera Consensus Service for real-time updates
func (r *HederaAccreditationReader) StartConsensusListener(ctx context.Context) error {
	// Subscribe to consensus messages starting from now
	subscribeTransaction := hedera.NewTopicMessageQuery().
		SetTopicID(r.consensusTopicID).
		SetStartTime(time.Now())
	
	// Set up message handler
	_, err := subscribeTransaction.Subscribe(r.client, func(message hedera.TopicMessage) {
		r.handleConsensusMessage(message)
	})
	
	if err != nil {
		return fmt.Errorf("failed to subscribe to consensus topic: %w", err)
	}
	
	r.logger.Info("Started listening to consensus service",
		zap.String("topicId", r.consensusTopicID.String()))
	
	return nil
}

// handleConsensusMessage processes incoming consensus messages
func (r *HederaAccreditationReader) handleConsensusMessage(message hedera.TopicMessage) {
	var event AccreditationEvent
	
	// Parse the message
	if err := json.Unmarshal(message.Contents, &event); err != nil {
		r.logger.Error("Failed to parse consensus message", zap.Error(err))
		return
	}
	
	r.logger.Debug("Received consensus message",
		zap.String("type", event.Type),
		zap.String("tldName", event.TLDName),
		zap.String("registrarClID", event.RegistrarClID),
		zap.String("action", string(event.Action)),
		zap.Bool("success", event.Success))
	
	// Update cache based on the event
	if event.Success {
		cacheKey := fmt.Sprintf("%s:%s", event.TLDName, event.RegistrarClID)
		
		switch event.Action {
		case ActionCreate:
			r.cache[cacheKey] = true
		case ActionRevoke:
			r.cache[cacheKey] = false
		}
		
		r.logger.Debug("Updated cache from consensus message",
			zap.String("cacheKey", cacheKey),
			zap.Bool("isAccredited", r.cache[cacheKey]))
	}
}

// GetRegistrarInfo queries registrar information from the smart contract
func (r *HederaAccreditationReader) GetRegistrarInfo(ctx context.Context, clID string) (*entities.Registrar, error) {
	// Query the registrars mapping in the smart contract
	contractQuery := hedera.NewContractCallQuery().
		SetContractID(r.contractID).
		SetGas(50000).
		SetFunction("registrars",
			hedera.NewContractFunctionParameters().
				AddString(clID))
	
	response, err := contractQuery.Execute(r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to query registrar: %w", err)
	}
	
	// Parse the response (this would need to match your Solidity struct)
	// The exact parsing depends on how Solidity returns the struct
	if len(response.AsBytes()) == 0 {
		return nil, fmt.Errorf("registrar not found: %s", clID)
	}
	
	// Extract values from response
	// Note: This is simplified - actual implementation depends on ABI encoding
	clIDResult := response.GetString(0)
	gurID := response.GetUint256(1)
	status := response.GetString(2)
	ianaStatus := response.GetString(3)
	exists := response.GetBool(4)
	
	if !exists {
		return nil, fmt.Errorf("registrar not found: %s", clID)
	}
	
	// Convert to domain entity
	registrar := &entities.Registrar{
		ClID:       entities.ClIDType(clIDResult),
		GurID:      int(gurID.Int64()),
		Status:     entities.RegistrarStatus(status),
		IANAStatus: entities.IANARegistrarStatus(ianaStatus),
	}
	
	return registrar, nil
}

// ClearCache clears the internal cache (useful for testing or manual refresh)
func (r *HederaAccreditationReader) ClearCache() {
	r.cache = make(map[string]bool)
	r.logger.Debug("Cache cleared")
}

// Close closes the Hedera client connection
func (r *HederaAccreditationReader) Close() error {
	if err := r.client.Close(); err != nil {
		return fmt.Errorf("failed to close Hedera client: %w", err)
	}
	return nil
}

// AccreditationEvent represents events from consensus service (shared with publisher)
type AccreditationEvent struct {
	Type          string                 `json:"type"`
	TLDName       string                 `json:"tldName"`
	RegistrarClID string                 `json:"registrarClID"`
	Action        AccreditationAction    `json:"action"`
	Success       bool                   `json:"success"`
	Error         string                 `json:"error,omitempty"`
	TransactionID string                 `json:"transactionId"`
	Timestamp     time.Time              `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	RequestID     string                 `json:"requestId,omitempty"`
}

type AccreditationAction string

const (
	ActionCreate AccreditationAction = "CREATE"
	ActionRevoke AccreditationAction = "REVOKE"
)
