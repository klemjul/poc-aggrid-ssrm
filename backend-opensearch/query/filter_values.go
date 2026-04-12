package query

import "fmt"

// allowedFilterValuesCols defines the columns for which global distinct values
// can be fetched. Only these columns are permitted to avoid injection.
var allowedFilterValuesCols = map[string]string{
	"category":    "category",
	"subcategory": "subcategory",
}

// FilterValuesRequest is the request body for the filter-values endpoint.
type FilterValuesRequest struct {
	ColID      string `json:"colId"`
	SearchText string `json:"searchText"`
	Limit      int    `json:"limit"`
}

// FilterValuesResponse is the response body for the filter-values endpoint.
type FilterValuesResponse struct {
	Values []string `json:"values"`
}

// maxFilterValuesLimit is the upper bound for the `limit` request parameter to
// prevent excessively large terms aggregations.
const maxFilterValuesLimit = 1000

// BuildFilterValuesBody builds the OpenSearch aggregation body for fetching
// global distinct values for a whitelisted column. If searchText is provided,
// it narrows results to values starting with that prefix (case-insensitive).
func BuildFilterValuesBody(req FilterValuesRequest) (map[string]any, error) {
	col, ok := allowedFilterValuesCols[req.ColID]
	if !ok {
		return nil, fmt.Errorf("disallowed colId for filter values: %s", req.ColID)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 200
	}
	if limit > maxFilterValuesLimit {
		limit = maxFilterValuesLimit
	}

	body := map[string]any{
		"size": 0,
		"aggs": map[string]any{
			"values": map[string]any{
				"terms": map[string]any{
					"field": col,
					"size":  limit,
					"order": map[string]any{"_key": "asc"},
				},
			},
		},
	}

	if req.SearchText != "" {
		body["query"] = map[string]any{
			"prefix": map[string]any{
				col: map[string]any{
					"value":            req.SearchText,
					"case_insensitive": true,
				},
			},
		}
	}

	return body, nil
}
