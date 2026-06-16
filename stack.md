# Tech Stack

This document details the technologies used in `domain-os`.

## Backend

The backend is a robust service designed for high reliability and domain complexity.

-   **Language**: **Go** (Golang) - Chosen for performance, concurrency, and type safety.
-   **Architecture**: Domain-Driven Design (DDD) / Hexagonal.
-   **Web Framework**: **Gin** - High-performance HTTP web framework.
-   **Database**: **PostgreSQL** - Primary relational database.
-   **ORM**: **GORM** - Used for database interactions, mapping entities to tables.
-   **Workflow Engine**: **Temporal** - Handles long-running, reliable distributed workflows (e.g., async imports, domain lifecycle).
-   **Message Queue**: **RabbitMQ** - For event-driven messaging and decoupling services.
-   **Caching**: **Redis** - High-performance caching layer.
-   **Object Storage**: **MinIO** - S3-compatible object storage for documents/exports.
-   **CLI**: **Cobra** - Library for creating powerful modern CLI interactions.

## Frontend

The frontend is a modern, responsive web application.

-   **Framework**: **Next.js 15** (App Router) - Server-side rendering and static generation.
-   **Library**: **React 19** - Component-based UI library.
-   **Language**: **TypeScript** - Static typing for robust frontend code.
-   **Styling**: **Tailwind CSS v4** - Utility-first CSS framework.
-   **UI Components**: **Radix UI** primitives (base for Shadcn-like components).
-   **State Management**: **Zustand** - Minimalist state management.
-   **Data Fetching**: **TanStack Query** (React Query) - Server state management.
-   **Forms**: **React Hook Form** + **Zod** (Validation).
-   **Authentication**: **Auth0** - Identity management.
-   **Testing**: **Vitest** + **React Testing Library**.

## Infrastructure & DevOps

-   **Containerization**: **Docker** - Containerizing backend, frontend, and dependencies.
-   **Orchestration (Dev)**: **Tilt** - Kubernetes/Docker Compose microservices development environment.
-   **Task Runner**: **Makefile** - Automating build, test, and run commands.
-   **Testing**: **Testify** (Go backend), **Ginkgo** (BDD style tests).

## Key Concepts

-   **Strict Separation**: The frontend and backend are decoupled, communicating via REST APIs.
-   **Interfaces**: The backend relies heavily on interfaces to separate `domain` logic from `infrastructure` implementation.
