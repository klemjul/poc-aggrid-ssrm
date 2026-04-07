// Package query translates AG-Grid Server Side Row Model requests into
// PostgreSQL queries against the products table.
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

// TextFilter holds a text column filter from AG-Grid.
type TextFilter struct {
	FilterType string `json:"filterType"`
	Type       string `json:"type"`
	Filter     string `json:"filter"`
}

// NumberFilter holds a number column filter from AG-Grid.
type NumberFilter struct {
	FilterType string  `json:"filterType"`
	Type       string  `json:"type"`
	Filter     float64 `json:"filter"`
	FilterTo   float64 `json:"filterTo"`
}

// FilterModel represents a single column filter entry from AG-Grid.
// AG-Grid sends either text or number filters; we decode lazily in the builder.
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

// allowedColumns maps user-provided column names to hardcoded SQL column identifiers.
// Column names are NEVER taken directly from user input in SQL strings — the value
// from this map (a compile-time constant) is used instead.
var allowedColumns = map[string]string{
	"id":          "id",
	"name":        "name",
	"category":    "category",
	"subcategory": "subcategory",
	"price":       "price",
	"quantity":    "quantity",
	"rating":      "rating",
	"created_at":  "created_at",
}

// leafColumns are the columns selected for leaf-level (non-grouped) rows.
var leafColumns = []string{
	"id", "name", "category", "subcategory",
	"price", "quantity", "rating",
	"created_at::text AS created_at",
}

// safeCol looks up a column name in the allowedColumns whitelist and returns the
// hardcoded SQL identifier (not the user-provided value). Returns an error if the
// column name is not in the whitelist.
func safeCol(userCol string) (string, error) {
	if col, ok := allowedColumns[userCol]; ok {
		return col, nil
	}
	return "", fmt.Errorf("disallowed column: %s", userCol)
}

// BuildDataQuery returns the main data SELECT and a matching COUNT query together
// with the positional argument slice.
func BuildDataQuery(req SearchRequest) (dataSQL, countSQL string, args []any, err error) {
	grouping := isGrouping(req)
	groupCol := ""
	if grouping {
		groupCol, err = safeCol(req.RowGroupCols[len(req.GroupKeys)].Field)
		if err != nil {
			return "", "", nil, err
		}
	}

	var conditions []string
	args = []any{}

	// --- WHERE clauses from filterModel ---
	filterSQL, filterArgs, err := buildFilter(req.FilterModel, &args)
	if err != nil {
		return "", "", nil, err
	}
	if filterSQL != "" {
		conditions = append(conditions, filterSQL)
	}
	args = filterArgs

	// --- WHERE clauses from groupKeys (drill-down) ---
	for i, key := range req.GroupKeys {
		col, cerr := safeCol(req.RowGroupCols[i].Field)
		if cerr != nil {
			return "", "", nil, cerr
		}
		args = append(args, key)
		conditions = append(conditions, fmt.Sprintf("%s = $%d", col, len(args)))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// --- ORDER BY ---
	orderClause, err := buildOrder(req.SortModel, grouping, groupCol)
	if err != nil {
		return "", "", nil, err
	}

	// --- LIMIT / OFFSET ---
	limit := req.EndRow - req.StartRow
	if limit <= 0 {
		limit = 100
	}
	offset := req.StartRow

	// --- Build SELECT ---
	if grouping {
		dataSQL = fmt.Sprintf(
			"SELECT %s FROM products %s GROUP BY %s %s LIMIT %d OFFSET %d",
			groupCol, whereClause, groupCol, orderClause, limit, offset,
		)
		countSQL = fmt.Sprintf(
			"SELECT COUNT(DISTINCT %s) FROM products %s",
			groupCol, whereClause,
		)
	} else {
		cols := strings.Join(leafColumns, ", ")
		dataSQL = fmt.Sprintf(
			"SELECT %s FROM products %s %s LIMIT %d OFFSET %d",
			cols, whereClause, orderClause, limit, offset,
		)
		countSQL = fmt.Sprintf("SELECT COUNT(*) FROM products %s", whereClause)
	}

	return dataSQL, countSQL, args, nil
}

// isGrouping returns true when the request is asking for group-level rows
// (not yet at leaf level).
func isGrouping(req SearchRequest) bool {
	return len(req.RowGroupCols) > 0 && len(req.GroupKeys) < len(req.RowGroupCols)
}

// buildFilter converts filterModel entries into SQL fragments.
func buildFilter(filterModel map[string]FilterModel, args *[]any) (string, []any, error) {
	var parts []string
	for userCol, fm := range filterModel {
		col, err := safeCol(userCol)
		if err != nil {
			return "", nil, err
		}
		part, err := buildFilterEntry(col, fm, args)
		if err != nil {
			return "", nil, err
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, " AND "), *args, nil
}

func buildFilterEntry(col string, fm FilterModel, args *[]any) (string, error) {
	switch fm.FilterType {
	case "text":
		return buildTextFilter(col, fm, args)
	case "number":
		return buildNumberFilter(col, fm, args)
	default:
		return "", nil
	}
}

func buildTextFilter(col string, fm FilterModel, args *[]any) (string, error) {
	val, ok := fm.Filter.(string)
	if !ok {
		return "", nil
	}
	switch fm.Type {
	case "equals":
		*args = append(*args, val)
		return fmt.Sprintf("%s = $%d", col, len(*args)), nil
	case "notEqual":
		*args = append(*args, val)
		return fmt.Sprintf("%s != $%d", col, len(*args)), nil
	case "contains":
		*args = append(*args, "%"+val+"%")
		return fmt.Sprintf("%s ILIKE $%d", col, len(*args)), nil
	case "notContains":
		*args = append(*args, "%"+val+"%")
		return fmt.Sprintf("%s NOT ILIKE $%d", col, len(*args)), nil
	case "startsWith":
		*args = append(*args, val+"%")
		return fmt.Sprintf("%s ILIKE $%d", col, len(*args)), nil
	case "endsWith":
		*args = append(*args, "%"+val)
		return fmt.Sprintf("%s ILIKE $%d", col, len(*args)), nil
	default:
		return "", nil
	}
}

func buildNumberFilter(col string, fm FilterModel, args *[]any) (string, error) {
	val, ok := toFloat64(fm.Filter)
	if !ok {
		return "", nil
	}
	switch fm.Type {
	case "equals":
		*args = append(*args, val)
		return fmt.Sprintf("%s = $%d", col, len(*args)), nil
	case "notEqual":
		*args = append(*args, val)
		return fmt.Sprintf("%s != $%d", col, len(*args)), nil
	case "greaterThan":
		*args = append(*args, val)
		return fmt.Sprintf("%s > $%d", col, len(*args)), nil
	case "greaterThanOrEqual":
		*args = append(*args, val)
		return fmt.Sprintf("%s >= $%d", col, len(*args)), nil
	case "lessThan":
		*args = append(*args, val)
		return fmt.Sprintf("%s < $%d", col, len(*args)), nil
	case "lessThanOrEqual":
		*args = append(*args, val)
		return fmt.Sprintf("%s <= $%d", col, len(*args)), nil
	case "inRange":
		*args = append(*args, val, fm.FilterTo)
		return fmt.Sprintf("%s BETWEEN $%d AND $%d", col, len(*args)-1, len(*args)), nil
	default:
		return "", nil
	}
}

func buildOrder(sortModel []SortModel, grouping bool, groupCol string) (string, error) {
	if len(sortModel) == 0 {
		if grouping {
			return fmt.Sprintf("ORDER BY %s ASC", groupCol), nil
		}
		return "ORDER BY created_at DESC", nil
	}
	var parts []string
	for _, s := range sortModel {
		col, err := safeCol(s.ColID)
		if err != nil {
			return "", err
		}
		dir := "ASC"
		if strings.ToLower(s.Sort) == "desc" {
			dir = "DESC"
		}
		parts = append(parts, fmt.Sprintf("%s %s", col, dir))
	}
	return "ORDER BY " + strings.Join(parts, ", "), nil
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
