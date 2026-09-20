package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// querier is satisfied by both *sql.DB and *sql.Tx, letting every read/write
// method below go through q(ctx) instead of s.db directly.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type txKeyType struct{}

var txKey = txKeyType{}

// q returns the transaction WithTx placed on ctx, or the store's shared
// connection if none is active.
func (s *Store) q(ctx context.Context) querier {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return s.db
}

// WithTx runs fn in a single database transaction: every Store method called
// with the ctx fn receives joins that same transaction via q(ctx). A non-nil
// return from fn rolls back and nothing persists; a nil return commits.
//
// Needed so executeAction (ROADMAP S6 c2) can revalidate preconditions and
// apply a fleet-state mutation atomically — a test asserts a mid-way failure
// leaves neither the unit nor the call changed. A WithTx call nested inside
// another reuses the outer transaction rather than starting a second one,
// since SQLite's single connection (DEC-002) cannot run nested transactions.
func (s *Store) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return fn(ctx)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: begin transaction: %w", err)
	}
	if err := fn(context.WithValue(ctx, txKey, tx)); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("sqlite: rollback after error (%v): %w", err, rbErr)
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: commit transaction: %w", err)
	}
	return nil
}
