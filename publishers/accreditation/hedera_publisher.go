package accreditation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashgraph/hedera-sdk-go/v2"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"go.uber.org/zap"
)

// HederaAccreditationPublisher publishes accreditation events to Hedera Consensus Service
// and interacts with the AccreditationContract smart contract
type HederaAccreditationPublisher struct {
	// Hedera client for consensus service and smart contracts
	client           *hedera.Client
	consensusTopicID hedera.TopicID
	contractID       hedera.ContractID

	// Account for signing transactions
	operatorAccountID hedera.AccountID
	operatorKey       hedera.PrivateKey

	logger *zap.Logger
}

// AccreditationCommand represents the command to process accreditation
type AccreditationCommand struct {
	TLDName       string                 `json:"tldName"`
	RegistrarClID string                 `json:"registrarClID"`
	Action        AccreditationAction    `json:"action"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	RequestID     string                 `json:"requestId"`
}

// AccreditationAction represents the type of accreditation action
type AccreditationAction string

const (
	ActionCreate AccreditationAction = "CREATE"
	ActionRevoke AccreditationAction = "REVOKE"
)

// AccreditationEvent represents events published to Hedera Consensus Service
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
}

// NewHederaAccreditationPublisher creates a new Hedera-based publisher
func NewHederaAccreditationPublisher(
	network string,
	operatorAccountID hedera.AccountID,
	operatorKey hedera.PrivateKey,
	consensusTopicID hedera.TopicID,
	contractID hedera.ContractID,
	logger *zap.Logger,
) (*HederaAccreditationPublisher, error) {

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

	return &HederaAccreditationPublisher{
		client:            client,
		consensusTopicID:  consensusTopicID,
		contractID:        contractID,
		operatorAccountID: operatorAccountID,
		operatorKey:       operatorKey,
		logger:            logger,
	}, nil
}

// ProcessAccreditation processes an accreditation command using Hedera smart contract
func (p *HederaAccreditationPublisher) ProcessAccreditation(ctx context.Context, cmd AccreditationCommand) error {
	p.logger.Info("Processing accreditation command",
		zap.String("tldName", cmd.TLDName),
		zap.String("registrarClID", cmd.RegistrarClID),
		zap.String("action", string(cmd.Action)),
		zap.String("requestId", cmd.RequestID))

	var err error
	var txResponse hedera.TransactionResponse

	switch cmd.Action {
	case ActionCreate:
		txResponse, err = p.createAccreditation(ctx, cmd)
	case ActionRevoke:
		txResponse, err = p.revokeAccreditation(ctx, cmd)
	default:
		return fmt.Errorf("unsupported action: %s", cmd.Action)
	}

	// Create event for consensus service
	event := AccreditationEvent{
		Type:          "AccreditationProcessed",
		TLDName:       cmd.TLDName,
		RegistrarClID: cmd.RegistrarClID,
		Action:        cmd.Action,
		Success:       err == nil,
		Timestamp:     time.Now().UTC(),
		Metadata:      cmd.Metadata,
		RequestID:     cmd.RequestID,
	}

	if err != nil {
		event.Error = err.Error()
		p.logger.Error("Accreditation processing failed", zap.Error(err))
	} else {
		event.TransactionID = txResponse.TransactionID.String()
		p.logger.Info("Accreditation processed successfully",
			zap.String("transactionId", event.TransactionID))
	}

	// Publish event to Hedera Consensus Service
	if publishErr := p.publishEvent(ctx, event); publishErr != nil {
		p.logger.Error("Failed to publish event to consensus service", zap.Error(publishErr))
		// Don't fail the operation if consensus publishing fails
	}

	return err
}

// createAccreditation calls the smart contract's createAccreditation function
func (p *HederaAccreditationPublisher) createAccreditation(ctx context.Context, cmd AccreditationCommand) (hedera.TransactionResponse, error) {
	// Prepare contract call
	contractCall := hedera.NewContractExecuteTransaction().
		SetContractID(p.contractID).
		SetGas(100000). // Adjust gas limit as needed
		SetFunction("createAccreditation",
			hedera.NewContractFunctionParameters().
				AddString(cmd.TLDName).
				AddString(cmd.RegistrarClID))

	// Execute transaction
	txResponse, err := contractCall.Execute(p.client)
	if err != nil {
		return hedera.TransactionResponse{}, fmt.Errorf("failed to execute createAccreditation: %w", err)
	}

	// Get receipt to check for success
	receipt, err := txResponse.GetReceipt(p.client)
	if err != nil {
		return txResponse, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return txResponse, fmt.Errorf("transaction failed with status: %s", receipt.Status)
	}

	return txResponse, nil
}

// revokeAccreditation calls the smart contract's revokeAccreditation function
func (p *HederaAccreditationPublisher) revokeAccreditation(ctx context.Context, cmd AccreditationCommand) (hedera.TransactionResponse, error) {
	reason := "Manual revocation"
	if reasonVal, ok := cmd.Metadata["reason"].(string); ok {
		reason = reasonVal
	}

	contractCall := hedera.NewContractExecuteTransaction().
		SetContractID(p.contractID).
		SetGas(100000).
		SetFunction("revokeAccreditation",
			hedera.NewContractFunctionParameters().
				AddString(cmd.TLDName).
				AddString(cmd.RegistrarClID).
				AddString(reason))

	txResponse, err := contractCall.Execute(p.client)
	if err != nil {
		return hedera.TransactionResponse{}, fmt.Errorf("failed to execute revokeAccreditation: %w", err)
	}

	receipt, err := txResponse.GetReceipt(p.client)
	if err != nil {
		return txResponse, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return txResponse, fmt.Errorf("transaction failed with status: %s", receipt.Status)
	}

	return txResponse, nil
}

// publishEvent publishes an event to Hedera Consensus Service
func (p *HederaAccreditationPublisher) publishEvent(ctx context.Context, event AccreditationEvent) error {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Submit message to consensus topic
	txResponse, err := hedera.NewTopicMessageSubmitTransaction().
		SetTopicID(p.consensusTopicID).
		SetMessage(eventJSON).
		Execute(p.client)

	if err != nil {
		return fmt.Errorf("failed to submit message to consensus service: %w", err)
	}

	// Get receipt to confirm success
	_, err = txResponse.GetReceipt(p.client)
	if err != nil {
		return fmt.Errorf("failed to get consensus message receipt: %w", err)
	}

	p.logger.Debug("Event published to consensus service",
		zap.String("topicId", p.consensusTopicID.String()),
		zap.String("transactionId", txResponse.TransactionID.String()))

	return nil
}

// RegisterRegistrar registers a registrar with the smart contract
func (p *HederaAccreditationPublisher) RegisterRegistrar(ctx context.Context, registrar *entities.Registrar) error {
	contractCall := hedera.NewContractExecuteTransaction().
		SetContractID(p.contractID).
		SetGas(150000).
		SetFunction("registerRegistrar",
			hedera.NewContractFunctionParameters().
				AddString(registrar.ClID.String()).
				AddUint256(hedera.NewBigInteger(int64(registrar.GurID))).
				AddString(registrar.Status.String()).
				AddString(registrar.IANAStatus.String()))

	txResponse, err := contractCall.Execute(p.client)
	if err != nil {
		return fmt.Errorf("failed to register registrar: %w", err)
	}

	receipt, err := txResponse.GetReceipt(p.client)
	if err != nil {
		return fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return fmt.Errorf("registrar registration failed: %s", receipt.Status)
	}

	p.logger.Info("Registrar registered successfully",
		zap.String("clID", registrar.ClID.String()),
		zap.String("transactionId", txResponse.TransactionID.String()))

	return nil
}

// RegisterTLD registers a TLD with the smart contract
func (p *HederaAccreditationPublisher) RegisterTLD(ctx context.Context, tld *entities.TLD) error {
	contractCall := hedera.NewContractExecuteTransaction().
		SetContractID(p.contractID).
		SetGas(150000).
		SetFunction("registerTLD",
			hedera.NewContractFunctionParameters().
				AddString(tld.Name.String()).
				AddString(tld.Type.String()).
				AddString("active")) // Assuming active status

	txResponse, err := contractCall.Execute(p.client)
	if err != nil {
		return fmt.Errorf("failed to register TLD: %w", err)
	}

	receipt, err := txResponse.GetReceipt(p.client)
	if err != nil {
		return fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return fmt.Errorf("TLD registration failed: %s", receipt.Status)
	}

	p.logger.Info("TLD registered successfully",
		zap.String("name", tld.Name.String()),
		zap.String("transactionId", txResponse.TransactionID.String()))

	return nil
}

// Close closes the Hedera client connection
func (p *HederaAccreditationPublisher) Close() error {
	if err := p.client.Close(); err != nil {
		return fmt.Errorf("failed to close Hedera client: %w", err)
	}
	return nil
}
