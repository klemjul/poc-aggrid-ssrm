package query

import (
	"strings"
	"testing"
)

// helper to check that a string contains a substring (case-insensitive).
func containsCI(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(strings.ToLower(s), strings.ToLower(sub)) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}

func TestLeafQuery_NoFilters(t *testing.T) {
	req := SearchRequest{StartRow: 0, EndRow: 100}
	dataSQL, countSQL, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "SELECT")
	containsCI(t, dataSQL, "FROM products")
	containsCI(t, dataSQL, "LIMIT 100")
	containsCI(t, dataSQL, "OFFSET 0")
	containsCI(t, countSQL, "COUNT(*)")
	if len(args) != 0 {
		t.Errorf("expected no args, got %v", args)
	}
}

func TestLeafQuery_TextFilter_Contains(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   50,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "contains", Filter: "gadget"},
		},
	}
	dataSQL, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "WHERE")
	containsCI(t, dataSQL, "name ILIKE")
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
	if args[0] != "%gadget%" {
		t.Errorf("expected %%gadget%%, got %v", args[0])
	}
}

func TestLeafQuery_TextFilter_StartsWith(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "startsWith", Filter: "Pro"},
		},
	}
	_, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	if args[0] != "Pro%" {
		t.Errorf("expected 'Pro%%', got %v", args[0])
	}
}

func TestLeafQuery_TextFilter_EndsWith(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "endsWith", Filter: "Pro"},
		},
	}
	_, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	if args[0] != "%Pro" {
		t.Errorf("expected '%%Pro', got %v", args[0])
	}
}

func TestLeafQuery_NumberFilter_Equals(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"price": {FilterType: "number", Type: "equals", Filter: float64(99.99)},
		},
	}
	dataSQL, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "price = $1")
	if args[0] != 99.99 {
		t.Errorf("expected 99.99, got %v", args[0])
	}
}

func TestLeafQuery_NumberFilter_GreaterThan(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"quantity": {FilterType: "number", Type: "greaterThan", Filter: float64(10)},
		},
	}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "quantity > $1")
}

func TestLeafQuery_NumberFilter_InRange(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"price": {FilterType: "number", Type: "inRange", Filter: float64(10), FilterTo: 50},
		},
	}
	dataSQL, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "BETWEEN")
	if len(args) != 2 {
		t.Fatalf("expected 2 args for inRange, got %d", len(args))
	}
}

func TestLeafQuery_SortModel(t *testing.T) {
	req := SearchRequest{
		StartRow:  0,
		EndRow:    100,
		SortModel: []SortModel{{ColID: "price", Sort: "desc"}},
	}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "ORDER BY price DESC")
}

func TestLeafQuery_MultipleSorts(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		SortModel: []SortModel{
			{ColID: "category", Sort: "asc"},
			{ColID: "price", Sort: "desc"},
		},
	}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "ORDER BY category ASC, price DESC")
}

func TestGroupQuery_Level0(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{},
	}
	dataSQL, countSQL, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "SELECT category")
	containsCI(t, dataSQL, "GROUP BY category")
	containsCI(t, countSQL, "COUNT(DISTINCT category)")
}

func TestGroupQuery_Level1(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{"Electronics"},
	}
	dataSQL, countSQL, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "SELECT subcategory")
	containsCI(t, dataSQL, "GROUP BY subcategory")
	containsCI(t, dataSQL, "WHERE")
	containsCI(t, dataSQL, "category = $1")
	containsCI(t, countSQL, "COUNT(DISTINCT subcategory)")
	if len(args) != 1 || args[0] != "Electronics" {
		t.Errorf("expected args=[Electronics], got %v", args)
	}
}

func TestGroupQuery_LeafAfterGroups(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   50,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{"Electronics", "Phones"},
	}
	dataSQL, _, args, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	// All group keys provided → leaf query
	containsCI(t, dataSQL, "SELECT id")
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "Electronics" || args[1] != "Phones" {
		t.Errorf("unexpected args %v", args)
	}
}

func TestDisallowedColumn_Filter(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"malicious; DROP TABLE products; --": {FilterType: "text", Type: "contains", Filter: "x"},
		},
	}
	_, _, _, err := BuildDataQuery(req)
	if err == nil {
		t.Fatal("expected error for disallowed filter column")
	}
}

func TestDisallowedColumn_Sort(t *testing.T) {
	req := SearchRequest{
		StartRow:  0,
		EndRow:    10,
		SortModel: []SortModel{{ColID: "'; DROP TABLE products; --", Sort: "asc"}},
	}
	_, _, _, err := BuildDataQuery(req)
	if err == nil {
		t.Fatal("expected error for disallowed sort column")
	}
}

func TestDisallowedColumn_Group(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		RowGroupCols: []ColumnVO{
			{Field: "evil_column"},
		},
		GroupKeys: []string{},
	}
	_, _, _, err := BuildDataQuery(req)
	if err == nil {
		t.Fatal("expected error for disallowed group column")
	}
}

func TestPagination(t *testing.T) {
	req := SearchRequest{StartRow: 200, EndRow: 300}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "LIMIT 100")
	containsCI(t, dataSQL, "OFFSET 200")
}

func TestDefaultSort_Leaf(t *testing.T) {
	req := SearchRequest{StartRow: 0, EndRow: 100}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "ORDER BY created_at DESC")
}

func TestDefaultSort_Group(t *testing.T) {
	req := SearchRequest{
		StartRow:     0,
		EndRow:       100,
		RowGroupCols: []ColumnVO{{Field: "category"}},
		GroupKeys:    []string{},
	}
	dataSQL, _, _, err := BuildDataQuery(req)
	if err != nil {
		t.Fatal(err)
	}
	containsCI(t, dataSQL, "ORDER BY category ASC")
}
