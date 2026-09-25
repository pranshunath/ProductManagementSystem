# ProductHub — Enterprise Product, Inventory & Order Management Platform

[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Fiber Framework](https://img.shields.io/badge/Fiber-v2-00ACD7?style=flat&logo=fiber)](https://gofiber.io)
[![GORM](https://img.shields.io/badge/ORM-GORM-7952B3?style=flat)](https://gorm.io)
[![MySQL](https://img.shields.io/badge/Database-MySQL-4479A1?style=flat&logo=mysql)](https://mysql.com)
[![gRPC](https://img.shields.io/badge/Service_Mesh-gRPC-244c5a?style=flat&logo=grpc)](https://grpc.io)
[![Redis](https://img.shields.io/badge/Cache-Redis_7-DC382D?style=flat&logo=redis)](https://redis.io)
[![OpenAPI 3.0](https://img.shields.io/badge/OpenAPI-3.0.3-85EA2D?style=flat&logo=swagger)](http://localhost:8081/swagger)

**ProductHub** is a high-performance enterprise portfolio platform engineered in **Go** following **Clean Architecture**. It showcases production software engineering: a Go Fiber HTTP API Gateway, gRPC internal service mesh, MySQL with ACID transactions and pessimistic concurrency locking, Redis Cache-Aside with sliding-window rate limiting and idempotent checkout replay, buffered channel worker pools, an interactive OpenAPI 3.0 Swagger UI, and a glassmorphic customer storefront and admin operations console.

---

## 1. High-Level Architecture

```
                    ┌────────────────────────────────────────────────────────┐
                    │                    CLIENT INTERFACES                   │
                    │   Customer Storefront (/)  |  Admin Console (/admin)   │
                    │         Interactive OpenAPI Swagger UI (/swagger)      │
                    └───────────────────────────┬────────────────────────────┘
                                                │ HTTP / REST (JSON)
                                                ▼
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│                             FIBER HTTP API GATEWAY (:8081)                                 │
│  ├── Global Recover, CORS, Request ID (X-Request-ID) & Structured Request Logger           │
│  ├── JWT Authentication & Role-Based Access Control (CUSTOMER vs ADMIN)                   │
│  ├── Sliding Window Rate Limiting (Redis-backed, X-RateLimit headers)                      │
│  ├── Idempotent Mutation Middleware (RFC-4122 v4 Idempotency-Key locking & replay)          │
│  └── Centralized Error Handling with Standardized API Response Envelope                    │
└───────────────────────────┬──────────────────────────────────┬─────────────────────────────┘
                            │                                  │
          Internal gRPC     │                                  │ Domain Service
          Service Mesh      │                                  │ Invocation
          (:50051)          ▼                                  ▼
┌───────────────────────────────────────┐    ┌───────────────────────────────────────────────┐
│           gRPC SERVICES               │    │             CORE DOMAIN SERVICES              │
│  ├── ProductServiceServer             │    │  ├── UserService (bcrypt, JWT claims)         │
│  │   (GetProduct, ListProducts)       │    │  ├── CategoryService (CRUD & Tree)            │
│  └── InventoryServiceServer           │    │  ├── ProductService (Cache-Aside invalidation)│
│      (CheckStock, DeductStock)        │    │  ├── InventoryService (Pessimistic Locking)   │
│                                       │    │  ├── CartService (Redis & GORM synchronization)│
└───────────────────────────────────────┘    │  └── OrderService (ACID Multi-Table Checkout) │
                                             └───────────────────────┬───────────────────────┘
                                                                     │
                                             Asynchronous Events     │ Dispatches Events
                                             (Buffered Go Channel)   ▼
                                             ┌───────────────────────────────────────────────┐
                                             │             WORKER POOL ENGINE                │
                                             │  ├── 4 Concurrent Goroutines                  │
                                             │  ├── Buffered Channel Queue (Buffer: 100)     │
                                             │  ├── Handlers: ORDER_CREATED, LOW_STOCK_ALERT │
                                             │  └── Graceful Queue Draining on SIGTERM       │
                                             └───────────────────────────────────────────────┘
                                                                     │
                                                                     ▼
                                             ┌───────────────────────────────────────────────┐
                                             │           REPOSITORIES & STORAGE              │
                                             │  ├── Redis 7 (Cache-Aside, Rate Limit, Locks) │
                                             │  └── MySQL 8 (ACID, InnoDB, SELECT FOR UPDATE)│
                                             └───────────────────────────────────────────────┘
```

---

## 2. Platform Engineering Highlights

| Feature | Implementation Details |
| :--- | :--- |
| **Clean Architecture** | Strict separation: `Handler/Controller` → `Service` → `Repository` → `Database`. Zero domain leaks. |
| **Concurrency Protection** | Inventory reservation employs MySQL `SELECT ... FOR UPDATE` row locks, preventing race conditions under high concurrency. |
| **gRPC Service Mesh** | Protocol Buffers (`proto3`) define cross-service contracts on port `50051` with unary RPCs for internal catalog and stock queries. |
| **Redis Cache-Aside** | Product detail reads are cached with 15-minute TTL. Mutations (Update, Restock, Checkout) immediately invalidate affected keys. Automatic fallback to memory cache if Redis is offline. |
| **Sliding Window Rate Limiter**| Redis sorted-set sliding window tracking request velocity with RFC-compliant response headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`). |
| **Idempotent Mutations** | Checkout uses `Idempotency-Key` headers with distributed Redis locking to guarantee zero duplicate charges upon network timeouts. |
| **Background Worker Pool** | Non-blocking dispatcher pushing to a buffered channel consumed by a pool of worker goroutines, complete with backpressure handling and graceful shutdown. |
| **Interactive OpenAPI 3.0** | Comprehensive Swagger specification served at `/swagger` with live API execution and Bearer authentication support. |
| **Dual Frontends** | Customer Storefront (`/`) with live search, cart drawer, and checkout modal; Admin Operations Console (`/admin.html`) with real-time KPI metrics, inventory restocking, order state machine, and worker telemetry. |

---

## 3. Directory Layout

```
producthub/
├── cmd/
│   └── api/
│       └── main.go                 # Gateway bootstrap, gRPC server launch, worker startup, graceful shutdown
├── docs/
│   └── swagger.json                # Complete OpenAPI 3.0.3 specification
├── internal/
│   ├── api/
│   │   ├── router.go               # Fiber gateway routing, middleware chains & error handler
│   │   └── swagger.go              # Swagger UI server and OpenAPI document router
│   ├── cache/
│   │   ├── cache.go                # CacheService abstract interface
│   │   └── redis.go                # RedisCacheService with MemoryCacheService automatic fallback
│   ├── config/
│   │   └── config.go               # Environment loader with defaults
│   ├── controllers/                # HTTP Handlers (Health, Auth, Category, Product, Inventory, Cart, Order)
│   ├── database/                   # GORM MySQL driver, AutoMigrate, and Seeder with demo data
│   ├── grpc/
│   │   ├── clients/                # gRPC client connection pool and wrappers
│   │   └── servers/                # gRPC server implementation (Product & Inventory)
│   ├── middleware/                 # JWT Auth, RBAC, Rate Limiting, Idempotency, Request Tracing
│   ├── models/                     # GORM domain models (User, Category, Product, Inventory, Cart, Order)
│   ├── proto/                      # Protocol Buffer definitions (producthub.proto)
│   ├── repositories/               # Repository interfaces and GORM implementations
│   ├── services/                   # Business domain logic with ACID transactions
│   ├── validators/                 # Payload validation logic
│   └── workers/                    # Goroutine worker pool, event dispatcher, handlers, and metrics
├── pkg/
│   ├── jwt/                        # HMAC-SHA256 JWT token generator and claims parser
│   ├── pb/                         # Generated Protobuf Go code (*.pb.go, *_grpc.pb.go)
│   └── response/                   # Unified JSON envelope: {success, data, pagination, error}
├── tests/                          # 26 automated unit and integration tests
├── web/
│   ├── admin.html                  # Enterprise Operations Console & Admin Dashboard
│   ├── index.html                  # Customer Storefront UI
│   ├── css/
│   │   └── style.css               # Glassmorphic dark design system with neon accents
│   └── js/
│       ├── admin.js                # Admin dashboard controller & live worker telemetry
│       └── app.js                  # Storefront controller, live search, cart drawer & checkout
├── docker-compose.yml              # Isolated MySQL 8 (:3307) and Redis 7 (:6381) containers
└── README.md
```

---

## 4. Port & Service Allocations

To avoid conflicts with existing background services, ProductHub runs on dedicated isolated ports:

| Service | Port | Host Address | Description |
| :--- | :--- | :--- | :--- |
| **HTTP API Gateway** | `8081` | `http://localhost:8081` | Public Fiber REST Gateway & Static UI |
| **Internal gRPC Mesh**| `50051`| `localhost:50051` | High-throughput internal protobuf RPCs |
| **MySQL Database** | `3307` | `localhost:3307` | Docker container database (`producthub_db`) |
| **Redis Cache** | `6381` | `localhost:6381` | Docker container Redis instance |

---

## 5. Quickstart & Verification

### Step 1: Start Isolated Infrastructure (Docker Compose)
```bash
docker compose up -d
```
*Spins up MySQL 8 on `localhost:3307` and Redis 7 on `localhost:6381`.*

### Step 2: Run Automated Test Suite
```bash
go test -count=1 ./tests/...
```
*Executes all 26 unit and integration tests across Auth, Concurrency, gRPC, Redis, Workers, Storefront, Admin Console, and Swagger.*

### Step 3: Run the Application
```bash
go run cmd/api/main.go
```
*Or build the standalone production executable:*
```bash
go build -o bin/producthub.exe ./cmd/api
./bin/producthub.exe
```

---

## 6. Web Interfaces & Portals

### 🏪 Customer Storefront
* **URL**: `http://localhost:8081/`
* **Features**: Live search, category chips, stock status indicators, sliding cart drawer, modal checkout with client-generated `Idempotency-Key` (RFC-4122 v4 UUID), order history tracking, and fast demo login.

### 🛠️ Enterprise Operations Console (Admin Dashboard)
* **URL**: `http://localhost:8081/admin.html`
* **Features**:
  * **Auth Gate**: Enforces `ADMIN` role authentication.
  * **Overview & KPIs**: Live metrics for Total Products, Low Stock Alerts, Total Stock Units, and Worker Queue Depth.
  * **Product Catalog**: Add, edit, or deactivate products with live validation.
  * **Inventory Restock**: 1-Click stock replenishment modal with audit trail notes.
  * **Order State Machine**: State transitions (`PENDING` → `CONFIRMED` → `PROCESSING` → `SHIPPED` → `DELIVERED`, or `CANCELLED`).
  * **Worker Pool Monitor**: Live 3-second polling telemetry displaying active goroutines, buffered channel queue depth gauge, and tasks processed/failed.

### 📚 Interactive OpenAPI 3.0 Documentation (Swagger UI)
* **Interactive UI**: `http://localhost:8081/swagger` (or `http://localhost:8081/docs`)
* **Raw OpenAPI JSON**: `http://localhost:8081/swagger/doc.json`

---

## 7. Demo Accounts & Credentials

The platform auto-seeds standard demo accounts on first database initialization:

| Account Type | Email | Password | Role | Permissions |
| :--- | :--- | :--- | :--- | :--- |
| **Administrator** | `admin@example.com` | `Admin@123` | `ADMIN` | Catalog CRUD, Inventory Restock, Order Transitions, Worker Telemetry |
| **Customer** | `customer@example.com` | `Customer@123` | `CUSTOMER` | Catalog Browsing, Shopping Cart, ACID Checkout, Order Cancellation |

---

## 8. REST API Endpoint Reference

### Health & Observability
* `GET /api/health` — Liveness & database connection probe

### Authentication & Users
* `POST /api/auth/register` — Create new customer account
* `POST /api/auth/login` — Authenticate and receive JWT (Rate limited: 15 req/min)
* `GET /api/auth/me` — Current authenticated user profile (`Bearer JWT`)

### Categories & Products
* `GET /api/categories` — List all product categories
* `POST /api/categories` — Create category (`ADMIN` only)
* `GET /api/products` — List products (with pagination, search, category filter, sorting)
* `GET /api/products/:id` — Get product detail (Redis Cache-Aside with `X-Cache` header)
* `POST /api/products` — Create product (`ADMIN` only)
* `PUT /api/products/:id` — Update product & invalidate cache (`ADMIN` only)
* `DELETE /api/products/:id` — Deactivate product (`ADMIN` only)

### Inventory Management
* `POST /api/inventory/check-stock` — Validate item stock availability
* `POST /api/inventory/restock` — Add stock with audit reason (`ADMIN` only)
* `GET /api/inventory/summary` — Aggregate warehouse stock KPIs (`ADMIN` only)
* `GET /api/inventory/transactions` — Stock audit log (`ADMIN` only)

### Cart & Orders
* `GET /api/cart` — View shopping cart with subtotal & tax
* `POST /api/cart/items` — Add product to cart
* `PUT /api/cart/items/:product_id` — Update item quantity
* `DELETE /api/cart/items/:product_id` — Remove item from cart
* `DELETE /api/cart` — Clear cart
* `GET /api/orders` — List customer's orders
* `POST /api/orders` — Checkout cart into order (Protected by `Idempotency-Key` header)
* `POST /api/orders/:id/cancel` — Cancel order and restore reserved stock

### Admin Operations & Telemetry
* `GET /api/admin/orders` — View all system orders (`ADMIN` only)
* `PUT /api/admin/orders/:id/status` — Transition order status (`ADMIN` only)
* `GET /api/admin/workers/stats` — Live background worker telemetry (`ADMIN` only)

---

## 9. 12-Phase Roadmap Status

* [x] **Phase 1**: Project Initialization, Fiber Gateway, GORM MySQL Connection & Health Probe
* [x] **Phase 2**: Domain Models, GORM Auto-Migrations, Seeder, Repositories & Services
* [x] **Phase 3**: Product & Category REST APIs with Safe Filtering & Validation
* [x] **Phase 4**: JWT Authentication, bcrypt Hashing, Middleware & RBAC
* [x] **Phase 5**: gRPC Protocol Buffers Contracts, Servers, Clients & Mesh Integration
* [x] **Phase 6**: Inventory Management, Atomic Stock Reservation & Concurrency Race Protection
* [x] **Phase 7**: Shopping Cart, Orders, ACID Checkout & Order State Machine
* [x] **Phase 8**: Redis Integration (Cache-Aside, Rate Limiting & Checkout Idempotency)
* [x] **Phase 9**: Background Worker Pool, Goroutines, Buffered Channels & Queue Draining
* [x] **Phase 10**: Responsive Customer Storefront UI with Cart Drawer & Idempotent Checkout
* [x] **Phase 11**: Enterprise Operations Console & Admin Dashboard with Real-Time KPIs
* [x] **Phase 12**: OpenAPI 3.0 Specification, Interactive Swagger UI & Production Polish
