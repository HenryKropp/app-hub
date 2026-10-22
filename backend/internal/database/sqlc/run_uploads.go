package sqlc

import "context"

// ClearRunInputs removes uploaded file bodies and form values once a job ends.
// The manifest and immutable run/version metadata remain in their own records.
func (q *Queries) ClearRunInputs(ctx context.Context, id int64) error {
	_, err := q.db.Exec(ctx, `UPDATE agent_jobs SET payload=payload-'script' WHERE id=$1 AND kind='run' AND status IN ('done','failed')`, id)
	return err
}
