# ProductHub — Product, Inventory & Order Management Platform

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Fiber Framework](https://img.shields.io/badge/Fiber-v2-00ACD7?style=flat&logo=fiber)](https://gofiber.io)
[![GORM](https://img.shields.io/badge/ORM-GORM-7952B3?style=flat)](https://gorm.io)
[![MySQL](https://img.shields.io/badge/Database-MySQL-4479A1?style=flat&logo=mysql)](https://mysql.com)

ProductHub is an enterprise-grade backend and portfolio platform built in Go demonstrating production software engineering practices: Clean Architecture, gRPC internal service mesh, Fiber HTTP API gateway, MySQL with ACID transactions and row-locking concurrency control, Redis caching & rate limiting, background worker pools, and a modern responsive dashboard.

---

## 1. High-Level Architecture

```
[ Frontend Dashboard ]
        │
        │ HTTP / REST (JSON)
        ▼
[ Go Fiber API Gateway (:8081) ]
  ├── CORS & Security
  ├── JWT Auth & Role Middleware
  ├── Rate Limiting & Idempotency Key (Redis)
  ├── Structured Logging & Request Tracing (X-Request-ID)
  └── Centralized Error Handling
        │
        │ Internal Service Communication (gRPC & Direct Contracts)
        ▼
[ Core Domain Services ]
  ├── Auth & User Service (bcrypt, JWT claims)
  ├── Product Service (Search, Category filtering, Redis Caching)
  ├── Inventory Service (Stock allocation, Row-level locking, Audit trails)
  └── Order Service (Multi-entity ACID transactions, State machine)
        │
        ▼
[ Repository Layer (GORM) ]
        │
        ▼
[ MySQL Database (ACID / InnoDB) ]
```

---

## 2. Directory Structure

```
producthub/
├── cmd/
│   └── api/
│       └── main.go                 # Main application entry point & graceful shutdown
├── internal/
│   ├── api/
│   │   └── router.go               # Fiber router, middleware registration, centralized error handler
│   ├── config/
│   │   └── config.go               # Environment configuration loader with defaults
│   ├── controllers/
│   │   └── health_controller.go    # Liveness & readiness probes
│   └── database/
│       └── mysql.go                # GORM MySQL driver, pool config, auto-db creation
├── pkg/
│   └── response/
│       └── response.go             # Standardized API response format ({success, data, pagination, error})
├── tests/
│   └── health_test.go              # Health & routing integration tests
├── .env.example                    # Template environment variables
├── .env                            # Active environment configuration
├── .gitignore                      # Git ignore file
├── docker-compose.yml              # Local isolated MySQL 8 & Redis 7 services
├── go.mod                          # Go module definition
├── go.sum                          # Go dependency checksums
└── README.md                       # Project documentation
```

---

## 3. Standardized API Response Format

All endpoints return a predictable JSON contract:

### Success Response
```json
{
  "success": true,
  "data": { ... }
}
```

### Paginated List Response
```json
{
  "success": true,
  "data": [ ... ],
  "pagination": {
    "page": 1,
    "limit": 10,
    "total": 100,
    "total_pages": 10
  }
}
```

### Standard Error Response
```json
{
  "success": false,
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "Product with ID 42 was not found",
    "details": null
  }
}
```

---

## 4. Phase 1 Verification & Quickstart

### Prerequisites
* Go 1.22+ (Installed: `1.27.1`)
* Optional: Docker & Docker Compose (or local MySQL)

### Step 1: Start Database (Optional via Docker Compose)
If you do not have a local MySQL running or wish to isolate ProductHub from other projects:
```bash
docker compose up -d
```
*(Starts MySQL on port `3307` and Redis on port `6381` with root password `root`)*

### Step 2: Run Tests
```bash
go test -v ./...
```

### Step 3: Run the API Gateway
```bash
go run cmd/api/main.go
```
The gateway will boot on port `8081` (configurable in `.env` via `APP_PORT`).

### Step 4: Verify Health Endpoint
```bash
curl http://localhost:8081/api/health
```
Sample response:
```json
{
  "success": true,
  "data": {
    "database": "connected",
    "db_latency": "1.24ms",
    "service": "ProductHub API",
    "status": "UP",
    "timestamp": "2026-09-25T11:51:35Z",
    "uptime": "12s",
    "version": "1.0.0"
  }
}
```
