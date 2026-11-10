# Software Requirements Specification (SRS)
## Project: E-Commerce Store Platform (Backend)

**Document Version:** 1.1  
**Status:** Approved for Implementation  
**Architecture Style:** Modular Monolith (Golang 1.22+ & Gin Framework)  
**Database Approach:** PostgreSQL + `sqlc` + `pgx` (Type-safe compile-time SQL)  

---

## 1. Introduction

### 1.1 Purpose
This document specifies the software requirements for a high-performance backend serving an online single-merchant retail shop. The application is built using Go, Gin, and raw SQL queries compiled type-safely via `sqlc` and executed via `pgxpool`.

### 1.2 Project Scope
The platform powers a single-merchant storefront selling **physical products**. Key capabilities include:
- Seamless customer experience: Catalog browsing, search/filter, cart management, both **guest checkout** and **registered customer checkout**, shipping address tracking, and order history.
- Administrative management: Product creation, category hierarchies, variant and inventory controls, order fulfillment, and status tracking.
- Hardened security: Granular CORS rules, OWASP security headers, IP rate limiting, Bcrypt password hashing, and input validation.

### 1.3 Definitions & Conventions
- **Single Store**: One merchant manages catalog and fulfillment (not a multi-vendor marketplace).
- **Physical Goods**: Products require stock tracking, weight/dimensions (optional), shipping addresses, and delivery status tracking.
- **Guest Checkout**: Customers can place an order by providing their email and shipping address without needing an account; optionally creating an account post-purchase.
- **sqlc**: SQL compiler that generates type-safe Go code from raw SQL queries.

---

## 2. Overall Description

### 2.1 System Architecture
```
+-------------------------------------------------------+
|                 Client Frontends                      |
|         (Web Storefront, Mobile App, Admin UI)        |
+---------------------------+---------------------------+
                            | HTTPS / REST JSON
                            v
+-------------------------------------------------------+
|                    Gin HTTP Router                    |
|  +-------------------------------------------------+  |
|  | Middleware Pipeline:                            |  |
|  | - CORS Config (Restricted Origins)              |  |
|  | - OWASP Security Headers (CSP, HSTS, NoSniff)   |  |
|  | - Rate Limiter (IP-based token bucket)          |  |
|  | - Request Body Size Limiter (Max 2MB)           |  |
|  | - JWT Authentication & RBAC Filter              |  |
|  +-------------------------------------------------+  |
+---------------------------+---------------------------+
|               Modular Domain Services                 |
|  +-------------+  +-------------+  +---------------+  |
|  | Auth/User   |  |   Catalog   |  | Cart & Orders |  |
|  +-------------+  +-------------+  +---------------+  |
|  |  Payments   |  |  Inventory  |  | Notifications |  |
|  +-------------+  +-------------+  +---------------+  |
+---------------------------+---------------------------+
                            | Type-safe queries (sqlc)
                            v
+-------------------------------------------------------+
|          PostgreSQL Database (pgxpool)                |
|       (ACID Transactions, Row-level Locking)          |
+-------------------------------------------------------+
```

### 2.2 User Personas & Roles
1. **Guest Shopper**:
   - Browses catalog, adds products to cart (session/cookie-tracked).
   - Can directly checkout by supplying shipping details and email address.
2. **Registered Customer**:
   - All guest capabilities + saved shipping addresses, order tracking dashboard, and repeat checkout.
3. **Store Administrator**:
   - Manages product listings, variants, stock levels, and order fulfillment states (`PROCESSING`, `SHIPPED`, `DELIVERED`).

---

## 3. Functional Requirements

### 3.1 Module: Identity & Access Management (IAM)
- **REQ-IAM-01**: Registration with email, full name, and password.
- **REQ-IAM-02**: Passwords securely hashed with Bcrypt (cost 12) or Argon2id.
- **REQ-IAM-03**: JWT token generation (short-lived access token, 15m; refresh token, 7d).
- **REQ-IAM-04**: Role-Based Access Control middleware distinguishing `customer` and `admin`.
- **REQ-IAM-05**: Customer address book (multiple addresses, designate default).

### 3.2 Module: Product Catalog & Physical Variants
- **REQ-CAT-01**: Hierarchical category tree (Parent -> Subcategories).
- **REQ-CAT-02**: Products have title, slug, description, base price, brand, status (`draft`, `published`, `archived`), and image gallery.
- **REQ-CAT-03**: Physical product variants with unique SKU, variant title (e.g., "Size: M, Color: Black"), weight/dimensions (for shipping calculation), price override, and available stock.
- **REQ-CAT-04**: Public search with pagination, filtering (category, price range, stock status), and sorting (price, popularity, newest).
- **REQ-CAT-05**: Admin endpoints for complete CRUD over categories, products, and variants.

### 3.3 Module: Inventory Management
- **REQ-INV-01**: Atomic stock count per variant.
- **REQ-INV-02**: Concurrency safety: during checkout, rows are locked (`SELECT FOR UPDATE`) to prevent overselling.
- **REQ-INV-03**: Immediate stock reduction upon successful order initiation or checkout reservation.

### 3.4 Module: Shopping Cart
- **REQ-CART-01**: Guest cart identified by client UUID (`X-Cart-ID` or cookie).
- **REQ-CART-02**: Cart merge: when a guest logs in, their guest cart items automatically merge into their customer cart.
- **REQ-CART-03**: Cart operations: add variant, update quantity, remove item, clear cart.
- **REQ-CART-04**: Dynamic price & availability validation during cart retrieval.

### 3.5 Module: Checkout & Order Processing
- **REQ-ORD-01**: **Guest & Customer Checkout**:
  - Registered customers can checkout using saved address ID or new address.
  - Guests checkout by providing `guest_email`, recipient name, phone, and complete shipping address.
- **REQ-ORD-02**: Price integrity: Subtotal, shipping fee, tax, and grand total calculated strictly server-side.
- **REQ-ORD-03**: Address snapshotting: Full shipping address copied as a JSON snapshot onto the order record.
- **REQ-ORD-04**: Order Lifecycle State Machine:
  `PENDING_PAYMENT` -> `PAID` -> `PROCESSING` -> `SHIPPED` -> `DELIVERED` | `CANCELLED`
- **REQ-ORD-05**: Tracking info: Admins can update shipment tracking carrier and tracking number upon dispatch.

### 3.6 Module: Payments
- **REQ-PAY-01**: Payment Gateway Intent (Stripe / Razorpay / Mock gateway for testing).
- **REQ-PAY-02**: Secure asynchronous webhook handler with cryptographic signature verification.
- **REQ-PAY-03**: Idempotent webhook processing to avoid double-crediting orders.

---

## 4. Security Requirements

### 4.1 CORS Policy
- Configurable via `.env` (`CORS_ALLOWED_ORIGINS`).
- Development: `http://localhost:3000`, `http://localhost:5173`.
- Production: Exact storefront domains.
- Allowed Methods: `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `OPTIONS`.
- Allowed Headers: `Origin`, `Content-Type`, `Accept`, `Authorization`, `X-Cart-ID`.
- Credentials support enabled (`AllowCredentials: true`).

### 4.2 OWASP Security Headers Middleware
- `X-Frame-Options: DENY` (anti-clickjacking).
- `X-Content-Type-Options: nosniff` (anti-MIME sniffing).
- `X-XSS-Protection: 1; mode=block`.
- `Strict-Transport-Security: max-age=31536000; includeSubDomains` (HSTS).
- `Referrer-Policy: strict-origin-when-cross-origin`.

### 4.3 Rate Limiting
- Sensitive Endpoints (`/api/v1/auth/login`, `/api/v1/auth/register`): Max 5 requests/minute per client IP.
- Public Endpoints: Max 100 requests/minute per client IP to safeguard against scraping/DDoS.

### 4.4 Payload Protection & Injection Defense
- Request body size capped at 2MB (`http.MaxBytesReader`) to mitigate memory-exhaustion attacks.
- SQL Injection eliminated by 100% parameterized queries via `sqlc` and `pgx`.
- Input validation on all incoming DTOs using Gin's `binding:"required,email,min=8"`.
- Timing attack prevention for webhook secrets using `crypto/subtle.ConstantTimeCompare`.

---

## 5. Technology Stack Summary

- **Language:** Go (1.22+)
- **HTTP Framework:** Gin (`github.com/gin-gonic/gin`)
- **Database:** PostgreSQL (with `jackc/pgx/v5` connection pool)
- **Query Compiler:** `sqlc` (generates type-safe Go code from pure SQL)
- **Migrations:** `golang-migrate/migrate` (versioned `.up.sql` and `.down.sql`)
- **Authentication:** `golang-jwt/jwt/v5` + `golang.org/x/crypto/bcrypt`
- **Configuration:** `spf13/viper` or clean `caarlos0/env` / `.env`
