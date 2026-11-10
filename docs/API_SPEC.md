# REST API Specification

**Base URL:** `/api/v1`  
**Data Format:** JSON (`application/json`)  
**Authentication:** Bearer Token (JWT in `Authorization: Bearer <token>`)

---

## 1. Authentication & User APIs

### 1.1 Register Customer
- **Endpoint:** `POST /auth/register`
- **Access:** Public
- **Request Body:**
  ```json
  {
    "full_name": "John Doe",
    "email": "john@example.com",
    "password": "StrongPassword123!"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "success": true,
    "message": "User registered successfully",
    "data": {
      "user": {
        "id": 1,
        "email": "john@example.com",
        "full_name": "John Doe",
        "role": "customer"
      },
      "token": "eyJhbGciOi..."
    }
  }
  ```

### 1.2 Login
- **Endpoint:** `POST /auth/login`
- **Access:** Public
- **Request Body:**
  ```json
  {
    "email": "john@example.com",
    "password": "StrongPassword123!"
  }
  ```
- **Response (200 OK):**
  ```json
  {
    "success": true,
    "data": {
      "user": {
        "id": 1,
        "email": "john@example.com",
        "full_name": "John Doe",
        "role": "customer"
      },
      "token": "eyJhbGciOi..."
    }
  }
  ```

### 1.3 Get Current Profile
- **Endpoint:** `GET /users/me`
- **Access:** Authenticated (`Bearer <token>`)
- **Response (200 OK):** Returns current user details and saved shipping addresses.

---

## 2. Product & Category Catalog APIs

### 2.1 List Categories
- **Endpoint:** `GET /categories`
- **Access:** Public
- **Query Params:** `parent_id` (optional filter)
- **Response (200 OK):** Array of category objects.

### 2.2 Create Category (Admin Only)
- **Endpoint:** `POST /admin/categories`
- **Access:** Admin (`Bearer <token>`)
- **Request Body:**
  ```json
  {
    "name": "Electronics",
    "parent_id": null,
    "description": "Gadgets, smartphones, and accessories"
  }
  ```

### 2.3 List Products (Search & Filter)
- **Endpoint:** `GET /products`
- **Access:** Public
- **Query Params:**
  - `page` (default 1)
  - `limit` (default 20, max 100)
  - `category_id` (optional)
  - `search` (keyword search on title and description)
  - `min_price` / `max_price`
  - `sort` (`newest`, `price_asc`, `price_desc`)
- **Response (200 OK):**
  ```json
  {
    "success": true,
    "data": [
      {
        "id": 10,
        "title": "Wireless Noise Cancelling Headphones",
        "slug": "wireless-noise-cancelling-headphones",
        "base_price": 149.99,
        "category": { "id": 2, "name": "Audio" },
        "images": ["https://cdn.example.com/img1.jpg"],
        "in_stock": true
      }
    ],
    "pagination": {
      "current_page": 1,
      "total_pages": 4,
      "total_items": 72
    }
  }
  ```

### 2.4 Get Product by Slug / ID
- **Endpoint:** `GET /products/:id` or `GET /products/slug/:slug`
- **Access:** Public
- **Response (200 OK):** Detailed product with all variants, specifications, images, and current stock status.

### 2.5 Create Product (Admin Only)
- **Endpoint:** `POST /admin/products`
- **Access:** Admin
- **Request Body:**
  ```json
  {
    "category_id": 2,
    "title": "Ergonomic Mechanical Keyboard",
    "description": "Compact 75% hot-swappable mechanical keyboard",
    "base_price": 89.99,
    "variants": [
      {
        "sku": "KB-75-RED",
        "name": "Red Switch",
        "price_override": null,
        "stock_quantity": 40,
        "attributes": { "switch": "Red Linear" }
      },
      {
        "sku": "KB-75-BLUE",
        "name": "Blue Switch",
        "price_override": 94.99,
        "stock_quantity": 25,
        "attributes": { "switch": "Blue Clicky" }
      }
    ]
  }
  ```

---

## 3. Cart APIs

### 3.1 Get Cart
- **Endpoint:** `GET /cart`
- **Access:** Public (uses `X-Cart-Id` header for guests) or Authenticated (`Bearer <token>`)
- **Response (200 OK):** Current line items, item prices, quantities, and calculated subtotal.

### 3.2 Add Item to Cart
- **Endpoint:** `POST /cart/items`
- **Request Body:**
  ```json
  {
    "variant_id": 12,
    "quantity": 2
  }
  ```

### 3.3 Update Item Quantity / Remove
- **Endpoint:** `PATCH /cart/items/:id` (`{"quantity": 3}`)
- **Endpoint:** `DELETE /cart/items/:id`

---

## 4. Checkout & Order APIs

### 4.1 Create Order (Checkout)
- **Endpoint:** `POST /orders/checkout`
- **Access:** Authenticated
- **Request Body:**
  ```json
  {
    "address_id": 5,
    "payment_method": "stripe"
  }
  ```
- **Response (201 Created):**
  ```json
  {
    "success": true,
    "data": {
      "order_number": "ORD-2026-A1B2",
      "total_amount": 184.98,
      "status": "PENDING_PAYMENT",
      "payment_client_secret": "pi_3MtwBwLkdIwHu7ix_secret_xxx"
    }
  }
  ```

### 4.2 List Customer Orders
- **Endpoint:** `GET /orders`
- **Access:** Authenticated
- **Response (200 OK):** Paginated list of customer's previous orders.

### 4.3 Get Order Details
- **Endpoint:** `GET /orders/:order_number`
- **Access:** Authenticated (Owner or Admin)

---

## 5. Webhooks & Payments

### 5.1 Payment Webhook Handler
- **Endpoint:** `POST /webhooks/payment`
- **Access:** Public (Signature Verified via `Stripe-Signature` or HMAC header)
- **Description:** Idempotently handles asynchronous events (`charge.succeeded`, `payment_intent.succeeded`) and marks order as `PAID`.

---

## 6. Standard Error Responses

All API errors return consistent JSON:

```json
{
  "success": false,
  "error": {
    "code": "INSUFFICIENT_STOCK",
    "message": "Only 2 units available for variant ID 12",
    "details": null
  }
}
```

HTTP Status Codes used:
- `200 OK`: Request succeeded
- `201 Created`: Resource successfully created
- `400 Bad Request`: Validation failure or bad input
- `401 Unauthorized`: Missing or invalid JWT
- `403 Forbidden`: Authenticated user lacks permission (non-admin)
- `404 Not Found`: Resource does not exist
- `409 Conflict`: Conflict state (e.g. duplicate email or SKU)
- `422 Unprocessable Entity`: Business logic failure (e.g., out of stock)
- `500 Internal Server Error`: Unhandled server error
