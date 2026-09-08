You are working on my existing **Go e-commerce backend**.

Implement the following API endpoints directly in the existing codebase.

## Rules

1. **Inspect before modifying.**
   Check the project structure, router, handlers, services, repositories, models, database/migrations, JWT/auth middleware, role middleware, product/category implementation, tests, and `go.mod`.

2. Follow the **existing architecture and coding style**. Do not redesign the project.

3. Reuse existing code whenever possible. Do not create duplicate auth, JWT, middleware, models, repositories, or services.

4. Do not modify unrelated functionality.

5. After implementation:

   * Run `gofmt`
   * Run `go test ./...`
   * Run `go build ./...`
   * Fix errors caused by your changes.

---

# API Endpoints

## Public

```text
GET  /health
GET  /health/ready

POST /api/v1/auth/register
POST /api/v1/auth/login

GET  /api/v1/products
GET  /api/v1/products/:slug

GET  /api/v1/categories
GET  /api/v1/categories/:slug
```

`/health` = liveness check.

`/health/ready` = readiness check. Verify required dependencies such as the database using the existing architecture.

Register/login must be public.

Products and categories are publicly readable.

---

# Authenticated

All endpoints below require a valid JWT:

```text
GET    /api/v1/auth/me

GET    /api/v1/cart
POST   /api/v1/cart/items
PUT    /api/v1/cart/items/:id
DELETE /api/v1/cart/items/:id

POST   /api/v1/orders
GET    /api/v1/orders
GET    /api/v1/orders/:id
```

## `/api/v1/auth/me`

Return the current authenticated user using the user ID from JWT/context.

Do not accept a user ID from the client.

---

# Cart

### GET `/api/v1/cart`

Return the authenticated user's cart and relevant product information.

### POST `/api/v1/cart/items`

Add a product to the user's cart.

Validate:

* Product exists
* Quantity is valid
* Product is available according to the existing product/inventory model

If the product is already in the cart, update/increase its quantity instead of creating a duplicate item.

### PUT `/api/v1/cart/items/:id`

Update cart-item quantity.

Ensure:

* Item exists
* Item belongs to the authenticated user
* Quantity is valid

### DELETE `/api/v1/cart/items/:id`

Remove the cart item.

Users must not be able to modify another user's cart items by guessing an ID.

---

# Orders

### POST `/api/v1/orders`

Create an order from the authenticated user's cart.

Process:

1. Get authenticated user.
2. Load their cart.
3. Reject empty cart.
4. Validate product availability.
5. Calculate totals on the server.
6. Create order.
7. Create order items.
8. Store the product price at purchase time.
9. Associate order with the user.
10. Clear the cart after success.

Never trust client-provided prices or totals.

Use a database transaction if supported.

### GET `/api/v1/orders`

Return only the authenticated user's orders.

Use existing pagination conventions if available.

### GET `/api/v1/orders/:id`

Return the order only when it belongs to the authenticated user.

Users must not access another user's orders.

---

# Admin

Admin endpoints require:

```text
Valid JWT + Admin role
```

Reuse the existing authorization middleware.

```text
POST   /api/v1/products
PUT    /api/v1/products/:id
DELETE /api/v1/products/:id

POST   /api/v1/categories
PUT    /api/v1/categories/:id
DELETE /api/v1/categories/:id

GET    /api/v1/admin/orders
PUT    /api/v1/admin/orders/:id/status
```

## Products

Implement create, update, and delete using the existing product architecture.

## Categories

Implement create, update, and delete using the existing category architecture.

## Admin Orders

`GET /api/v1/admin/orders`

Return all orders. Follow existing pagination/filtering conventions.

`PUT /api/v1/admin/orders/:id/status`

Allow admins to update order status.

Reuse the existing status definition if available.

If none exists, use controlled values:

```text
pending
confirmed
processing
shipped
delivered
cancelled
```

Reject invalid statuses.

---

# Security

Verify:

* Public routes work without JWT.
* Authenticated routes reject missing/invalid JWT.
* Normal users cannot access admin routes.
* Users cannot access another user's cart.
* Users cannot modify another user's cart items.
* Users cannot access another user's orders.
* Admin routes require JWT + admin role.
* JWT secret/config comes from existing environment/configuration.
* No secrets are hardcoded.
* Passwords/sensitive auth data are never returned.

Use the project's existing:

* HTTP status codes
* JSON response format
* Error format
* Validation
* Logging
* Service/repository pattern
* Database transaction pattern
* Naming conventions

---

# Database

Inspect existing models/migrations before creating anything.

If cart/order functionality does not exist, add the required models using the current architecture.

Expected relationship:

```text
User
 ├── Cart
 │    └── CartItem → Product
 │
 └── Order
      └── OrderItem → Product
```

Order items must store the product price at the time of purchase.

---

# Tests

Add/update tests for the important cases.

### Auth

* Register success
* Duplicate registration
* Login success
* Invalid credentials
* `/auth/me` with valid JWT
* `/auth/me` without JWT

### Products/Categories

* Public list/detail
* Admin create/update/delete
* Normal user cannot modify

### Cart

* View cart
* Add item
* Update quantity
* Remove item
* Invalid quantity
* Product not found
* Cannot modify another user's item

### Orders

* Empty cart rejected
* Successful order
* Correct server-side total
* Cart cleared after order
* User only sees own orders
* Cannot access another user's order

### Admin

* Admin can list orders
* Admin can update status
* Normal user cannot access admin routes
* Invalid status rejected

---

# Implementation Workflow

## 1. Inspect

First inspect the repository and briefly identify:

* Architecture
* Router
* Auth/JWT
* Product/category implementation
* Models
* Middleware
* Database
* Tests

## 2. Plan

Create a short implementation plan based on the actual project.

Do not ask for approval unless something is genuinely ambiguous.

## 3. Implement

Implement using the existing architecture in this order:

```text
Auth
 ↓
Products/Categories
 ↓
Cart
 ↓
Orders
 ↓
Admin
```

Pay attention to existing commented-out product/category routes.

## 4. Verify

Run:

```bash
gofmt
go test ./...
go build ./...
```

Fix failures caused by your changes.

## 5. Review

Check for authorization bugs, ownership issues, validation problems, transaction problems, incorrect relationships, duplicate code, incorrect status codes, and sensitive-data exposure.

Fix issues you find.

## 6. Final Report

Return:

```text
Implemented:
- ...

Files changed:
- ...

Database changes:
- ...

Routes added:
- ...

Tests:
- ...

Build status:
- ...

Follow-up:
- ...
```

**Important: Actually modify the project files. Do not only provide example code or instructions.**
