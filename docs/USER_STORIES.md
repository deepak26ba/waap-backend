# Agile User Stories & Acceptance Criteria

This document defines the high-level Epics and User Stories guiding the WAAP backend implementation.

---

## Epic 1: Project Scaffolding & Infrastructure (US-01)
### US-01: System Scaffolding & Environment Setup
- **As a** backend engineer,
- **I want** a modular Go codebase with configuration loading, database connection pooling, and automated schema migrations,
- **So that** subsequent services have a reliable, robust, and observable runtime foundation.

#### Acceptance Criteria:
1. Go 1.26+ module compiles cleanly.
2. 12-Factor environment configuration loaded from `.env` or system environment.
3. Database connection pool managed via `pgxpool` with ping health checks (`/healthz`).
4. Type-safe SQL queries generated via `sqlc`.
5. Database migrations applied automatically at startup.

---

## Epic 2: Security & Middleware Pipeline (US-02)
### US-02: API Security & Standardized Envelope
- **As a** platform developer and API consumer,
- **I want** incoming requests secured by OWASP security headers, CORS whitelisting, rate limiting, and 2MB body limit, with all responses adhering to a unified JSON envelope,
- **So that** the system is resilient against common web vulnerabilities and client integration is predictable.

#### Acceptance Criteria:
1. Security headers present: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `HSTS`.
2. CORS allows configurable origins.
3. IP-based token-bucket rate limiter rejects excess requests with HTTP 429.
4. Request bodies larger than 2MB rejected with HTTP 413.
5. All success and failure responses follow standard `{ "success": bool, "data": ..., "error": ..., "pagination": ... }` envelope.
6. Unhandled panics captured and returned as unified HTTP 500 responses without exposing stack traces.

---

## Epic 3: Identity & Access Management (US-03)
### US-03: User Registration, Authentication & RBAC
- **As a** customer or store administrator,
- **I want** to register an account with a secure password and authenticate using email and password to receive JWT credentials,
- **So that** I can access role-protected resources securely.

#### Acceptance Criteria:
1. `POST /api/v1/auth/register`: Validates email, full name, and minimum 8-character password.
2. Passwords hashed using Bcrypt with cost factor 12.
3. Duplicate registration attempts rejected with HTTP 409 Conflict.
4. `POST /api/v1/auth/login`: Authenticates valid credentials and issues a short-lived access token (15m) and refresh token (7d).
5. Non-existent user login attempts execute dummy Bcrypt verification to prevent timing-based user enumeration.
6. `POST /api/v1/auth/refresh`: Re-issues fresh access tokens given a valid refresh token.
7. Role-Based Access Control (RBAC) middleware enforces `customer` and `admin` permissions.

---

## Epic 4: Customer Profile & Address Management (US-04)
### US-04: User Profile & Shipping Addresses
- **As an** authenticated customer,
- **I want** to view and update my profile and maintain a book of saved shipping addresses,
- **So that** I can reuse my addresses during checkout.

#### Acceptance Criteria:
1. `GET /api/v1/users/me`: Returns the authenticated customer's profile.
2. `PUT /api/v1/users/me`: Allows updating name and phone number.
3. `POST /api/v1/users/addresses`: Adds a new shipping address.
4. `GET /api/v1/users/addresses`: Lists all shipping addresses belonging to the authenticated customer.

---

## Epic 5: Product Catalog & Search (US-05)
### US-05: Product Catalog Browsing & Search
- **As a** shopper,
- **I want** to browse categories and products with pagination, search, and filtering,
- **So that** I can discover items to purchase.

#### Acceptance Criteria:
1. `GET /api/v1/categories`: Lists category hierarchy.
2. `GET /api/v1/products`: Lists active products with pagination and category filtering.
3. `GET /api/v1/products/:id`: Returns product details including variants and stock status.

---

## Epic 6: Shopping Cart (US-06)
### US-06: Shopping Cart Management
- **As a** guest or registered customer,
- **I want** to add, update, and remove items in my shopping cart,
- **So that** I can review my selected items before checkout.

#### Acceptance Criteria:
1. Cart items track product ID, variant ID, and quantity.
2. Adding an existing item increments its quantity.
3. Out-of-stock items cannot be added beyond available inventory.

---

## Epic 7: Orders & Checkout (US-07)
### US-07: Order Placement & Inventory Allocation
- **As a** shopper,
- **I want** to submit an order with a shipping address and payment method,
- **So that** my purchase is reserved and confirmed.

#### Acceptance Criteria:
1. Atomic inventory decrement prevents overselling via database transactions.
2. Orders assigned an initial status of `PENDING`.
3. Customer receives an order confirmation with unique order number.
