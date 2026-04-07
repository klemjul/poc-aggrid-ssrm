package migration

import (
	"database/sql"
	"fmt"
)

const schema = `
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS products (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT        NOT NULL,
    category    TEXT        NOT NULL,
    subcategory TEXT        NOT NULL,
    price       NUMERIC(10,2) NOT NULL,
    quantity    INT         NOT NULL,
    rating      NUMERIC(3,2) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// Apply runs the SQL migration against the given database.
func Apply(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}
