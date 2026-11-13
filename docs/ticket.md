 Project Implementation Tickets & Roadmap
See **[USER_STORIES.md](./USER_STORIES.md)** for full Agile user story definitions, business context, and acceptance criteria.
**Legend:**
- 🟩 `DONE`: Completed and verified
- 🟦 `IN PROGRESS`: Currently being worked on
---
## Phase 1: Project Scaffolding & Infrastructure
## Phase 1: Project Scaffolding & Infrastructure (Epic 1: US-01)
- [ ] **TICKET-01**: **Project Scaffolding & Environment Setup**
- [ ] **TICKET-01**: **Project Scaffolding & Environment Setup** `[US-01]`
  - **Description**: Initialize the Go module (`go.mod`), setup clean directory layout (`cmd/`, `internal/`, `pkg/`), create `.env.example`, and write `docker-compose.yml` for local PostgreSQL 16.
  - **Acceptance Criteria**: Go compiles cleanly, Docker Compose starts PostgreSQL successfully.
- [ ] **TICKET-02**: **Database Schema Migrations**
- [ ] **TICKET-02**: **Database Schema Migrations** `[US-01]`
  - **Description**: Create initial migration files (`000001_init_schema.up.sql` and `down.sql`) defining all 9 tables from `docs/DATABASE_DESIGN.md`.
  - **Acceptance Criteria**: Migrations can be applied and rolled back without errors.
- [ ] **TICKET-03**: **Database Connection Pool & `sqlc` Setup**
- [ ] **TICKET-03**: **Database Connection Pool & `sqlc` Setup** `[US-01]`
  - **Description**: Configure `pgxpool` for PostgreSQL connection management, configure `sqlc.yaml`, write initial database queries, and provide `/healthz` probe.
  - **Acceptance Criteria**: Server connects to DB, `sqlc generate` succeeds, and `/healthz` returns `{"status": "healthy"}`.
---
## Phase 2: Security & Middleware Pipeline
## Phase 2: Security & Middleware Pipeline (Epic 2: US-02)
- [ ] **TICKET-04**: **Security Middleware (OWASP, CORS, Limits)**
- [ ] **TICKET-04**: **Security Middleware (OWASP, CORS, Limits)** `[US-02]`
  - **Description**: Implement Gin middleware for CORS whitelist, OWASP security headers (`X-Frame-Options`, `nosniff`, `HSTS`), request body size limiter (2MB), and IP-based rate limiting.
  - **Acceptance Criteria**: Security headers present on responses, oversized bodies rejected with 413, rate limits enforced.
- [ ] **TICKET-05**: **Standard Response Envelope & Error Handling**
- [ ] **TICKET-05**: **Standard Response Envelope & Error Handling** `[US-02]`
  - **Description**: Standardize API response format `{success, data, error, pagination}` and Gin global error recovery handler.
  - **Acceptance Criteria**: Consistent JSON output across success and failure conditions.
---
## Phase 3: Identity & Access Management (IAM)
## Phase 3: Identity & Access Management (IAM) (Epic 3: US-03, US-04)
- [ ] **TICKET-06**: **User Registration & Bcrypt Hashing**
- [ ] **TICKET-06**: **User Registration & Bcrypt Hashing** `[US-03]`
  - **Description**: Endpoint `POST /api/v1/auth/register` with input validation (email format, password min length 8) and Bcrypt (cost 12) hashing.
  - **Acceptance Criteria**: User saved to DB, duplicate emails rejected with 409 Conflict.
- [ ] **TICKET-07**: **User Login & JWT Generation**
- [ ] **TICKET-07**: **User Login & JWT Generation** `[US-03]`
  - **Description**: Endpoint `POST /api/v1/auth/login` returning short-lived JWT access token and refresh token with user claims (`user_id`, `role`, `email`).
  - **Acceptance Criteria**: Valid credentials return JWT, invalid credentials return 401 Unauthorized.