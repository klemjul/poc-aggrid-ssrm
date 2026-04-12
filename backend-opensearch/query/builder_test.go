package query

import (
	"encoding/json"
	"strings"
	"testing"
)

// bodyJSON marshals the search body to a JSON string for assertions.
func bodyJSON(t *testing.T, req SearchRequest) string {
	t.Helper()
	body, _, err := BuildSearchBody(req)
	if err != nil {
		t.Fatalf("BuildSearchBody: %v", err)
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}

func TestLeafBody_NoFilters(t *testing.T) {
	req := SearchRequest{StartRow: 0, EndRow: 100}
	body, isGroup, err := BuildSearchBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if isGroup {
		t.Error("expected isGroup=false")
	}
	if body["from"] != 0 {
		t.Errorf("expected from=0, got %v", body["from"])
	}
	if body["size"] != 100 {
		t.Errorf("expected size=100, got %v", body["size"])
	}
}

func TestLeafBody_Pagination(t *testing.T) {
	req := SearchRequest{StartRow: 200, EndRow: 300}
	body, _, err := BuildSearchBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if body["from"] != 200 {
		t.Errorf("expected from=200, got %v", body["from"])
	}
	if body["size"] != 100 {
		t.Errorf("expected size=100, got %v", body["size"])
	}
}

func TestLeafBody_DefaultSort(t *testing.T) {
	s := bodyJSON(t, SearchRequest{StartRow: 0, EndRow: 10})
	want := `"created_at"`
	if !containsStr(s, want) {
		t.Errorf("expected default sort on created_at, got: %s", s)
	}
}

func TestLeafBody_SortModel(t *testing.T) {
	req := SearchRequest{
		StartRow:  0,
		EndRow:    100,
		SortModel: []SortModel{{ColID: "price", Sort: "desc"}},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"price"`) || !containsStr(s, `"desc"`) {
		t.Errorf("expected price desc sort, got: %s", s)
	}
}

func TestLeafBody_SortModel_TextField(t *testing.T) {
	req := SearchRequest{
		StartRow:  0,
		EndRow:    100,
		SortModel: []SortModel{{ColID: "name", Sort: "asc"}},
	}
	s := bodyJSON(t, req)
	// name must be sorted on .keyword sub-field
	if !containsStr(s, `"name.keyword"`) {
		t.Errorf("expected name.keyword for sort, got: %s", s)
	}
}

func TestLeafBody_TextFilter_Contains(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   50,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "contains", Filter: "gadget"},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"name.keyword"`) {
		t.Errorf("expected name.keyword for text filter, got: %s", s)
	}
	if !containsStr(s, `"*gadget*"`) {
		t.Errorf("expected wildcard *gadget*, got: %s", s)
	}
	if !containsStr(s, `"case_insensitive":true`) {
		t.Errorf("expected case_insensitive:true, got: %s", s)
	}
}

func TestLeafBody_TextFilter_StartsWith(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "startsWith", Filter: "Pro"},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"Pro*"`) {
		t.Errorf("expected Pro* wildcard, got: %s", s)
	}
}

func TestLeafBody_TextFilter_EndsWith(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"name": {FilterType: "text", Type: "endsWith", Filter: "Kit"},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"*Kit"`) {
		t.Errorf("expected *Kit wildcard, got: %s", s)
	}
}

func TestLeafBody_TextFilter_Equals(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"category": {FilterType: "text", Type: "equals", Filter: "Electronics"},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"term"`) {
		t.Errorf("expected term query for equals, got: %s", s)
	}
	if !containsStr(s, `"Electronics"`) {
		t.Errorf("expected Electronics in term, got: %s", s)
	}
}

func TestLeafBody_NumberFilter_Equals(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"price": {FilterType: "number", Type: "equals", Filter: float64(99.99)},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"term"`) {
		t.Errorf("expected term query for number equals, got: %s", s)
	}
	if !containsStr(s, `"price"`) {
		t.Errorf("expected price field, got: %s", s)
	}
}

func TestLeafBody_NumberFilter_GreaterThan(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"quantity": {FilterType: "number", Type: "greaterThan", Filter: float64(10)},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"range"`) || !containsStr(s, `"gt"`) {
		t.Errorf("expected range gt query, got: %s", s)
	}
}

func TestLeafBody_NumberFilter_InRange(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"price": {FilterType: "number", Type: "inRange", Filter: float64(10), FilterTo: 50},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"gte"`) || !containsStr(s, `"lte"`) {
		t.Errorf("expected range gte/lte query, got: %s", s)
	}
}

func TestGroupBody_Level0(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{},
	}
	body, isGroup, err := BuildSearchBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !isGroup {
		t.Error("expected isGroup=true")
	}
	if body["size"] != 0 {
		t.Errorf("expected size=0 for group query, got %v", body["size"])
	}
	aggs, ok := body["aggs"].(map[string]any)
	if !ok {
		t.Fatal("expected aggs in body")
	}
	if _, ok := aggs["groups"]; !ok {
		t.Error("expected groups aggregation")
	}
	if _, ok := aggs["total_count"]; !ok {
		t.Error("expected total_count aggregation")
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"category"`) {
		t.Errorf("expected category field in group body, got: %s", s)
	}
}

func TestGroupBody_Level1_WithGroupKey(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{"Electronics"},
	}
	body, isGroup, err := BuildSearchBody(req)
	if err != nil {
		t.Fatal(err)
	}
	if !isGroup {
		t.Error("expected isGroup=true")
	}
	// The body should contain a term filter for category=Electronics
	s, _ := json.Marshal(body)
	if !containsStr(string(s), `"Electronics"`) {
		t.Errorf("expected Electronics term filter, got: %s", s)
	}
	if !containsStr(string(s), `"subcategory"`) {
		t.Errorf("expected subcategory aggregation field, got: %s", s)
	}
}

func TestGroupBody_LeafAfterAllGroupKeys(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   50,
		RowGroupCols: []ColumnVO{
			{Field: "category"},
			{Field: "subcategory"},
		},
		GroupKeys: []string{"Electronics", "Phones"},
	}
	body, isGroup, err := BuildSearchBody(req)
	if err != nil {
		t.Fatal(err)
	}
	// All group keys provided → leaf query
	if isGroup {
		t.Error("expected isGroup=false for fully-drilled-down request")
	}
	if body["size"] != 50 {
		t.Errorf("expected size=50, got %v", body["size"])
	}
}

func TestGroupBody_DefaultSort(t *testing.T) {
	req := SearchRequest{
		StartRow:     0,
		EndRow:       100,
		RowGroupCols: []ColumnVO{{Field: "category"}},
		GroupKeys:    []string{},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"_key"`) || !containsStr(s, `"asc"`) {
		t.Errorf("expected _key:asc default group sort, got: %s", s)
	}
}

func TestDisallowedField_Filter(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		FilterModel: map[string]FilterModel{
			"malicious; DROP TABLE products; --": {FilterType: "text", Type: "contains", Filter: "x"},
		},
	}
	_, _, err := BuildSearchBody(req)
	if err == nil {
		t.Fatal("expected error for disallowed filter field")
	}
}

func TestDisallowedField_Sort(t *testing.T) {
	req := SearchRequest{
		StartRow:  0,
		EndRow:    10,
		SortModel: []SortModel{{ColID: "'; DROP TABLE products; --", Sort: "asc"}},
	}
	_, _, err := BuildSearchBody(req)
	if err == nil {
		t.Fatal("expected error for disallowed sort field")
	}
}

func TestDisallowedField_Group(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   10,
		RowGroupCols: []ColumnVO{
			{Field: "evil_column"},
		},
		GroupKeys: []string{},
	}
	_, _, err := BuildSearchBody(req)
	if err == nil {
		t.Fatal("expected error for disallowed group field")
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}

// --- Set filter ---

func TestLeafBody_SetFilter_Category(t *testing.T) {
	vals := []string{"Electronics", "Furniture"}
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {FilterType: "set", Values: vals},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"terms"`) {
		t.Errorf("expected terms query for set filter, got: %s", s)
	}
	if !containsStr(s, `"Electronics"`) || !containsStr(s, `"Furniture"`) {
		t.Errorf("expected Electronics and Furniture in terms, got: %s", s)
	}
}

func TestLeafBody_SetFilter_Empty(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {FilterType: "set", Values: []string{}},
		},
	}
	s := bodyJSON(t, req)
	// Empty set filter should produce match_all (no filter clause added)
	if containsStr(s, `"terms"`) {
		t.Errorf("expected no terms query for empty set filter, got: %s", s)
	}
}

// --- Multi filter ---

func TestLeafBody_MultiFilter_SetOnly(t *testing.T) {
	setFM := &FilterModel{FilterType: "set", Values: []string{"Electronics"}}
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {
				FilterType:   "multi",
				FilterModels: []*FilterModel{nil, setFM},
			},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"terms"`) {
		t.Errorf("expected terms query from set child, got: %s", s)
	}
	if !containsStr(s, `"Electronics"`) {
		t.Errorf("expected Electronics in terms, got: %s", s)
	}
}

func TestLeafBody_MultiFilter_TextOnly(t *testing.T) {
	textFM := &FilterModel{FilterType: "text", Type: "contains", Filter: "elec"}
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {
				FilterType:   "multi",
				FilterModels: []*FilterModel{textFM, nil},
			},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"wildcard"`) {
		t.Errorf("expected wildcard from text child, got: %s", s)
	}
	if !containsStr(s, `"*elec*"`) {
		t.Errorf("expected *elec* wildcard, got: %s", s)
	}
}

func TestLeafBody_MultiFilter_TextAndSet(t *testing.T) {
	textFM := &FilterModel{FilterType: "text", Type: "contains", Filter: "elec"}
	setFM := &FilterModel{FilterType: "set", Values: []string{"Electronics", "Gadgets"}}
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {
				FilterType:   "multi",
				FilterModels: []*FilterModel{textFM, setFM},
			},
		},
	}
	s := bodyJSON(t, req)
	if !containsStr(s, `"wildcard"`) {
		t.Errorf("expected wildcard from text child, got: %s", s)
	}
	if !containsStr(s, `"terms"`) {
		t.Errorf("expected terms from set child, got: %s", s)
	}
	// Two active filters → combined with bool.filter
	if !containsStr(s, `"bool"`) {
		t.Errorf("expected bool wrapper for two active filters, got: %s", s)
	}
}

func TestLeafBody_MultiFilter_AllNil(t *testing.T) {
	req := SearchRequest{
		StartRow: 0,
		EndRow:   100,
		FilterModel: map[string]FilterModel{
			"category": {
				FilterType:   "multi",
				FilterModels: []*FilterModel{nil, nil},
			},
		},
	}
	s := bodyJSON(t, req)
	// No active child filters → should produce match_all
	if containsStr(s, `"terms"`) || containsStr(s, `"wildcard"`) {
		t.Errorf("expected no filter clause for all-nil multi filter, got: %s", s)
	}
}

// --- Filter values body ---

func TestBuildFilterValuesBody_Category(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `"terms"`) {
		t.Errorf("expected terms aggregation, got: %s", s)
	}
	if !containsStr(s, `"category"`) {
		t.Errorf("expected category field in aggregation, got: %s", s)
	}
}

func TestBuildFilterValuesBody_Subcategory(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "subcategory"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `"subcategory"`) {
		t.Errorf("expected subcategory field, got: %s", s)
	}
}

func TestBuildFilterValuesBody_WithSearchText(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category", SearchText: "elec"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `"prefix"`) {
		t.Errorf("expected prefix query when searchText provided, got: %s", s)
	}
	if !containsStr(s, `"elec"`) {
		t.Errorf("expected searchText value in prefix query, got: %s", s)
	}
}

func TestBuildFilterValuesBody_DefaultLimit(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `200`) {
		t.Errorf("expected default limit of 200, got: %s", s)
	}
}

func TestBuildFilterValuesBody_CustomLimit(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `50`) {
		t.Errorf("expected custom limit 50, got: %s", s)
	}
}

func TestBuildFilterValuesBody_DisallowedCol(t *testing.T) {
	_, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "price"})
	if err == nil {
		t.Fatal("expected error for disallowed colId")
	}
}

func TestBuildFilterValuesBody_InjectionAttempt(t *testing.T) {
	_, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "'; DROP TABLE products; --"})
	if err == nil {
		t.Fatal("expected error for injection attempt colId")
	}
}

func TestBuildFilterValuesBody_LimitClamped(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category", Limit: 99999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	// The limit should be clamped to maxFilterValuesLimit (1000), not 99999
	if containsStr(s, `99999`) {
		t.Errorf("expected limit to be clamped, but 99999 found in body: %s", s)
	}
	if !containsStr(s, `1000`) {
		t.Errorf("expected clamped limit of 1000 in body, got: %s", s)
	}
}

func TestBuildFilterValuesBody_OrderByKey(t *testing.T) {
	body, err := BuildFilterValuesBody(FilterValuesRequest{ColID: "category"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, _ := json.Marshal(body)
	s := string(b)
	if !containsStr(s, `"_key"`) {
		t.Errorf("expected _key ordering in terms aggregation, got: %s", s)
	}
	if !containsStr(s, `"asc"`) {
		t.Errorf("expected asc ordering in terms aggregation, got: %s", s)
	}
}
