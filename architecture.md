# Architecture

This document outlines the high-level architecture of `domain-os`, designed with **Context-Driven Development** and **Domain-Driven Design (DDD)** principles at its core. The system emphasizes separation of concerns, long-term maintainability, and scalability.

## Core Architectural Style

The application follows a **Hexagonal (Ports and Adapters)** architecture, structured around DDD patterns.

### Key Principles

1.  **Entities at the Core**: The heart of the system is the Domain layer, containing `Entities` and `Value Objects` that encapsulate business logic and state. This layer is dependency-free (no imports of database drivers, HTTP frameworks, etc.).
2.  **Abstraction via Interfaces**: Interaction between layers is strictly defined by interfaces (Ports). The Domain layer defines the rules (interfaces), and the outer layers adhere to them.
3.  **Repository Pattern**: Data access is abstracted using the Repository pattern. The Domain layer defines what persistence operations are needed (`DomainRepository` interface), while the Infrastructure layer provides the implementation (`PostgresDomainRepository`).
4.  **Long-Running Processes**: Complex business flows that span time (e.g., domain lifecycle, transfers, escrow imports) are managed using **Temporal** workflows.

---

## Layered Structure

The logical organization maps directly to the `internal/` directory structure:

### 1. Domain Layer (`internal/domain`)
The epicenter of business logic.
-   **Entities** (`internal/domain/entities/`): Plain Go structs representing core business concepts (e.g., `Domain`, `Contact`, `Host`). These contain validations and business rules.
-   **Repositories (Interfaces)** (`internal/domain/repositories/`): Interfaces defining contract operations for persistence. This ensures the domain logic is decoupled from the underlying storage mechanism.
-   **Value Objects**: Immutable types (e.g., `Money`, `Phone`) ensuring type safety and validity.

### 2. Application Layer (`internal/application`)
Orchestrates the domain objects to perform specific user tasks.
-   **Services**: Implements the business use cases (transacting with Repositories).
-   **Workflows & Activities** (`internal/application/workflows`, `activities`): Defines **Temporal** workflows for reliable, fault-tolerant execution of multi-step processes.
-   **Interfaces**: Defines contracts for application services.

### 3. Infrastructure Layer (`internal/infrastructure`)
Provides technical capabilities that support the layers above (Adapters).
-   **Database**: Concrete implementations of Repository interfaces (e.g., using **GORM** or raw SQL).
-   **Temporal Client**: Configuration and connection logic for the Temporal server.
-   **External Adapters**: Clients for third-party services (e.g., payment gateways, external registries).

### 4. Interface Layer (`internal/interface`)
The entry points into the system.
-   **API**: REST/gRPC handlers that accept external requests and delegate to the Application layer.
-   **CLI**: Command-line tools for administration and operations.

---

## Patterns in Use

### Repository Pattern (Strict Enforcement)
Direct database access is strictly forbidden in the Application and Domain layers.
-   **Rule**: All data access must go through a Repository interface defined in `internal/domain/repositories`.
-   **Benefit**: Allows swapping storage backends (e.g., Postgres to Memory for tests) without changing business logic and facilitates unit testing via mocking.

### Temporal for Long-Running Processes
We use Temporal to guarantee the execution of critical, long-duration workflows.
-   **Usage**: Operations like `Escrow Import`, `Domain Transfer`, or async data processing.
-   **Benefit**: Provides out-of-the-box retries, state recovery after failures, and observability for complex flows.

### Dependency Injection
Dependencies (repositories, clients) are injected into services/handlers at startup, ensuring modularity and testability.

### Factory Pattern
Used where complex object creation logic is required, ensuring entities start in a valid state.

## Testing Strategy & Enforcement

Testing is not optional; it is a critical enforcement mechanism for our architecture and business rules.

### Principles
*   **Testable by Design**: The use of interfaces and dependency injection is primarily to facilitate testing. Code that is hard to test is considered architectural debt.
*   **Coverage**: We aim for high test coverage in the Domain and Application layers.
*   **Mocking**: External dependencies (database, third-party APIs) must be mocked in unit tests to ensure speed and isolation.

### Required Tests
1.  **Unit Tests**: Required for all Entities, Value Objects, and Application Services.
    *   *Focus*: Business logic correctness, edge cases, and validation rules.
2.  **Integration Tests**: Required for Repositories and Infrastructure adapters.
    *   *Focus*: Verifying that SQL queries and external client interactions work as expected with real (containerized) dependencies.
3.  **Workflow Tests**: Required for Temporal workflows.
    *   *Focus*: Verifying state transitions, retries, and activity orchestration using the Temporal Test Framework.

> **Rule**: New features or bug fixes must include corresponding tests. Pull Requests without tests will be rejected.
