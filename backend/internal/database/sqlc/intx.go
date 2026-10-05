package sqlc

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// InTx runs fn with Queries bound to one transaction: all or nothing.
// The underlying connection may be a pool or already a transaction (savepoint).
func (q *Queries) InTx(ctx context.Context, fn func(*Queries) error) error {
	b, ok := q.db.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return errors.New("database connection cannot start a transaction")
	}
	tx, err := b.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
