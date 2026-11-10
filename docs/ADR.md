# Architecture Decision Records (ADR)

## ADR-001: Architecture Style - Modular Monolith
- **Status:** Accepted
- **Context:** The system needs transactional consistency for stock/orders, high performance, and rapid delivery without premature distributed systems complexity.
- **Decision:** Build a single deployable Go binary organized by domain modules (`auth`, `catalog`, `inventory`, `cart`, `order`, `payment`).
- **Consequences:** ACID transactions across tables, simple single-binary deployment, clear module separation via interfaces.

---

## ADR-002: Language & Framework Selection (Go + Gin)
- **Status:** Accepted
- **Context:** High throughput, standard REST APIs, middleware flexibility.
- **Decision:** Go 1.22+ with **Gin** web framework (`github.com/gin-gonic/gin`).
- **Consequences:**
  - Fast execution with low memory footprint (< 40MB).
  - Clean middleware ecosystem (CORS, Recovery, Security Headers, Custom Rate Limiter).
  - Idiomatic struct tag validation via `go-playground/validator`.

---

## ADR-003: Database Layer (PostgreSQL + `sqlc` + `pgx/v5`)
- **Status:** Accepted
- **Context:** E-commerce checkout requires raw SQL performance, atomic updates, row-level concurrency control (`SELECT FOR UPDATE`), and type-safety without the magic/reflection overhead of traditional ORMs.
- **Decision:** PostgreSQL with **`sqlc`** to generate 100% type-safe Go code from plain `.sql` files, running on **`pgxpool`** (`github.com/jackc/pgx/v5`).
- **Consequences:**
  - Zero reflection overhead at runtime.
  - Compile-time SQL validation and schema alignment.
  - Granular control over transaction isolation levels during checkout.

---

## ADR-004: Checkout Model (Guest & Customer Checkout)
- **Status:** Accepted
- **Context:** Reduce cart abandonment while allowing repeat customers to save shipping addresses.
- **Decision:** Support both:
  - Registered customers checkout with their account.
  - Guests checkout with their email, phone, and shipping address without creating a password.
  - Cart merge automatically migrates guest cart items to the user's cart upon registration or login.
- **Consequences:**
  - `orders.user_id` is nullable.
  - `orders.customer_email` and `orders.shipping_address_snapshot` guarantee all fulfillment data is preserved regardless of account status.

---

## ADR-005: Security Hardening (OWASP, CORS, Rate Limiting)
- **Status:** Accepted
- **Context:** Financial and user data protection against common web threats.
- **Decision:**
  - **CORS**: Strict domain whitelist via environment config.
  - **OWASP Headers**: Inject `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `HSTS`, `Referrer-Policy`.
  - **Rate Limiting**: Token bucket rate limiter for auth routes (5 req/min) and general endpoints (100 req/min).
  - **Payload Limits**: Restrict request bodies to 2MB using `http.MaxBytesReader`.
  - **Password Security**: Bcrypt with cost 12 or Argon2id.
