package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (d *DB) Migrate(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(417203)`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS "fy-schema-migrations" (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	// Adopt databases created before migration version tracking was introduced.
	var existing bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass('public."fy-units"') IS NOT NULL`).Scan(&existing); err != nil {
		return err
	}
	if existing {
		if _, err = tx.Exec(ctx, `INSERT INTO "fy-schema-migrations" (name) VALUES ('001_initial_schema.sql'),('002_rls_policies.sql') ON CONFLICT DO NOTHING`); err != nil {
			return err
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		if filepath.Ext(file.Name()) != ".sql" {
			continue
		}
		var applied bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT FROM "fy-schema-migrations" WHERE name=$1)`, file.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(data)); err != nil {
			return fmt.Errorf("migration %s: %w", file.Name(), err)
		}
		if _, err = tx.Exec(ctx, `INSERT INTO "fy-schema-migrations" (name) VALUES ($1)`, file.Name()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
