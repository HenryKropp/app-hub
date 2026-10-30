package handler

import (
	"encoding/json"
	"net/http"

	"github.com/HenryKropp/app-hub/backend/internal/vault"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) checkCredential(w http.ResponseWriter, r *http.Request) {
	a, me, _, _, ok := h.appFor(w, r)
	if !ok || !h.credentialReady(w) || !h.allowExecution(w, r, a.ID) {
		return
	}
	id := r.PathValue("cid")
	if !credentialIDValid(id) {
		writeError(w, 404, "Credencial não encontrada.")
		return
	}
	var body struct {
		Environment string `json:"environment"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	// Write before acquiring a transaction connection. Otherwise concurrent
	// uses can exhaust the pool while waiting for a second audit connection.
	// Only copy a known, authorized ID from storage, never an arbitrary path
	// value: a client may accidentally put a token in place of an ID.
	var attempt int64
	err := h.q.SecurityDB().QueryRow(r.Context(), `INSERT INTO credential_audit(credential_id,actor_id,revision,action)
		SELECT id,$2,revision,'use.started' FROM credential_vault
		WHERE id=$1 AND app_id=$3 AND role_id=$4 AND environment=$5 AND NOT revoked RETURNING id`, id, me.ID, a.ID, me.RoleID, body.Environment).Scan(&attempt)
	if err == pgx.ErrNoRows {
		writeError(w, 404, "Credencial ativa não autorizada para este app e cargo.")
		return
	}
	if err != nil {
		credentialFailure(w)
		return
	}
	tx, err := h.credentialTx(r.Context())
	if err != nil {
		credentialFailure(w)
		return
	}
	defer tx.Rollback(r.Context())
	var c credentialMeta
	var endpoint string
	var raw []byte
	// The row lock is held until the upstream call and outcome audit finish.
	// Revocation/rotation waits for an in-flight use; after it returns no old
	// credential can start another request. Nothing is cached or given to jobs.
	err = tx.QueryRow(r.Context(), `SELECT id,app_id,environment,role_id,revision,endpoint,envelope
		FROM credential_vault WHERE id=$1 AND app_id=$2 AND NOT revoked AND role_id=$3 AND environment=$4 FOR UPDATE`, id, a.ID, me.RoleID, body.Environment).
		Scan(&c.ID, &c.AppID, &c.Environment, &c.RoleID, &c.Revision, &endpoint, &raw)
	if err != nil {
		_, _ = tx.Exec(r.Context(), `UPDATE credential_audit SET action='use.failed' WHERE id=$1`, attempt)
		_ = tx.Commit(r.Context())
		writeError(w, 404, "Credencial ativa não autorizada para este app e cargo.")
		return
	}
	// The durable attempt above survives a crash even if this transaction
	// cannot record the selected revision or the final outcome.
	_, err = tx.Exec(r.Context(), `UPDATE credential_audit SET revision=$2 WHERE id=$1`, attempt, c.Revision)
	if err != nil {
		credentialFailure(w)
		return
	}
	var envelope vault.Envelope
	status := 0
	if json.Unmarshal(raw, &envelope) != nil {
		err = vault.ErrEnvelope
	} else {
		err = h.credentialKeys.Consume(envelope, h.credentialIdentity(c, endpoint), func(value []byte) error {
			var e error
			status, e = vault.CheckBearer(r.Context(), endpoint, value)
			return e
		})
	}
	action := "use.completed"
	if err != nil {
		action = "use.failed"
	}
	// No value, destination, response content, headers or raw error is persisted.
	if _, e := tx.Exec(r.Context(), `INSERT INTO credential_audit(credential_id,actor_id,revision,action) VALUES($1,$2,$3,$4)`, c.ID, me.ID, c.Revision, action); e != nil || tx.Commit(r.Context()) != nil {
		credentialFailure(w)
		return
	}
	if err != nil {
		credentialFailure(w)
		return
	}
	writeJSON(w, 200, map[string]any{"upstream_status": status, "credential_revision": c.Revision})
}
