// Package opensearch provides a configured OpenSearch client.
package opensearch

import (
	"fmt"
	"os"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
)

// Connect creates an OpenSearch client from environment variables and verifies
// the connection with a ping.
func Connect() (*opensearchgo.Client, error) {
	url := getEnv("OPENSEARCH_URL", "http://localhost:9200")

	cfg := opensearchgo.Config{
		Addresses: []string{url},
	}

	user := os.Getenv("OPENSEARCH_USER")
	password := os.Getenv("OPENSEARCH_PASSWORD")
	if user != "" && password != "" {
		cfg.Username = user
		cfg.Password = password
	}

	client, err := opensearchgo.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create opensearch client: %w", err)
	}

	res, err := client.Ping()
	if err != nil {
		return nil, fmt.Errorf("ping opensearch: %w", err)
	}
	defer res.Body.Close() //nolint:errcheck
	if res.IsError() {
		return nil, fmt.Errorf("opensearch ping returned %s", res.Status())
	}

	return client, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
