package api_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/klemjul/poc-aggrid-ssrm/backend/api"
	"github.com/klemjul/poc-aggrid-ssrm/backend/migration"
	"github.com/klemjul/poc-aggrid-ssrm/backend/query"
)

// testDB opens a connection to the test PostgreSQL database.
// It skips the test if no TEST_DB_DSN environment variable is set.
func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "5432"),
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", "postgres"),
			getEnv("DB_NAME", "products_db"),
		)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("skip integration test — cannot open DB: %v", err)
	}
	// Retry ping to handle slow DB startup
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("waiting for DB (%d/10)…", i+1)
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Skipf("skip integration test — DB not reachable: %v", err)
	}
	return db
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// setupTestDB applies migrations and seeds a small fixture dataset.
func setupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := migration.Apply(db); err != nil {
		t.Fatalf("migration: %v", err)
	}
	// Truncate and insert known rows for deterministic assertions.
	_, err := db.Exec(`TRUNCATE products`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	rows := []struct {
		name, cat, sub string
		price          float64
		qty            int
		rating         float64
	}{
		{"Widget A", "Electronics", "Phones", 199.99, 10, 4.5},
		{"Widget B", "Electronics", "Laptops", 999.99, 5, 4.8},
		{"Gadget C", "Clothing", "Shirts", 29.99, 50, 3.9},
		{"Gadget D", "Clothing", "Pants", 49.99, 30, 4.1},
		{"Tool E", "Electronics", "Phones", 59.99, 20, 4.0},
	}
	for _, r := range rows {
		_, err := db.Exec(
			`INSERT INTO products (name, category, subcategory, price, quantity, rating)
			 VALUES ($1,$2,$3,$4,$5,$6)`,
			r.name, r.cat, r.sub, r.price, r.qty, r.rating,
		)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func newHandler(db *sql.DB) *api.Handler {
	return &api.Handler{DB: db}
}

func postSearch(t *testing.T, h *api.Handler, req query.SearchRequest) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(req)
	r := httptest.NewRequest(http.MethodPost, "/api/search-products", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	return w
}

// --- Tests ---

func TestSearchProducts_AllRows(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	setupTestDB(t, db)
	h := newHandler(db)

	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 100})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Rows) != 5 {
		t.Errorf("expected 5 rows, got %d", len(resp.Rows))
	}
	if resp.LastRow != 5 {
		t.Errorf("expected lastRow=5, got %d", resp.LastRow)
	}
}

func TestSearchProducts_TextFilter(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	setupTestDB(t, db)
	h := newHandler(db)

	w := postSearch(t, h, query.SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]query.FilterModel{
			"name": {FilterType: "text", Type: "contains", Filter: "Gadget"},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp.Rows))
	}
}

func TestSearchProducts_GroupByCategory(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	setupTestDB(t, db)
	h := newHandler(db)

	w := postSearch(t, h, query.SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []query.ColumnVO{
			{Field: "category"},
		},
		GroupKeys: []string{},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// Should return 2 distinct categories: Electronics, Clothing
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 category groups, got %d", len(resp.Rows))
	}
}

func TestSearchProducts_GroupDrillDown(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	setupTestDB(t, db)
	h := newHandler(db)

	w := postSearch(t, h, query.SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []query.ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{"Electronics"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	// Electronics has Phones and Laptops
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 subcategory groups for Electronics, got %d", len(resp.Rows))
	}
}

func TestSearchProducts_Pagination(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	setupTestDB(t, db)
	h := newHandler(db)

	w := postSearch(t, h, query.SearchRequest{StartRow: 0, EndRow: 2})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var resp query.SearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp.Rows))
	}
	// More rows exist so lastRow should be -1
	if resp.LastRow != -1 {
		t.Errorf("expected lastRow=-1, got %d", resp.LastRow)
	}
}

func TestSearchProducts_MethodNotAllowed(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	h := newHandler(db)
	r := httptest.NewRequest(http.MethodGet, "/api/search-products", nil)
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSearchProducts_InvalidJSON(t *testing.T) {
	db := testDB(t)
	defer db.Close() //nolint:errcheck
	h := newHandler(db)
	r := httptest.NewRequest(http.MethodPost, "/api/search-products", bytes.NewBufferString("{bad json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SearchProducts(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
