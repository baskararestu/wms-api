# WMS API Integration Guidelines

This document outlines the strict flow and constraints required for this backend WMS project.

## 1. Internal WMS APIs

APIs owned exclusively by the WMS. All endpoints must be authenticated.

### 1.1 `GET /orders`

**Goal:** See all orders and their state.

- **Rules:** Return all orders, allow filtering by `wms_status`, sort by `updated_at` (desc).

### 1.2 `GET /orders/:order_sn`

**Goal:** View full order details.

- **Rules:** Return full detail or 404.

### 1.3 `POST /orders/:order_sn/pick`

**Goal:** Mark order as picking.

- **Rules:** Allowed ONLY when `wms_status` = `READY_TO_PICK`. Transitions to `PICKING`.

### 1.4 `POST /orders/:order_sn/pack`

**Goal:** Mark order as packed.

- **Rules:** Allowed ONLY when `wms_status` = `PICKING`. Transitions to `PACKED`.

### 1.5 `POST /orders/:order_sn/ship` (CRITICAL)

**Goal:** Ship an order and synchronize it with the marketplace.

- **Rules:**
  1. Validate `wms_status` = `PACKED`.
  2. Call marketplace API: `POST /logistic/ship`.
  3. Receive response (which contains the real `tracking_no`).
  4. Persist `tracking_number` and `shipping_status`.
  5. Update `wms_status` = `SHIPPED`.
- **Constraint:** NEVER generate your own tracking number. It must come from the Marketplace.

## 2. Marketplace Integration

The codebase must integrate with the Mock API handling:

- OAuth authorization & Signed requests
- Token refresh flows
- Handing errors gracefully (401, 429, random 500s)

## 3. Webhook Handling

Endpoints that the Marketplace calls to update the WMS:

- `POST /webhook/order-status`
- `POST /webhook/shipping-status`
- **Rules:** Update local order records safely, handle duplicate events (idempotence). These are public endpoints, do NOT require internal Auth.

## 4. Security

- Store marketplace tokens securely in DB ONLY.
- Authenticate Internal APIs using standard methods (e.g. JWT over users table).
- Validate all state transitions locally before moving forward.
