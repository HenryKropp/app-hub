package sqlc

// SecurityDB exposes the same workspace-bound executor used by generated
// queries. It must never be replaced by the portal's registry connection.
func (q *Queries) SecurityDB() DBTX { return q.db }
