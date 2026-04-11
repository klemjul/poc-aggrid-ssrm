# AG-Grid Server Side Row Model POC

This proof of concept explores how [AG-Grid's Server Side Row Model](https://www.ag-grid.com/react-data-grid/server-side-model/) (SSRM) works end-to-end with a Go backend. The goal is to validate that AG-Grid can efficiently handle **100 000+ rows** with server-side pagination, sorting, filtering, and row grouping — without loading all data into the browser.

Two backend variants are provided:

| Variant       | Backend directory      | Database           | Compose file                    |
| ------------- | ---------------------- | ------------------ | ------------------------------- |
| **PostgreSQL** | `backend/`            | PostgreSQL 17      | `docker-compose.yml`            |
| **OpenSearch** | `backend-opensearch/` | OpenSearch 2       | `docker-compose.opensearch.yml` |

Both variants expose the same HTTP API (`POST /api/search-products`) and work with the same React/AG-Grid frontend.

### Tech Stack

| Layer    | Technology                                     |
| -------- | ---------------------------------------------- |
| Frontend | React 19, TypeScript, Vite, AG-Grid Enterprise |
| Backend  | Go 1.24                                        |
| Database | PostgreSQL 17 **or** OpenSearch 2              |

### What it demonstrates

- **Server-side pagination** — the grid requests rows in pages, the backend returns only the requested slice
- **Sorting & filtering** — sort/filter parameters are sent to the API and translated to parameterised SQL (PostgreSQL) or OpenSearch query DSL (OpenSearch)
- **Row grouping** — group by `category` and `subcategory` with drill-down, computed entirely server-side
- **Dev Container** — one-click setup with pre-configured PostgreSQL, Go, and Node

---

## Getting Started

### Option A — Dev Container (recommended, PostgreSQL)

> Requires [VS Code](https://code.visualstudio.com/) + [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) + [Docker](https://docs.docker.com/get-docker/)

1. **Open in Dev Container** — open the repo in VS Code, then `Ctrl+Shift+P` → **Dev Containers: Reopen in Container**. This starts PostgreSQL, installs dependencies, and configures environment variables automatically.

2. **Seed the database**

   ```bash
   cd backend && go run ./cmd/seed/
   ```

3. **Start the backend**

   ```bash
   cd backend && go run .
   ```

4. **Start the frontend** (second terminal)

   ```bash
   cd frontend && npm run dev
   ```

5. Open <http://localhost:5173>

### Option B — Docker Compose with PostgreSQL

> Requires [Docker](https://docs.docker.com/get-docker/) & Docker Compose

```bash
cp .env.example .env
docker compose up --build -d
docker compose run --rm seed
```

Open <http://localhost:5173>

| Service  | Port | Description                 |
| -------- | ---- | --------------------------- |
| db       | 5432 | PostgreSQL 17               |
| backend  | 8080 | Go REST API                 |
| frontend | 5173 | React app (served by nginx) |

### Option C — Docker Compose with OpenSearch

```bash
docker compose -f docker-compose.opensearch.yml up --build -d
docker compose -f docker-compose.opensearch.yml run --rm seed
```

Open <http://localhost:5173>

| Service    | Port | Description                 |
| ---------- | ---- | --------------------------- |
| opensearch | 9200 | OpenSearch 2 (single-node)  |
| backend    | 8080 | Go REST API                 |
| frontend   | 5173 | React app (served by nginx) |

> **Note:** OpenSearch requires more memory than PostgreSQL. Ensure Docker has at least 2 GB of RAM available. The compose file caps the JVM heap at 512 MB (`OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m`).

---

## Development

### Backend (PostgreSQL)

```bash
cd backend
go run .                # start the API (needs PostgreSQL)
go test ./... -v        # all tests (unit + integration)
go test ./query/... -v  # unit tests only (no DB required)
```

### Backend (OpenSearch)

```bash
cd backend-opensearch
go run .                # start the API (needs OpenSearch on localhost:9200)
go test ./... -v        # all tests (unit only; no live OpenSearch needed)
go test ./query/... -v  # query-builder unit tests
```

### Frontend

```bash
cd frontend
npm run dev             # start dev server
npm run lint            # ESLint
npm run format:check    # Prettier check
npm run format          # Prettier fix
npm run build           # production build
```

Environment variables are documented in `.env.example`. In the Dev Container they are pre-configured via `remoteEnv`.

### API

**`POST /api/search-products`** — returns a page of rows matching the AG-Grid SSRM request format.

```json
// Request
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

// Response
{
  "rows": [{ "name": "...", "category": "...", "price": 29.99, ... }],
  "lastRow": 42
}
```

### Database schema

| Column      | Type          | Notes         |
| ----------- | ------------- | ------------- |
| id          | UUID          | PK, auto-gen  |
| name        | TEXT          |               |
| category    | TEXT          | groupable     |
| subcategory | TEXT          | groupable     |
| price       | NUMERIC(10,2) |               |
| quantity    | INT           |               |
| rating      | NUMERIC(3,2)  |               |
| created_at  | TIMESTAMPTZ   | default NOW() |

The OpenSearch index uses equivalent types: `keyword` for categorical fields, `float`/`integer` for numbers, `date` for timestamps, and a `text` + `.keyword` sub-field for `name` (to support both full-text and exact/wildcard filtering).

## Useful Links

- [AG-Grid Server Side Row Model](https://www.ag-grid.com/react-data-grid/server-side-model/)
- [Implementing the server side datasource](https://www.ag-grid.com/react-data-grid/server-side-model-datasource/#implementing-the-server-side-datasource)
- [AG-Grid Row Grouping (SSRM)](https://www.ag-grid.com/react-data-grid/server-side-model-grouping/)
- [AG-Grid Filtering (SSRM)](https://www.ag-grid.com/react-data-grid/server-side-model-filtering/)
- [OpenSearch Go client](https://github.com/opensearch-project/opensearch-go)
