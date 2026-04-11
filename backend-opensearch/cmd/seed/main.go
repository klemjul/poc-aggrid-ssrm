package main

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strconv"
	"time"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
	opensearchapi "github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/opensearch"
)

const batchSize   = 500
const logInterval = 10_000

var categories = []struct {
	name          string
	subcategories []string
}{
	{"Electronics", []string{"Phones", "Laptops", "Tablets", "Accessories", "Cameras"}},
	{"Clothing", []string{"Shirts", "Pants", "Dresses", "Jackets", "Shoes"}},
	{"Home & Garden", []string{"Furniture", "Decor", "Tools", "Lighting", "Plants"}},
	{"Sports", []string{"Fitness", "Outdoor", "Team Sports", "Water Sports", "Cycling"}},
	{"Books", []string{"Fiction", "Non-Fiction", "Science", "History", "Children"}},
	{"Toys", []string{"Action Figures", "Board Games", "Puzzles", "Dolls", "LEGO"}},
	{"Food & Beverage", []string{"Snacks", "Beverages", "Organic", "Frozen", "Bakery"}},
	{"Automotive", []string{"Parts", "Accessories", "Tools", "Tires", "Electronics"}},
}

var adjectives = []string{
	"Premium", "Deluxe", "Ultra", "Super", "Mega", "Pro", "Smart", "Classic",
	"Advanced", "Elite", "Essential", "Budget", "Standard", "Compact", "Portable",
}

var nouns = []string{
	"Widget", "Gadget", "Device", "Product", "Item", "Unit", "Module",
	"Component", "Bundle", "Kit", "Set", "Pack", "Series", "Edition", "Model",
}

func randomName(r *rand.Rand) string {
	return fmt.Sprintf("%s %s %04d",
		adjectives[r.IntN(len(adjectives))],
		nouns[r.IntN(len(nouns))],
		r.IntN(9999),
	)
}

func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

func newUUID() string {
	b := make([]byte, 16)
	if _, err := cryptorand.Read(b); err != nil {
		panic(fmt.Sprintf("generate uuid: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func countDocs(client *opensearchgo.Client, index string) (int, error) {
	res, err := client.Count(
		client.Count.WithIndex(index),
	)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	defer res.Body.Close() //nolint:errcheck
	if res.IsError() {
		return 0, nil // index may not exist yet
	}
	var cr struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&cr); err != nil {
		return 0, fmt.Errorf("decode count: %w", err)
	}
	return cr.Count, nil
}

func seed(client *opensearchgo.Client, index string, totalDocs int) error {
	// Use a fixed seed for deterministic data generation (same as the PostgreSQL seed).
	r := rand.New(rand.NewPCG(42, 0)) //nolint:gosec // deterministic seed, not security sensitive

	createdAt := time.Now().UTC()

	for start := 0; start < totalDocs; start += batchSize {
		end := start + batchSize
		if end > totalDocs {
			end = totalDocs
		}

		var buf bytes.Buffer
		for i := start; i < end; i++ {
			cat := categories[r.IntN(len(categories))]
			sub := cat.subcategories[r.IntN(len(cat.subcategories))]
			price := round2(float64(r.IntN(99900)+100) / 100.0)
			qty := r.IntN(1000) + 1
			rating := round2(float64(r.IntN(41)+10) / 10.0)

			id := newUUID()

			// Bulk action line
			action := map[string]any{"index": map[string]any{"_index": index, "_id": id}}
			if err := json.NewEncoder(&buf).Encode(action); err != nil {
				return fmt.Errorf("encode action: %w", err)
			}
			// Document line
			doc := map[string]any{
				"id":          id,
				"name":        randomName(r),
				"category":    cat.name,
				"subcategory": sub,
				"price":       price,
				"quantity":    qty,
				"rating":      rating,
				"created_at":  createdAt.Add(-time.Duration(i) * time.Second).Format(time.RFC3339),
			}
			if err := json.NewEncoder(&buf).Encode(doc); err != nil {
				return fmt.Errorf("encode doc: %w", err)
			}
		}

		res, err := opensearchapi.BulkRequest{
			Body: &buf,
		}.Do(context.Background(), client)
		if err != nil {
			return fmt.Errorf("bulk request: %w", err)
		}
		if res.IsError() {
			res.Body.Close() //nolint:errcheck
			return fmt.Errorf("bulk request returned %s", res.Status())
		}

		// Check for per-item errors in the bulk response.
		var br struct {
			Errors bool `json:"errors"`
			Items  []map[string]struct {
				Status int    `json:"status"`
				Error  string `json:"error"`
			} `json:"items"`
		}
		if err := json.NewDecoder(res.Body).Decode(&br); err != nil {
			res.Body.Close() //nolint:errcheck
			return fmt.Errorf("decode bulk response: %w", err)
		}
		res.Body.Close() //nolint:errcheck
		if br.Errors {
			return fmt.Errorf("bulk request contained errors at batch starting at %d", start)
		}

		if start%logInterval == 0 || end == totalDocs {
			log.Printf("indexed %d / %d documents", end, totalDocs)
		}
	}
	return nil
}

func main() {
	client, err := opensearch.Connect()
	if err != nil {
		log.Fatalf("connect to opensearch: %v", err)
	}

	index := getEnv("OPENSEARCH_INDEX", "products")

	// SEED_TOTAL allows overriding the default document count (useful in CI).
	totalDocs := 100_000
	if v := os.Getenv("SEED_TOTAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			totalDocs = n
		}
	}

	if err := opensearch.EnsureIndex(client, index); err != nil {
		log.Fatalf("ensure index: %v", err)
	}
	log.Println("index ready")

	count, err := countDocs(client, index)
	if err != nil {
		log.Fatalf("count docs: %v", err)
	}
	if count >= totalDocs {
		log.Printf("index already has %d documents, skipping seed", count)
		os.Exit(0)
	}

	log.Printf("seeding %d products…", totalDocs)
	if err := seed(client, index, totalDocs); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Println("seeding complete")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
