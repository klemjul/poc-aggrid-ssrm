package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
	opensearchapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

const indexMapping = `{
  "settings": {
    "number_of_shards": 1,
    "number_of_replicas": 0
  },
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "name": {
        "type": "text",
        "fields": { "keyword": { "type": "keyword" } }
      },
      "category":    { "type": "keyword" },
      "subcategory": { "type": "keyword" },
      "price":       { "type": "float" },
      "quantity":    { "type": "integer" },
      "rating":      { "type": "float" },
      "created_at":  { "type": "date" }
    }
  }
}`

// EnsureIndex creates the products index with the correct mapping if it does not
// already exist. It is idempotent: a 400 "resource_already_exists_exception" is
// treated as success.
func EnsureIndex(client *opensearchgo.Client, index string) error {
	res, err := opensearchapi.IndicesExistsRequest{
		Index: []string{index},
	}.Do(context.Background(), client)
	if err != nil {
		return fmt.Errorf("check index exists: %w", err)
	}
	defer res.Body.Close() //nolint:errcheck

	if res.StatusCode == 200 {
		return nil
	}
	if res.StatusCode != 404 {
		return fmt.Errorf("check index exists returned %s", res.Status())
	}

	// Index does not exist — create it.
	res2, err := opensearchapi.IndicesCreateRequest{
		Index: index,
		Body:  bytes.NewBufferString(indexMapping),
	}.Do(context.Background(), client)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer res2.Body.Close() //nolint:errcheck

	if res2.IsError() {
		var e map[string]any
		if err2 := json.NewDecoder(res2.Body).Decode(&e); err2 == nil {
			if errorObj, ok := e["error"].(map[string]any); ok {
				if errType, ok := errorObj["type"].(string); ok && errType == "resource_already_exists_exception" {
					return nil
				}
			}
		}
		return fmt.Errorf("create index returned %s", res2.Status())
	}
	return nil
}
