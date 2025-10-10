# domain-os# domain-os



Domain (Registry for now) Operating System (DOS)Domain (Registry for now) Operating System (DOS)



## Why? # Why?

  

The registry backend business operates in a high-volume low-margin setup, therefore a highly automated, robust, adaptable system that is cost efficient to operate is required. The current legacy systems are often monolithic, hard to maintain, and difficult to adapt to new requirements. They are often built on outdated technology stacks that are not well suited for the current and future needs of the industry. It should be flexible in terms of policy, infrastructure and quick to evolve and integrate.  The registry backend business operates in a high-volume low-margin setup, therefore a highly automated, robust, adaptable system that is cost efficient to operate is required. 

 The current legacy systems are often monolithic, hard to maintain, and difficult to adapt to new requirements.

## How? They are often built on outdated technology stacks that are not well suited for the current and future needs of the industry.

 It should be flexible in terms of policy, infrastructure and quick to evolve and integrate. 

By taking the accumulated knowledge of working in the industry and building a system that is modular, flexible, and easy to operate.

 # How?

Offer clear context, a blazing fast developer experience and feedback loop and lots of automated testing for safety. I've tried to focus on making this system easy to operate with a low overhead. While it is not yet optimized, it has testing, automation, visibility, and an open architecture and documentation. This way, when optimization is needed, it will be straightforward as we will have data and visibility to evolve the system quickly in the desired direction. The data model aligns with the RFCs and can easily be evolved, optimized or even be implemented with a different database because its de-coupled from the business logic.

By taking the accumulated knowledge of working in the industry and building a system that is modular, flexible, and easy to operate.

## What?



A registry system with core functionality that you can easily integrate into your existing service stack or build on top of. Offer clear context, a blazing fast developer experience and feedback loop and lots of automated testing for safety.

 I've tried to focus on making this system easy to operate with a low overhead. While it is not yet optimized, it has testing, automation, visibility, and an open architecture and documentation.

--- This way, when optimization is needed, it will be staightforward as we will have data and visibility to evolve the system quickly in the desired direction.

 The data model aligns with the RFCs and can easily be evolved, optimized or even be implemented with a different database because its de-coupled from the business logic.

# Running the app

  # What?

## Requirements  A registry system with core functionality that you can easily integrate into your existing service stack or build on top of



* Docker Desktop installed# Running the app

* [Doppler CLI](https://docs.doppler.com/docs/install-cli) for secrets management (development)

* Go 1.21+ (for local development)## Requirements

* Node.js 18+ (for frontend development)

* An API client (Postman, Insomnia, or use the Swagger UI)* You do need docker desktop installed 

* An API client, I’m using Postman in this video

## Quick Start

## Running the app

### Using Makefile (Recommended)

### Docker Compose using pre-built images

The project includes a comprehensive Makefile that simplifies all common tasks. View all available commands:

Video walkthrough: https://youtu.be/pobt7sm7ixw

```bash

make help* Download this zip file containing a docker-compose file and a basic .env file: https://drive.google.com/file/d/1dbsusOJ2g1FPLJ0rUBkYc3Av8PABgNab/view?usp=sharing

```* Unzip the file and open a terminal in the folder

* Run `BRANCH=latest docker compose --profile essential up`

**Start development environment:**

```bash* Open http://localhost:8080/swagger/index.html and download the Postman collection (doc.json)

# Start essential services (db, redis, epp-server, admin-api)* Import the Postman collection into your Postman client

make dev* Configure the environment variables in Postman (baseUrl and token) to match the .env file 

* Send your first request to the API

# Start all services including workers

make dev-fullIf you want to populate the system, start by creating a Registry Operator, then a TLD, Setup a Phase to enable the TLD, create Registrars, and then create Domains...



# Start frontend development server (in a separate terminal)

make dev-frontend


# Stop all services
make stop
```

**Running tests:**
```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Generate coverage report
make test-coverage
```

**Building and deploying:**
```bash
# Build all Docker images
make build-all

# Push images to Docker Hub
make push-all
```

**Other useful commands:**
```bash
# View logs
make dev-logs

# Restart services
make restart

# Clean up containers and volumes
make clean

# Access database shell
make shell-db
```

### Docker Compose (Manual)

If you prefer to use Docker Compose directly:

```bash
# Start essential services
export BRANCH=$(git branch --show-current)
doppler run -- docker compose --profile essential up -d

# View logs
docker compose logs -f

# Stop services
doppler run -- docker compose --profile essential down
```

### Using Pre-built Images

Video walkthrough: https://youtu.be/pobt7sm7ixw

1. Download [this zip file](https://drive.google.com/file/d/1dbsusOJ2g1FPLJ0rUBkYc3Av8PABgNab/view?usp=sharing) containing a docker-compose file and a basic .env file
2. Unzip the file and open a terminal in the folder
3. Run: `BRANCH=latest docker compose --profile essential up`

---

## Using the API

1. Open http://localhost:8080/swagger/index.html to view the API documentation
2. Download the Postman collection (doc.json) from the Swagger UI
3. Import the collection into Postman
4. Configure the environment variables in Postman (baseUrl and token) to match your .env file
5. Send your first request!

**Recommended workflow for populating the system:**
1. Create a Registry Operator
2. Create a TLD
3. Set up a Phase to enable the TLD
4. Create Registrars
5. Create Domains

---

## Frontend Admin Dashboard

The project includes a Next.js-based admin dashboard for managing registry operators and other entities.

```bash
# Install dependencies
cd frontend && npm install

# Start development server
npm run dev
# or from root: make dev-frontend

# Access at http://localhost:3000
```

**Features:**
- Registry Operator CRUD operations
- Dashboard with statistics
- Modern UI with Tailwind CSS and shadcn/ui components
- Type-safe API client with React Query

---

## Development Workflow

### Common Tasks

**Start everything for development:**
```bash
# Terminal 1: Start backend services
make dev

# Terminal 2: Start frontend
make dev-frontend
```

**Run tests before committing:**
```bash
make test
make test-integration
```

**Build and test locally:**
```bash
make build
make dev-build  # Rebuild and restart services
```

### Project Structure

```
domain-os/
├── cmd/              # Application entry points
│   ├── api/         # API servers (admin, whois)
│   ├── cli/         # CLI tools
│   ├── epp/         # EPP server
│   └── workers/     # Background workers
├── internal/        # Internal packages
│   ├── application/ # Application layer (services, commands, queries)
│   ├── domain/      # Domain layer (entities, repositories)
│   ├── infrastructure/ # Infrastructure layer (db, messaging)
│   └── interface/   # Interface layer (REST controllers)
├── frontend/        # Next.js admin dashboard
├── deploy/          # Deployment configurations
├── docs/            # Documentation
└── test/            # Integration and E2E tests
```

### Environment Variables

The project uses [Doppler](https://www.doppler.com/) for secrets management. For local development without Doppler, create a `.env` file based on `example.env`.

---

## Testing

The project includes comprehensive testing at multiple levels:

- **Unit Tests**: `make test` - Fast, isolated tests with mocked dependencies
- **Integration Tests**: `make test-integration` - Tests with real database and services
- **Coverage Reports**: `make test-coverage` - Generate HTML coverage reports
- **EPP Tests**: `make test-epp` - EPP-specific functionality tests

---

## Building and Deployment

### Docker Images

The project builds several Docker images:

- `geapex/domain-os` - Main admin API
- `geapex/epp-server` - EPP server
- `geapex/epp-client-api` - WHOIS/EPP client API

**Build locally:**
```bash
make build-all
```

**Build and push to registry:**
```bash
make push-all
```

### CI/CD

Integration tests run automatically in CI using `docker-compose-ci.yml`. The Makefile provides `test-integration` target that replicates this locally.

---

## Troubleshooting

**Port conflicts:**
```bash
# Check what's using ports
lsof -ti:8080  # Admin API
lsof -ti:3000  # Frontend
lsof -ti:5432  # PostgreSQL

# Kill processes if needed
make stop
```

**Database issues:**
```bash
# Reset database
make db-reset

# Access database shell
make shell-db
```

**Container issues:**
```bash
# Clean everything and start fresh
make clean
make dev
```

**View logs:**
```bash
# All services
make dev-logs

# Specific service
make logs-api
make logs-epp
make logs-db
```

---

## Contributing

1. Create a feature branch from `main`
2. Make your changes
3. Run tests: `make test && make test-integration`
4. Format code: `make fmt`
5. Submit a pull request

---

## License

See LICENSE file for details.
