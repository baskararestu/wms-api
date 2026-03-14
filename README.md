# Warehouse Management System (WMS) API

This project is a high-performance WMS Backend built strictly to handle internal warehouse logic and synchronize order data from external marketplaces (like Shopee and Lazada).

Built with Go, Fiber, GORM, and PostgreSQL.

## 🚨 Project Philosophy & Constraints

- **Backend Only**: This repository handles only the API and background workers.
- **Pragmatic Architecture**: Employs a flat 3-layer architecture (`Handler` -> `Service` -> `Repository`) to maximize development speed without over-engineering.
- **Strict State Machine**: The core logic exclusively manages the internal WMS lifecycle: `READY_TO_PICK` → `PICKING` → `PACKED` → `SHIPPED`.

## 📁 Folder Structure

```text
.
├── cmd/
│   └── api/
│       └── main.go           # Application Entrypoint
├── internal/
│   ├── config/               # Environment Variables & Setup
│   ├── database/             # GORM PostgreSQL Connection
│   ├── handlers/             # HTTP Fiber Controllers
│   ├── services/             # Core Business Logic & State Machines
│   ├── repositories/         # Database Interactions
│   ├── models/               # Entities and DTOs
│   └── integrations/         # External API Clients (OAuth, Webhooks)
├── tests/
│   ├── unit/                 # Mocked fast tests
│   └── integration/          # Live DB tests
├── docker-compose.yml        # Local PostgreSQL Setup
├── go.mod                    # Module (github.com/baskararestu/wms-api)
└── README.md
```

## 🚀 Getting Started

### Prerequisites

- Go 1.22+
- Docker & Docker Compose

### 1. Start the Database

Spin up the local PostgreSQL instance:

```bash
docker-compose up -d
```

### 2. Run the API

```bash
go mod tidy
go run cmd/api/main.go
```

The server will start on port `3000`.

### 3. Verification

Test the health endpoint:

```bash
curl http://localhost:3000/health
```

## 🧪 Testing Strategy

To maintain velocity, tests are strictly divided:

- **Unit Tests (`tests/unit`)**: Focus purely on `internal/services/` and `internal/handlers/`. Repositories and external clients are mocked.
- **Integration Tests (`tests/integration`)**: Focus purely on `internal/repositories/` and database interactions. These require a live database connection.

```bash
# Run Unit Tests
go test ./tests/unit/...

# Run Integration Tests
go test ./tests/integration/...
```
