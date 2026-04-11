// Package query translates AG-Grid Server Side Row Model requests into
// OpenSearch query DSL queries against the products index.
package query

import (
	"fmt"
	"strings"
)

// ColumnVO represents an AG-Grid column value object.
type ColumnVO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Field       string `json:"field"`
	AggFunc     string `json:"aggFunc"`
}

// SortModel represents a single sort instruction from AG-Grid.
type SortModel struct {
	ColID string `json:"colId"`
	Sort  string `json:"sort"`
}

// FilterModel represents a single column filter entry from AG-Grid.
type FilterModel struct {
	FilterType string  `json:"filterType"`
	Type       string  `json:"type"`
	Filter     any     `json:"filter"`
	FilterTo   float64 `json:"filterTo"`
}

// SearchRequest mirrors the AG-Grid SSRM request body.
type SearchRequest struct {
	StartRow     int                    `json:"startRow"`
	EndRow       int                    `json:"endRow"`
	SortModel    []SortModel            `json:"sortModel"`
	FilterModel  map[string]FilterModel `json:"filterModel"`
	RowGroupCols []ColumnVO             `json:"rowGroupCols"`
	GroupKeys    []string               `json:"groupKeys"`
	ValueCols    []ColumnVO             `json:"valueCols"`
}

// SearchResult is the AG-Grid SSRM response body.
type SearchResult struct {
	Rows    []map[string]any `json:"rows"`
	LastRow int              `json:"lastRow"`
}

// allowedFields maps user-provided column names to OpenSearch field names.
// Field names are NEVER taken directly from user input — the value from this
// map (a compile-time constant) is used instead.
var allowedFields = map[string]string{
	"id":          "id",
	"name":        "name",
	"category":    "category",
	"subcategory": "subcategory",
	"price":       "price",
	"quantity":    "quantity",
	"rating":      "rating",
	"created_at":  "created_at",
}

// textFields are fields stored as OpenSearch `text` type. Filtering and sorting
// on text fields must target the `.keyword` multi-field.
var textFields = map[string]bool{
	"name": true,
}

// safeField looks up a field name in the allowedFields whitelist and returns the
// hardcoded OpenSearch field name (not the user-provided value). Returns an error
// if the column name is not in the whitelist.
func safeField(userField string) (string, error) {
	if field, ok := allowedFields[userField]; ok {
		return field, nil
	}
	return "", fmt.Errorf("disallowed field: %s", userField)
}

// filterField returns the field path used for term/wildcard queries. For text
// fields this is the ".keyword" sub-field; for keyword/numeric fields it is the
// field itself.
func filterField(field string) string {
	if textFields[field] {
		return field + ".keyword"
	}
	return field
}

// BuildSearchBody builds the OpenSearch request body for the given SearchRequest.
// It returns the body map and whether the query is a group-level query.
func BuildSearchBody(req SearchRequest) (map[string]any, bool, error) {
	grouping := isGrouping(req)

	filterClauses, err := buildFilterClauses(req)
	if err != nil {
		return nil, false, err
	}

	var query map[string]any
	if len(filterClauses) > 0 {
		query = map[string]any{
			"bool": map[string]any{
				"filter": filterClauses,
			},
		}
	} else {
		query = map[string]any{"match_all": map[string]any{}}
	}

	if grouping {
		body, err := buildGroupBody(req, query)
		return body, true, err
	}
	body, err := buildLeafBody(req, query)
	return body, false, err
}

func buildLeafBody(req SearchRequest, query map[string]any) (map[string]any, error) {
	size := req.EndRow - req.StartRow
	if size <= 0 {
		size = 100
	}

	sort, err := buildSort(req.SortModel, false, "")
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"from":             req.StartRow,
		"size":             size,
		"query":            query,
		"sort":             sort,
		"track_total_hits": true,
		"_source": []string{
			"id", "name", "category", "subcategory",
			"price", "quantity", "rating", "created_at",
		},
	}, nil
}

func buildGroupBody(req SearchRequest, query map[string]any) (map[string]any, error) {
	groupField, err := safeField(req.RowGroupCols[len(req.GroupKeys)].Field)
	if err != nil {
		return nil, err
	}

	termsOrder, err := buildTermsOrder(req.SortModel, groupField)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"size":  0,
		"query": query,
		"aggs": map[string]any{
			"groups": map[string]any{
				"terms": map[string]any{
					"field": groupField,
					// size 10000 is generous for the expected cardinality of groupable fields
					// (e.g. ~8 categories, ~40 subcategories). For high-cardinality fields
					// a composite aggregation with cursor-based pagination would be required.
					"size":  10000,
					"order": termsOrder,
				},
			},
			"total_count": map[string]any{
				"cardinality": map[string]any{
					"field": groupField,
				},
			},
		},
	}, nil
}

// isGrouping returns true when the request is asking for group-level rows
// (not yet at leaf level).
func isGrouping(req SearchRequest) bool {
	return len(req.RowGroupCols) > 0 && len(req.GroupKeys) < len(req.RowGroupCols)
}

func buildFilterClauses(req SearchRequest) ([]any, error) {
	var clauses []any

	for userCol, fm := range req.FilterModel {
		field, err := safeField(userCol)
		if err != nil {
			return nil, err
		}
		clause, err := buildFilterClause(field, fm)
		if err != nil {
			return nil, err
		}
		if clause != nil {
			clauses = append(clauses, clause)
		}
	}

	// Group key drill-down: one term filter per resolved key.
	for i, key := range req.GroupKeys {
		field, err := safeField(req.RowGroupCols[i].Field)
		if err != nil {
			return nil, err
		}
		clauses = append(clauses, map[string]any{
			"term": map[string]any{field: key},
		})
	}

	return clauses, nil
}

func buildFilterClause(field string, fm FilterModel) (any, error) {
	switch fm.FilterType {
	case "text":
		return buildTextClause(field, fm)
	case "number":
		return buildNumberClause(field, fm)
	default:
		return nil, nil
	}
}

func buildTextClause(field string, fm FilterModel) (any, error) {
	val, ok := fm.Filter.(string)
	if !ok {
		return nil, nil
	}
	kf := filterField(field)
	switch fm.Type {
	case "equals":
		return map[string]any{"term": map[string]any{kf: val}}, nil
	case "notEqual":
		return map[string]any{
			"bool": map[string]any{
				"must_not": []any{map[string]any{"term": map[string]any{kf: val}}},
			},
		}, nil
	case "contains":
		return map[string]any{
			"wildcard": map[string]any{
				kf: map[string]any{"value": "*" + val + "*", "case_insensitive": true},
			},
		}, nil
	case "notContains":
		return map[string]any{
			"bool": map[string]any{
				"must_not": []any{map[string]any{
					"wildcard": map[string]any{
						kf: map[string]any{"value": "*" + val + "*", "case_insensitive": true},
					},
				}},
			},
		}, nil
	case "startsWith":
		return map[string]any{
			"wildcard": map[string]any{
				kf: map[string]any{"value": val + "*", "case_insensitive": true},
			},
		}, nil
	case "endsWith":
		return map[string]any{
			"wildcard": map[string]any{
				kf: map[string]any{"value": "*" + val, "case_insensitive": true},
			},
		}, nil
	default:
		return nil, nil
	}
}

func buildNumberClause(field string, fm FilterModel) (any, error) {
	val, ok := toFloat64(fm.Filter)
	if !ok {
		return nil, nil
	}
	switch fm.Type {
	case "equals":
		return map[string]any{"term": map[string]any{field: val}}, nil
	case "notEqual":
		return map[string]any{
			"bool": map[string]any{
				"must_not": []any{map[string]any{"term": map[string]any{field: val}}},
			},
		}, nil
	case "greaterThan":
		return map[string]any{"range": map[string]any{field: map[string]any{"gt": val}}}, nil
	case "greaterThanOrEqual":
		return map[string]any{"range": map[string]any{field: map[string]any{"gte": val}}}, nil
	case "lessThan":
		return map[string]any{"range": map[string]any{field: map[string]any{"lt": val}}}, nil
	case "lessThanOrEqual":
		return map[string]any{"range": map[string]any{field: map[string]any{"lte": val}}}, nil
	case "inRange":
		return map[string]any{
			"range": map[string]any{
				field: map[string]any{"gte": val, "lte": fm.FilterTo},
			},
		}, nil
	default:
		return nil, nil
	}
}

// buildSort produces the OpenSearch sort parameter for leaf queries.
func buildSort(sortModel []SortModel, grouping bool, groupField string) ([]any, error) {
	if len(sortModel) == 0 {
		if grouping {
			return []any{map[string]any{groupField: map[string]any{"order": "asc"}}}, nil
		}
		return []any{map[string]any{"created_at": map[string]any{"order": "desc"}}}, nil
	}
	var parts []any
	for _, s := range sortModel {
		field, err := safeField(s.ColID)
		if err != nil {
			return nil, err
		}
		dir := "asc"
		if strings.ToLower(s.Sort) == "desc" {
			dir = "desc"
		}
		// Text fields must be sorted on the keyword sub-field.
		sf := field
		if textFields[field] {
			sf = field + ".keyword"
		}
		parts = append(parts, map[string]any{sf: map[string]any{"order": dir}})
	}
	return parts, nil
}

// buildTermsOrder produces the "order" clause for a terms aggregation.
func buildTermsOrder(sortModel []SortModel, groupField string) (map[string]any, error) {
	if len(sortModel) == 0 {
		return map[string]any{"_key": "asc"}, nil
	}
	s := sortModel[0]
	field, err := safeField(s.ColID)
	if err != nil {
		return nil, err
	}
	dir := "asc"
	if strings.ToLower(s.Sort) == "desc" {
		dir = "desc"
	}
	if field == groupField {
		return map[string]any{"_key": dir}, nil
	}
	return map[string]any{"_count": dir}, nil
}

// toFloat64 converts JSON number types to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
