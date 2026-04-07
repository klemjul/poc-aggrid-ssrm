package main

import (
	"log"
	"net/http"
	"os"

	"github.com/rs/cors"

	"github.com/klemjul/poc-aggrid-ssrm/backend/api"
	"github.com/klemjul/poc-aggrid-ssrm/backend/db"
	"github.com/klemjul/poc-aggrid-ssrm/backend/migration"
)

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer database.Close()

	if err := migration.Apply(database); err != nil {
		log.Fatalf("apply migration: %v", err)
	}
	log.Println("database migration applied")

	h := &api.Handler{DB: database}

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
