package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"

	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/api"
	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/query"
)

// mockTransport is an http.RoundTripper that returns a pre-set response for every
// request, allowing the handler to be tested without a real OpenSearch cluster.
type mockTransport struct {
	body       string
	statusCode int
}

func (m *mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

// newMockHandler creates a Handler backed by a mock OpenSearch transport.
func newMockHandler(t *testing.T, osBody string, osStatus int) *api.Handler {
	t.Helper()
	client, err := opensearchgo.NewClient(opensearchgo.Config{
		Transport: &mockTransport{body: osBody, statusCode: osStatus},
	})
	if err != nil {
		t.Fatalf("new opensearch client: %v", err)
	}
	return &api.Handler{Client: client, Index: "products"}
}

// postSearch sends a POST /api/search-products request to the handler and
// returns the recorded response.
func postSearch(t *testing.T, h *api.Handler, req query.SearchRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/search-products", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	return w
}

// --- HealthCheck ---

func TestHealthCheck(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	api.HealthCheck(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("expected 'ok' in body, got: %s", w.Body.String())
	}
}

// --- Method / JSON validation ---

func TestSearchProducts_MethodNotAllowed(t *testing.T) {
	h := newMockHandler(t, "", 200)
	r := httptest.NewRequest(http.MethodGet, "/api/search-products", nil)
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSearchProducts_InvalidJSON(t *testing.T) {
	h := newMockHandler(t, "", 200)
	r := httptest.NewRequest(http.MethodPost, "/api/search-products", bytes.NewBufferString("{bad json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSearchProducts_DisallowedField(t *testing.T) {
	h := newMockHandler(t, "", 200)
	w := postSearch(t, h, query.SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]query.FilterModel{
			"'; DROP TABLE products; --": {FilterType: "text", Type: "contains", Filter: "x"},
		},
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for disallowed field, got %d", w.Code)
	}
}

// --- Leaf query ---

func TestSearchProducts_LeafQuery_AllRows(t *testing.T) {
	osResp := `{
		"hits": {
			"total": {"value": 2},
			"hits": [
				{"_source": {"id":"1","name":"Widget A","category":"Electronics","subcategory":"Phones","price":199.99,"quantity":10,"rating":4.5,"created_at":"2024-01-01T00:00:00Z"}},
				{"_source": {"id":"2","name":"Widget B","category":"Clothing","subcategory":"Shirts","price":29.99,"quantity":50,"rating":3.9,"created_at":"2024-01-02T00:00:00Z"}}
			]
		}
	}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 100})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp.Rows))
	}
	// All rows fit on one page → lastRow should equal totalCount
	if resp.LastRow != 2 {
		t.Errorf("expected lastRow=2, got %d", resp.LastRow)
	}
}

func TestSearchProducts_LeafQuery_Pagination_MoreRows(t *testing.T) {
	// OS reports 100 total hits but only returns 2 (page is not the last one)
	osResp := `{
		"hits": {
			"total": {"value": 100},
			"hits": [
				{"_source": {"id":"1","name":"Widget A","category":"Electronics","subcategory":"Phones","price":199.99,"quantity":10,"rating":4.5,"created_at":"2024-01-01T00:00:00Z"}},
				{"_source": {"id":"2","name":"Widget B","category":"Clothing","subcategory":"Shirts","price":29.99,"quantity":50,"rating":3.9,"created_at":"2024-01-02T00:00:00Z"}}
			]
		}
	}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 2})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// 0 + 2 < 100 → more rows exist
	if resp.LastRow != -1 {
		t.Errorf("expected lastRow=-1 (more rows), got %d", resp.LastRow)
	}
}

func TestSearchProducts_LeafQuery_EmptyResult(t *testing.T) {
	osResp := `{"hits": {"total": {"value": 0}, "hits": []}}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 100})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(resp.Rows))
	}
	// 0 + 0 >= 0 → last page
	if resp.LastRow != 0 {
		t.Errorf("expected lastRow=0, got %d", resp.LastRow)
	}
}

func TestSearchProducts_LeafQuery_ContentType(t *testing.T) {
	osResp := `{"hits": {"total": {"value": 0}, "hits": []}}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 10})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

// --- Group query ---

func TestSearchProducts_GroupQuery_Level0(t *testing.T) {
	osResp := `{
		"hits": {"total": {"value": 0}, "hits": []},
		"aggregations": {
			"groups": {
				"buckets": [
					{"key": "Electronics", "doc_count": 3},
					{"key": "Clothing",    "doc_count": 2}
				]
			},
			"total_count": {"value": 2}
		}
	}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []query.ColumnVO{
			{Field: "category"},
		},
		GroupKeys: []string{},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 group rows, got %d", len(resp.Rows))
	}
	// Verify the group field is present
	if resp.Rows[0]["category"] != "Electronics" {
		t.Errorf("expected first group=Electronics, got %v", resp.Rows[0]["category"])
	}
	// All groups fit on one page → lastRow = totalCount
	if resp.LastRow != 2 {
		t.Errorf("expected lastRow=2, got %d", resp.LastRow)
	}
}

func TestSearchProducts_GroupQuery_Pagination(t *testing.T) {
	// 10 buckets but we only request the first 3 (StartRow=0, EndRow=3)
	buckets := make([]map[string]any, 10)
	for i := range buckets {
		buckets[i] = map[string]any{"key": "Cat" + string(rune('A'+i)), "doc_count": 5}
	}
	bs, _ := json.Marshal(buckets)
	osResp := `{
		"hits": {"total": {"value": 0}, "hits": []},
		"aggregations": {
			"groups": {"buckets": ` + string(bs) + `},
			"total_count": {"value": 10}
		}
	}`
	h := newMockHandler(t, osResp, 200)
	w := postSearch(t, h, query.SearchRequest{
		StartRow:     0,
		EndRow:       3,
		RowGroupCols: []query.ColumnVO{{Field: "category"}},
		GroupKeys:    []string{},
	})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Rows) != 3 {
		t.Errorf("expected 3 rows (paginated groups), got %d", len(resp.Rows))
	}
	// 0 + 3 < 10 → more rows exist
	if resp.LastRow != -1 {
		t.Errorf("expected lastRow=-1, got %d", resp.LastRow)
	}
}

// --- OpenSearch error handling ---

func TestSearchProducts_OpenSearchError(t *testing.T) {
	osErrResp := `{"error":{"type":"search_phase_execution_exception","reason":"all shards failed"}}`
	h := newMockHandler(t, osErrResp, 500)
	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 10})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when OpenSearch returns error, got %d", w.Code)
	}
}
