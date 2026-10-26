package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/siteyonet/siteyonet/internal/middleware"
)

type DB struct {
	Pool *pgxpool.Pool
}

func NewDB(databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing db url: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping db failed: %w", err)
	}

	log.Println("PostgreSQL connection pool initialized successfully")
	return &DB{Pool: pool}, nil
}

// WithTenantTx starts a transaction and sets the RLS context variables
func (d *DB) WithTenantTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	user := middleware.GetCurrentUser(ctx)

	var tx pgx.Tx
	var err error
	if parent, ok := ctx.Value(middleware.TransactionKey).(pgx.Tx); ok {
		tx, err = parent.Begin(ctx)
	} else {
		tx, err = d.Pool.Begin(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if user != nil {
		// Set RLS session variables
		siteID := user.SiteID.String()
		userID := user.UserID.String()
		role := user.Role

		_, err = tx.Exec(ctx, `
			SELECT set_config('app.current_site_id', $1, true),
			       set_config('app.current_user_id', $2, true),
			       set_config('app.current_user_role', $3, true)
		`, siteID, userID, role)
		if err != nil {
			return fmt.Errorf("failed to set rls tenant context: %w", err)
		}
	} else {
		// Anonymous / Superadmin bypass
		_, _ = tx.Exec(ctx, "SELECT set_config('app.bypass_rls', 'true', true)")
	}

	if err := fn(tx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// LogAudit logs an event to "fy-audit-logs" table
func (d *DB) LogAudit(ctx context.Context, action, entityTable string, entityID string, oldVals, newVals interface{}) {
	user := middleware.GetCurrentUser(ctx)

	var siteID, userID *string
	if user != nil {
		s, u := user.SiteID.String(), user.UserID.String()
		siteID, userID = &s, &u
	}
	_, err := d.Executor(ctx).Exec(ctx, `INSERT INTO "fy-audit-logs"
		(site_id,user_id,action,entity_table,entity_id,old_values,new_values)
		VALUES ($1::uuid,$2::uuid,$3,$4,$5::uuid,$6,$7)`, siteID, userID, action, entityTable, entityID, oldVals, newVals)
	if err != nil {
		log.Printf("audit insert failed: %v", err)
	}

}

// CheckHealth verifies DB connectivity
func (d *DB) CheckHealth(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}
