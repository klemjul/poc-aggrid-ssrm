package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
	opensearchapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/query"
)

// Handler holds the OpenSearch client and index name for HTTP handlers.
type Handler struct {
	Client *opensearchgo.Client
	Index  string
}

// searchResponse mirrors the subset of the OpenSearch search response that this
// handler needs to parse.
type searchResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
	Aggregations struct {
		Groups struct {
			Buckets []struct {
				Key      any `json:"key"`
				DocCount int `json:"doc_count"`
			} `json:"buckets"`
		} `json:"groups"`
		TotalCount struct {
			Value int `json:"value"`
		} `json:"total_count"`
	} `json:"aggregations"`
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

	body, isGrouping, err := query.BuildSearchBody(req)
	if err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	res, err := opensearchapi.SearchRequest{
		Index: []string{h.Index},
		Body:  &buf,
	}.Do(r.Context(), h.Client)
	if err != nil {
		log.Printf("opensearch search error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close() //nolint:errcheck

	if res.IsError() {
		rawBody, _ := io.ReadAll(res.Body)
		log.Printf("opensearch search response error [%s]: %s", res.Status(), rawBody)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var osResp searchResponse
	if err := json.NewDecoder(res.Body).Decode(&osResp); err != nil {
		log.Printf("decode opensearch response: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	rows := make([]map[string]any, 0)
	var totalCount int

	if isGrouping {
		groupField := req.RowGroupCols[len(req.GroupKeys)].Field
		totalCount = osResp.Aggregations.TotalCount.Value
		buckets := osResp.Aggregations.Groups.Buckets

		start := req.StartRow
		end := req.EndRow
		if start > len(buckets) {
			start = len(buckets)
		}
		if end > len(buckets) {
			end = len(buckets)
		}
		for _, b := range buckets[start:end] {
			rows = append(rows, map[string]any{groupField: b.Key})
		}
	} else {
		totalCount = osResp.Hits.Total.Value
		for _, hit := range osResp.Hits.Hits {
			rows = append(rows, hit.Source)
		}
	}

	// Return the total matching row count on every response so the frontend can
	// map it directly to AG-Grid's rowCount in SSRM.
	lastRow := totalCount

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(query.SearchResult{
		Rows:    rows,
		LastRow: lastRow,
	}); err != nil {
		log.Printf("encode response: %v", err)
	}
}

// HealthCheck handles GET /healthz.
func HealthCheck(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintln(w, "ok")
}

// filterValuesOSResponse mirrors the OpenSearch aggregation response for the
// filter-values endpoint.
type filterValuesOSResponse struct {
	Aggregations struct {
		Values struct {
			Buckets []struct {
				Key string `json:"key"`
			} `json:"buckets"`
		} `json:"values"`
	} `json:"aggregations"`
}

// FilterValues handles POST /api/filter-values.
// It returns global distinct values for a whitelisted column, optionally
// narrowed by a search text prefix.
func (h *Handler) FilterValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req query.FilterValuesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	body, err := query.BuildFilterValuesBody(req)
	if err != nil {
		http.Error(w, "invalid request: "+err.Error(), http.StatusBadRequest)
		return
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	res, err := opensearchapi.SearchRequest{
		Index: []string{h.Index},
		Body:  &buf,
	}.Do(r.Context(), h.Client)
	if err != nil {
		log.Printf("opensearch filter-values error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer res.Body.Close() //nolint:errcheck

	if res.IsError() {
		rawBody, _ := io.ReadAll(res.Body)
		log.Printf("opensearch filter-values response error [%s]: %s", res.Status(), rawBody)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var osResp filterValuesOSResponse
	if err := json.NewDecoder(res.Body).Decode(&osResp); err != nil {
		log.Printf("decode opensearch filter-values response: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	values := make([]string, 0, len(osResp.Aggregations.Values.Buckets))
	for _, b := range osResp.Aggregations.Values.Buckets {
		values = append(values, b.Key)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(query.FilterValuesResponse{Values: values}); err != nil {
		log.Printf("encode filter-values response: %v", err)
	}
}
