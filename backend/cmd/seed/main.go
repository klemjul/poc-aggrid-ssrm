package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"

	"database/sql"

	_ "github.com/lib/pq"

	"github.com/klemjul/poc-aggrid-ssrm/backend/db"
	"github.com/klemjul/poc-aggrid-ssrm/backend/migration"
)

const totalRows = 10_000_000

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
		adjectives[r.Intn(len(adjectives))],
		nouns[r.Intn(len(nouns))],
		r.Intn(9999),
	)
}

func seed(database *sql.DB) error {
	r := rand.New(rand.NewSource(42)) //nolint:gosec // deterministic seed, not security sensitive

	const batchSize = 500
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}

		tx, err := database.Begin()
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}

		stmt, err := tx.Prepare(`
			INSERT INTO products (name, category, subcategory, price, quantity, rating)
			VALUES ($1, $2, $3, $4, $5, $6)`)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			return fmt.Errorf("prepare stmt: %w", err)
		}

		for i := start; i < end; i++ {
			cat := categories[r.Intn(len(categories))]
			sub := cat.subcategories[r.Intn(len(cat.subcategories))]
			price := math_round2(float64(r.Intn(99900)+100) / 100.0)
			qty := r.Intn(1000) + 1
			rating := math_round2(float64(r.Intn(41)+10) / 10.0)

			if _, err := stmt.Exec(randomName(r), cat.name, sub, price, qty, rating); err != nil {
				_ = stmt.Close()
				tx.Rollback() //nolint:errcheck
				return fmt.Errorf("insert row %d: %w", i, err)
			}
		}
		_ = stmt.Close()
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit batch: %w", err)
		}

		if start%10000 == 0 {
			log.Printf("inserted %d / %d rows", end, totalRows)
		}
	}
	return nil
}

func math_round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

func main() {
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer database.Close() //nolint:errcheck

	if err := migration.Apply(database); err != nil {
		log.Fatalf("apply migration: %v", err)
	}
	log.Println("migration applied")

	// Check if seeding is needed
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM products").Scan(&count); err != nil {
		log.Fatalf("count products: %v", err)
	}
	if count >= totalRows {
		log.Printf("database already has %d rows, skipping seed", count)
		os.Exit(0)
	}

	log.Printf("seeding %d products…", totalRows)
	if err := seed(database); err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Println("seeding complete")
}
