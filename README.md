# Warehouse Management System (WMS) API

This project is a high-performance WMS Backend built strictly to handle internal warehouse logic and synchronize order data from external marketplaces (like Shopee and Lazada).

Built with Go, Fiber, GORM, and PostgreSQL.

## 🏗️ Architecture

The application implements a clean, flat **3-Layer Domain-Driven Architecture**:

1. **Controller / Handler Layer (`handler.go`)**: Manages HTTP requests (Fiber), payload validation, and standardizing JSON responses.
2. **Service / Usecase Layer (`service.go`)**: Contains the core business logic. It combines multiple domains together (e.g., Auth logic, Order Lifecycle validation, interacting with external Marketplace API).
3. **Repository / Data Layer (`repository.go`)**: Manages directly abstracting PostgreSQL databases via GORM and any Redis caches.

**Key Traits:**

- **Dependency Injection**: Services and repositories rely on Interfaces, enabling trivial unit testing and mocking.
- **Modularity**: Domain folders (`auth`, `orders`, `integrations/marketplace`) cleanly scale up without tightly coupling logic.
- **Statelessness**: Allows WMS instances to seamlessly scale horizontally.

## 🗄️ Database Design

The database schema utilizes **PostgreSQL** explicitly:

- **`uuid` primary keys** (`gen_random_uuid()`) are utilized to ensure global uniqueness and harder-to-guess API endpoints.
- **Users & RefreshTokens**: Separated logic to allow hybrid Token verification without overloading the DB.
- **MarketplaceCredentials**: Stores OAuth/Token states locally bound to a `shop_id`.
- **Orders & OrderItems (1:N)**: A strictly relational mapping of the WMS stock request.
  - Critical indices on `order_sn`, `wms_status`, and `shop_id` optimize filtering and searching capabilities.
  - Core properties like `marketplace`, `buyer_info`, and `raw_marketplace_payload` have been aggressively stripped down to focus purely on fulfillment operation privacy and execution speed.

## 📦 Order Lifecycle

We treat the Warehouse operation as a separate source of truth from the remote Marketplace:

1. **Sync / Ingestion**: Orders are pulled via syncing endpoints/webhooks, triggering an **Upsert**. We track external statuses (`marketplace_status` e.g., "paid", "shipping", "cancelled").
2. **Internal WMS Status Constraints**: Once it hits the database, warehouse workers step the order through an independent internal chain:
   **`Ready to Pickup`** → **`Picking`** → **`Packed`** → **`Shipped`**
   - The API defends against invalid life-cycle steps (e.g., You cannot jump from "Ready to Pickup" to "Shipped").
   - WMS modifications never overwrite Marketplace fields and vice-versa.

## 🔌 Marketplace Integration

Emulates heavily-rate-limited external REST API flows (like Shopee/Tiktok platforms):

- **Oauth-like Connect Flow**: A one-step simulation connecting a shop: `Hit connect` 👉 `Fetch Token` 👉 `Fetch Shop Info` 👉 `Save Shop DB`.
- **HMAC Signatures**: Includes cryptographic secure request signing to external endpoints.
- **Resilience**: The native HTTP REST Client wraps network calls inside a custom Retry-Strategy handling `503` or timeout spikes automatically.

### Connect Shop (One-Step)

```bash
curl -X POST http://localhost:3000/api/integrations/marketplace/shops/connect/start \
	-H 'Content-Type: application/json' \
	-d '{"shop_id":"shopee-123"}'
```

### Get Connected Shop Detail

```bash
curl http://localhost:3000/api/integrations/marketplace/shops/shopee-123
```

## ⚠️ Error Handling

- **Consistent JSON Format**: Standardized `{"code": 40x, "message": "..."}` response wrapped throughout all points in the REST API.
- **Structured Error Mapping**: Domain layers leverage Sentinel errors (`ErrOrderNotFound`, `ErrInvalidStatusTransition`). Handlers translate these to HTTP explicit status codes:
  - `400 Bad Request` -> Request syntax invalid.
  - `404 Not Found` -> Resources (like `order_sn`) missing.
  - `422 Unprocessable Entity` -> Invalid lifecycle jumps.
  - `500 Internal Server Error` -> Generic database failures.
- **Log Masking**: Implements `ZeroLog` struct logging, preserving internal debug traits (like Go-side stack traces) strictly to standard output without leaking internal app secrets to the End-User payload response.

---

## 📁 Folder Structure

```text
.
├── cmd/
│   └── api/
│       └── main.go           # Application Entrypoint
├── internal/
│   ├── config/               # Environment Variables & Setup
│   ├── database/             # GORM PostgreSQL Connection
│   ├── app/                  # Application Wiring & Interface stitching
│   ├── auth/                 # Domain: Authentication & Credentials
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── model.go
│   ├── orders/               # Domain: Orders Processing
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   └── model.go
│   └── integrations/         # Domain: 3rd Party API Clients (Webhooks, OAuth)
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

## 🔐 Auth Flow (Hybrid Refresh Token)

Internal auth now uses:

- Access token (JWT, short-lived)
- Refresh token (opaque token)
- Hybrid persistence: refresh token hash in PostgreSQL + active session cache in Redis

### Login

```bash
curl -X POST http://localhost:3000/api/auth/login \
	-H 'Content-Type: application/json' \
	-d '{"email":"admin@wms.com","password":"admin123"}'
```

### Refresh Access Token

```bash
curl -X POST http://localhost:3000/api/auth/refresh \
	-H 'Content-Type: application/json' \
	-d '{"refresh_token":"<your-refresh-token>"}'
```

### Logout (Revoke Refresh Token)

```bash
curl -X POST http://localhost:3000/api/auth/logout \
	-H 'Content-Type: application/json' \
	-d '{"refresh_token":"<your-refresh-token>"}'
```

## 🔌 Marketplace Integration (Minimal Test Scope)

Flow untuk technical test dibuat sederhana: backend melakukan authorize + token exchange + save credential dalam satu endpoint.

### Connect Shop (One-Step)

```bash
curl -X POST http://localhost:3000/api/integrations/marketplace/shops/connect/start \
	-H 'Content-Type: application/json' \
	-d '{"shop_id":"shopee-123"}'
```

### Get Connected Shop Detail

```bash
curl http://localhost:3000/api/integrations/marketplace/shops/shopee-123
```
