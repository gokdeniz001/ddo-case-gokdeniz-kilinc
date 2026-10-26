package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/siteyonet/siteyonet/internal/middleware"
)

type Executor interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

func (d *DB) Executor(ctx context.Context) Executor {
	if tx, ok := ctx.Value(middleware.TransactionKey).(pgx.Tx); ok {
		return tx
	}
	return d.Pool
}

func (d *DB) BeginTenant(ctx context.Context) (context.Context, pgx.Tx, error) {
	user := middleware.GetCurrentUser(ctx)
	if user == nil {
		return ctx, nil, errors.New("authentication required")
	}
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return ctx, nil, err
	}
	_, err = tx.Exec(ctx, `SET LOCAL ROLE siteyonet_runtime`)
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT set_config('app.bypass_rls','false',true),
		set_config('app.current_site_id',$1,true), set_config('app.current_user_id',$2,true),
		set_config('app.current_user_role',$3,true)`, user.SiteID.String(), user.UserID.String(), user.Role)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		return ctx, nil, err
	}
	return context.WithValue(ctx, middleware.TransactionKey, tx), tx, nil
}
