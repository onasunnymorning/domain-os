package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/hashgraph/hedera-sdk-go/v2"
)

// DeploymentConfig holds the configuration for deploying the accreditation system
type DeploymentConfig struct {
	Network          string `json:"network"`
	OperatorAccount  string `json:"operatorAccount"`
	OperatorKey      string `json:"operatorKey"`
	ContractBytecode string `json:"contractBytecode"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: go run deploy.go <config.json>")
	}

	configFile := os.Args[1]

	// Load configuration
	configData, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	var config DeploymentConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	// Setup Hedera client
	var client *hedera.Client
	switch config.Network {
	case "testnet":
		client = hedera.ClientForTestnet()
	case "mainnet":
		client = hedera.ClientForMainnet()
	default:
		log.Fatalf("Unsupported network: %s", config.Network)
	}

	operatorAccountID, err := hedera.AccountIDFromString(config.OperatorAccount)
	if err != nil {
		log.Fatalf("Invalid operator account ID: %v", err)
	}

	operatorKey, err := hedera.PrivateKeyFromString(config.OperatorKey)
	if err != nil {
		log.Fatalf("Invalid operator private key: %v", err)
	}

	client.SetOperator(operatorAccountID, operatorKey)

	fmt.Printf("Deploying to %s network with account %s\n", config.Network, config.OperatorAccount)

	// Deploy smart contract
	contractID, err := deployContract(client, config.ContractBytecode)
	if err != nil {
		log.Fatalf("Failed to deploy contract: %v", err)
	}

	fmt.Printf("✅ Smart contract deployed: %s\n", contractID.String())

	// Create consensus topic
	topicID, err := createConsensusTopic(client)
	if err != nil {
		log.Fatalf("Failed to create consensus topic: %v", err)
	}

	fmt.Printf("✅ Consensus topic created: %s\n", topicID.String())

	// Output deployment information
	deploymentInfo := map[string]string{
		"network":          config.Network,
		"contractId":       contractID.String(),
		"consensusTopicId": topicID.String(),
		"operatorAccount":  config.OperatorAccount,
	}

	deploymentJSON, _ := json.MarshalIndent(deploymentInfo, "", "  ")

	// Save deployment info
	if err := os.WriteFile("deployment.json", deploymentJSON, 0644); err != nil {
		log.Printf("Warning: Failed to save deployment info: %v", err)
	}

	fmt.Println("\n📋 Deployment Summary:")
	fmt.Println(string(deploymentJSON))
	fmt.Println("\n🚀 Next steps:")
	fmt.Println("1. Update your service configurations with the above IDs")
	fmt.Println("2. Register initial TLDs and registrars")
	fmt.Println("3. Start the publisher and reader services")
	fmt.Println("4. Test with sample accreditation requests")
}

func deployContract(client *hedera.Client, bytecode string) (hedera.ContractID, error) {
	// Convert hex bytecode to bytes
	bytecodeBytes := []byte(bytecode) // In real implementation, decode from hex

	// Create the contract
	txResponse, err := hedera.NewContractCreateTransaction().
		SetBytecode(bytecodeBytes).
		SetGas(300000).
		SetConstructorParameters(hedera.NewContractFunctionParameters()).
		Execute(client)

	if err != nil {
		return hedera.ContractID{}, fmt.Errorf("failed to create contract: %w", err)
	}

	// Get the receipt
	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return hedera.ContractID{}, fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return hedera.ContractID{}, fmt.Errorf("contract creation failed: %s", receipt.Status)
	}

	return *receipt.ContractID, nil
}

func createConsensusTopic(client *hedera.Client) (hedera.TopicID, error) {
	// Create topic for accreditation events
	txResponse, err := hedera.NewTopicCreateTransaction().
		SetTopicMemo("Domain OS Accreditation Events").
		SetSubmitKey(client.GetOperatorPublicKey()).
		Execute(client)

	if err != nil {
		return hedera.TopicID{}, fmt.Errorf("failed to create topic: %w", err)
	}

	receipt, err := txResponse.GetReceipt(client)
	if err != nil {
		return hedera.TopicID{}, fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status != hedera.StatusSuccess {
		return hedera.TopicID{}, fmt.Errorf("topic creation failed: %s", receipt.Status)
	}

	return *receipt.TopicID, nil
}
