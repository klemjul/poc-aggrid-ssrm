# AG-Grid Server Side Row Model POC

A proof-of-concept full-stack application demonstrating [AG-Grid](https://www.ag-grid.com/) in **Server Side Row Model** (SSRM) mode with:

- **Frontend** — React 19 + Vite + TypeScript, AG-Grid Enterprise (SSRM, row grouping, sorting, filtering)
- **Backend** — Go REST API that translates AG-Grid requests into PostgreSQL queries
- **Database** — PostgreSQL 17 with a `products` table (100 000 rows)

---

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose

### 1. Start the stack

```bash
cp .env.example .env
docker compose up --build -d
```

This starts:

| Service  | Port  | Description                    |
|----------|-------|--------------------------------|
| db       | 5432  | PostgreSQL 17                  |
| backend  | 8080  | Go REST API                    |
| frontend | 5173  | React app (served by nginx)    |

### 2. Seed the database

```bash
docker compose run --rm seed
```

Inserts 100 000 sample product rows (skips if already seeded).

### 3. Open the app

Navigate to <http://localhost:5173>

---

## Development

### Backend (Go)

```bash
cd backend
go run . # requires a running PostgreSQL instance
```

Set environment variables (see `.env.example`) or export them manually.

```bash
# Run all tests (unit + integration)
go test ./... -v

# Unit tests only (no DB required)
go test ./query/... -v
```

### Frontend (React)

```bash
cd frontend
npm install
VITE_API_URL=http://localhost:8080 npm run dev
```

```bash
npm run lint          # ESLint
npm run format:check  # Prettier check
npm run format        # Prettier fix
npm run build         # Production build
```

---

## Architecture

```
frontend (React + AG-Grid SSRM)
    │  POST /api/search-products  { startRow, endRow, sortModel, filterModel, rowGroupCols, groupKeys }
    ▼
backend (Go HTTP API)
    │  BuildDataQuery → parameterised SQL
    ▼
PostgreSQL 17 — products table
```

### Products table schema

| Column      | Type           | Notes          |
|-------------|----------------|----------------|
| id          | UUID           | PK, auto-gen   |
| name        | TEXT           |                |
| category    | TEXT           | groupable      |
| subcategory | TEXT           | groupable      |
| price       | NUMERIC(10,2)  |                |
| quantity    | INT            |                |
| rating      | NUMERIC(3,2)   |                |
| created_at  | TIMESTAMPTZ    | default NOW()  |

### API

`POST /api/search-products`

Request body (AG-Grid SSRM format):

```json
{
  "startRow": 0,
  "endRow": 100,
  "sortModel": [{ "colId": "price", "sort": "desc" }],
  "filterModel": {
    "category": { "filterType": "text", "type": "contains", "filter": "Electronics" }
  },
  "rowGroupCols": [{ "field": "category" }],
  "groupKeys": []
}
```

Response:

```json
{
  "rows": [...],
  "lastRow": 42
}
```

`GET /healthz` — liveness probe

---

## CI

GitHub Actions (`.github/workflows/ci.yml`) runs on every push and pull request:

| Job              | Description                                              |
|------------------|----------------------------------------------------------|
| `lint-go`        | golangci-lint                                            |
| `lint-frontend`  | ESLint + Prettier check                                  |
| `test-go`        | `go test ./...` with a PostgreSQL service container      |
| `build-frontend` | `npm run build`                                          |
| `build-docker`   | Docker image builds for backend, seed, and frontend      |
