# Domain OS Copilot Instructions

Domain OS is a domain registry operating system built using Clean Architecture and DDD patterns in Go.

## Architecture Overview

This is a **Clean Architecture** system with strict layer separation:
- `internal/domain/entities/` - Domain entities with embedded business logic and validation
- `internal/domain/repositories/` - Repository interfaces (domain contracts)
- `internal/application/services/` - Application services implementing business use cases
- `internal/application/interfaces/` - Service interfaces
- `internal/application/commands/` & `queries/` - CQRS-style input/output structures
- `internal/infrastructure/` - External concerns (DB, HTTP, message brokers)
- `internal/interface/rest/` - HTTP controllers using Gin framework
- `cmd/` - Application entry points (API servers, CLI tools, workers)

## Key Patterns & Conventions

### Entity Validation & Error Handling
- Entities have embedded `Validate()` methods that check business rules
- Use `errors.Join()` to wrap domain errors with context errors
- Domain errors are defined in entities (e.g., `ErrInvalidDomain`, `ErrDomainExists`)
- Controllers handle domain errors specifically: `if errors.Is(err, entities.ErrInvalidDomain)`

Example error handling:
```go
if err != nil {
    return nil, errors.Join(ErrInvalidAccreditation, err)
}
```

### Repository Pattern
- All repositories follow `NewGorm[Entity]Repository(db *gorm.DB)` naming
- Use transactions in tests: `tx := s.db.Begin(); defer tx.Rollback()`
- Repository methods often return domain entities, not DB structs
- Bulk operations available: `BulkCreate(ctx context.Context, entities []*Entity) error`

### Service Layer
- Services inject multiple repositories via constructor pattern
- Use interfaces for all service dependencies (enables testing)
- Services handle business orchestration between entities and repositories
- Example: `NewDomainService(dRepo, hRepo, roidService, nndrepo, tldRepo, ...)`

### REST Controllers
- Use Gin framework with grouped routes and middleware
- Controllers inject service interfaces, not implementations
- Swagger annotations for API documentation (`@Summary`, `@Router`, etc.)
- Standard patterns: `NewControllerName(e *gin.Engine, service Interface, handler gin.HandlerFunc)`

### Testing
- Use testify/suite for integration tests with database setup
- Tests use transactions to avoid interference: `tx := s.db.Begin(); defer tx.Rollback()`
- Repository tests verify CRUD operations and constraints
- Mock repositories available for unit testing services

## Development Workflows

### Local Development
```bash
# Start essential services (DB + API)
BRANCH=latest docker compose --profile essential up

# Run unit tests (requires PostgreSQL)
./unittests.sh

# Run integration tests
./integrationTests.sh

# Local development with Tilt
tilt up
```

### Database
- PostgreSQL with GORM ORM
- Auto-migration enabled in development
- Connection handled by `postgres.NewConnection()` with config struct
- Use `context.WithContext(ctx)` for all DB operations

### Domain-Specific Rules
- Domain names validated using custom `DomainName` type with `NewDomainName()`
- ROIDs (Registry Object IDs) follow format: `{id}_{ObjectType}-APEX`
- IDN (Internationalized Domain Names) have special `UName`/`OriginalName` fields
- Domain status combinations have complex validation rules
- Monetary values stored in smallest currency units (cents) using `go-money`

### Entry Points
- `cmd/api/ry-admin/` - Main admin API server
- `cmd/epp/` - EPP (Extensible Provisioning Protocol) server
- `cmd/whois/` - WHOIS server
- `cmd/workers/` - Background processing workers

### Key External Dependencies
- Gin for HTTP routing
- GORM for ORM
- Swagger for API documentation
- Temporal for workflow orchestration
- RabbitMQ for messaging
- New Relic for monitoring

## Smart Contract Architecture (Hedera Integration)

### Decentralized Publisher/Reader Pattern
- `contracts/` - Smart contracts implementing domain business logic
- `publishers/` - Services that write to Hedera smart contracts and consensus service
- `readers/` - Services that query smart contracts and listen to consensus events
- Business logic extracted from domain entities into Solidity contracts

### Hedera Integration Patterns
- Use Hedera Consensus Service for event streaming and audit trails
- Smart contracts enforce business rules (e.g., ICANN accreditation for gTLDs)
- Publisher services validate commands and execute contract functions
- Reader services query contracts and maintain local caches for performance
- Events published to consensus topics for real-time updates

Example smart contract pattern:
```solidity
function createAccreditation(string memory tldName, string memory registrarClID) 
    external onlyOwner returns (bool success) {
    // Business logic from entities.Registrar.AccreditFor()
    require(registrars[registrarClID].status == "ok", "Invalid registrar status");
    accreditations[tldName][registrarClID] = true;
    emit AccreditationCreated(tldName, registrarClID, block.timestamp);
}
```

## Common Gotchas
- Repository constructors return concrete types, not interfaces
- Domain entities must call `Validate()` before persistence
- Use `ClIDType`, `DomainName`, `RoidType` instead of raw strings
- Database unique constraint violations return specific error codes (pgconn 23505)
- Tests run sequentially to avoid database conflicts
- Smart contract gas limits require optimization for complex operations
- Hedera account management and private key security are critical
