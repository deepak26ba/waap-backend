# Go E-Commerce Backend (Modular Monolith)

A production-ready e-commerce platform backend built in Golang using a modular monolithic architecture.

---

## 📚 Project Documentation

Comprehensive documentation has been prepared in the [`docs/`](./docs) folder:

1. **[Software Requirements Specification (SRS)](./docs/SRS.md)**: Detailed functional requirements, user stories, personas, NFRs, and milestones.
2. **[Database Design & Schema](./docs/DATABASE_DESIGN.md)**: Complete table definitions, data types, foreign keys, indexes, and Mermaid ER diagram.
3. **[REST API Specification](./docs/API_SPEC.md)**: Endpoint definitions, request/response contracts, authentication flow, and error formats.
4. **[Architecture Decision Records (ADR)](./docs/ADR.md)**: Architectural rationales (Modular Monolith, Go runtime, transactional consistency).

---

## 🏗️ Architecture Overview

The system is organized into decoupled domain packages inside `internal/`:
- **`auth`**: User registration, login, JWT issuance, RBAC.
- **`catalog`**: Categories, products, and variant management.
- **`inventory`**: Stock tracking and row-level reservation locking.
- **`cart`**: Guest and authenticated cart operations.
- **`order`**: Checkout pipeline and order lifecycle state machine.
- **`payment`**: Gateway adapters (Stripe / Razorpay / Mock) and webhooks.

---

## 🚀 Quick Start (Coming Next)

1. Initialize Go module: `go mod init <module-name>`
2. Configure `.env` file
3. Run migrations and start API server
