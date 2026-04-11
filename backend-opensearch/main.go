package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rs/cors"

	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/api"
	"github.com/klemjul/poc-aggrid-ssrm/backend-opensearch/opensearch"
)

func main() {
	client, err := opensearch.Connect()
	if err != nil {
		log.Fatalf("connect to opensearch: %v", err)
	}

	index := getEnv("OPENSEARCH_INDEX", "products")
	if err := opensearch.EnsureIndex(client, index); err != nil {
		log.Fatalf("ensure index: %v", err)
	}
	log.Printf("opensearch index %q ready", index)

	h := &api.Handler{Client: client, Index: index}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/search-products", h.SearchProducts)
	mux.HandleFunc("/healthz", api.HealthCheck)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler(mux)

	addr := ":" + getEnv("PORT", "8080")
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, corsHandler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
