package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/klemjul/poc-aggrid-ssrm/backend/query"
)

// Handler holds the database connection for the HTTP handlers.
type Handler struct {
	DB *sql.DB
}

// SearchProducts handles POST /api/search-products.
func (h *Handler) SearchProducts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req query.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	dataSQL, countSQL, args, err := query.BuildDataQuery(req)
	if err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Count total matching rows for lastRow
	var totalCount int
	if err := h.DB.QueryRowContext(r.Context(), countSQL, args...).Scan(&totalCount); err != nil {
		log.Printf("count query error: %v\nsql: %s", err, countSQL)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Fetch data rows
	rows, err := h.DB.QueryContext(r.Context(), dataSQL, args...)
	if err != nil {
		log.Printf("data query error: %v\nsql: %s", err, dataSQL)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close() //nolint:errcheck

	cols, err := rows.Columns()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var result []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			// lib/pq returns NUMERIC columns as []byte; convert to float64
			// so encoding/json serialises them as numbers, not base64.
			if b, ok := values[i].([]byte); ok {
				if f, err := strconv.ParseFloat(string(b), 64); err == nil {
					values[i] = f
				} else {
					values[i] = string(b)
				}
			}
			row[col] = values[i]
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Return the total matching row count on every response so the frontend can
	// map it directly to AG-Grid's rowCount in SSRM.
	lastRow := totalCount

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(query.SearchResult{
		Rows:    result,
		LastRow: lastRow,
	}); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// HealthCheck handles GET /healthz.
func HealthCheck(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintln(w, "ok")
}
