# Hedera Accreditation Smart Contract Integration

This directory contains the Hedera-based implementation of the Domain OS accreditation system using smart contracts and the Hedera Consensus Service.

## Architecture Overview

```
┌─────────────────┐    ┌──────────────────────────┐    ┌─────────────────┐
│   Publishers    │───▶│    Hedera Network        │───▶│     Readers     │
│                 │    │                          │    │                 │
│ - HTTP API      │    │ ┌──────────────────────┐ │    │ - Query API     │
│ - CLI Tools     │    │ │ AccreditationContract│ │    │ - Event Stream  │
│ - Admin Panel   │    │ │   (Smart Contract)   │ │    │ - Cache Layer   │
│                 │    │ └──────────────────────┘ │    │                 │
│                 │    │ ┌──────────────────────┐ │    │                 │
│                 │    │ │  Consensus Service   │ │    │                 │
│                 │    │ │    (Event Log)       │ │    │                 │
│                 │    │ └──────────────────────┘ │    │                 │
└─────────────────┘    └──────────────────────────┘    └─────────────────┘
```

## Components

### 1. Smart Contract (`AccreditationContract.sol`)
- Implements core business logic from your domain entities
- Enforces ICANN accreditation rules for gTLDs
- Emits events for state changes
- Stores accreditation mappings on-chain

### 2. Publisher Service (`hedera_publisher.go`)
- Receives accreditation commands via HTTP/gRPC
- Validates and executes smart contract functions
- Publishes events to Hedera Consensus Service
- Handles error cases and retries

### 3. Reader Service (`hedera_reader.go`)
- Queries smart contract for current state
- Listens to consensus service for real-time updates
- Maintains local cache for performance
- Provides REST API for queries

## Setup Instructions

### Prerequisites

1. **Hedera Account**: Create accounts on Hedera Testnet/Mainnet
2. **Go Dependencies**: Install Hedera SDK
```bash
go mod tidy
go get github.com/hashgraph/hedera-sdk-go/v2
```

### 1. Deploy Smart Contract

```bash
# Compile contract (requires Solidity compiler)
solc --abi --bin contracts/accreditation/AccreditationContract.sol

# Deploy using Hedera SDK (see deployment script)
go run scripts/deploy_contract.go
```

### 2. Create Consensus Topic

```bash
# Create topic for events
go run scripts/create_topic.go
```

### 3. Configure Services

Create `config/hedera.yaml`:
```yaml
hedera:
  network: "testnet"  # or "mainnet"
  operator:
    accountId: "0.0.1234"
    privateKey: "302e020100300506032b657004220420..."
  
  accreditation:
    contractId: "0.0.5678"
    consensusTopicId: "0.0.9012"
    
publisher:
  port: 8080
  
reader:
  port: 8081
  cache:
    ttl: "5m"
```

## Usage Examples

### Starting Services

```bash
# Start publisher service
go run cmd/publisher/main.go

# Start reader service  
go run cmd/reader/main.go
```

### API Calls

```bash
# Create accreditation
curl -X POST http://localhost:8080/accreditations \
  -H "Content-Type: application/json" \
  -d '{
    "tldName": "com",
    "registrarClID": "example-rar",
    "action": "CREATE"
  }'

# Check accreditation status
curl http://localhost:8081/accreditations/com/example-rar

# Response:
{
  "tldName": "com",
  "registrarClID": "example-rar", 
  "isAccredited": true,
  "lastUpdated": "2025-09-19T10:30:00Z",
  "source": "contract"
}
```

## Benefits Over Current Architecture

### 1. **Decentralization**
- No single point of failure
- Consensus-based validation
- Immutable audit trail

### 2. **Scalability** 
- Publishers scale horizontally
- Readers can be deployed globally
- Caching reduces contract queries

### 3. **Transparency**
- All accreditations on public ledger
- Real-time event streaming
- Verifiable business rules

### 4. **Cost Efficiency**
- Pay-per-transaction model
- No infrastructure maintenance
- Efficient consensus mechanism

## Migration Strategy

### Phase 1: Parallel Operation
- Deploy Hedera services alongside existing system
- Mirror writes to both systems
- Compare results for validation

### Phase 2: Read Migration
- Gradually shift read traffic to Hedera readers
- Monitor performance and accuracy
- Keep existing system as fallback

### Phase 3: Write Migration
- Start writing to Hedera first
- Sync to existing system for backup
- Monitor for issues

### Phase 4: Full Migration
- Stop writing to old system
- Use Hedera as source of truth
- Decommission legacy infrastructure

## Monitoring

### Key Metrics
- Transaction success rate
- Query response times  
- Consensus message lag
- Cache hit ratio

### Alerts
- Contract execution failures
- Consensus service disconnection
- High query latency
- Cache invalidation events

## Security Considerations

1. **Private Key Management**: Use secure key storage (HSM, KMS)
2. **Access Control**: Implement proper authentication/authorization
3. **Rate Limiting**: Prevent abuse of contract functions
4. **Input Validation**: Validate all inputs before contract calls
5. **Error Handling**: Don't expose sensitive information in errors

## Cost Optimization

1. **Batch Operations**: Group multiple operations when possible
2. **Efficient Queries**: Use views for read-only operations
3. **Caching Strategy**: Implement intelligent caching with TTL
4. **Gas Optimization**: Optimize contract code for lower gas usage

## Next Steps

1. **Test Deployment**: Deploy to Hedera Testnet
2. **Load Testing**: Verify performance under load
3. **Integration**: Connect with existing Domain OS APIs
4. **Monitoring Setup**: Implement comprehensive monitoring
5. **Documentation**: Create operational runbooks
