-- Governança (ADR 046).

-- name: GetGovernanceSettings :one
SELECT it_approval, version_rule, code_channel, mcp_restricted FROM security_settings WHERE id;

-- name: UpdateGovernanceSettings :exec
UPDATE security_settings
SET it_approval = sqlc.arg(it_approval), version_rule = sqlc.arg(version_rule),
    code_channel = sqlc.arg(code_channel), mcp_restricted = sqlc.arg(mcp_restricted)
WHERE id;

-- name: GetAppGovernance :one
SELECT * FROM app_governance WHERE app_id = $1;

-- name: UpsertAppGovernance :exec
INSERT INTO app_governance (app_id, approved_version, approved_risk, ever_approved)
VALUES (sqlc.arg(app_id), sqlc.narg(approved_version), sqlc.arg(approved_risk), sqlc.arg(ever_approved))
ON CONFLICT (app_id) DO UPDATE
SET approved_version = EXCLUDED.approved_version,
    approved_risk = EXCLUDED.approved_risk,
    ever_approved = app_governance.ever_approved OR EXCLUDED.ever_approved;

-- name: SetAppVersionRule :exec
INSERT INTO app_governance (app_id, version_rule) VALUES (sqlc.arg(app_id), sqlc.narg(version_rule))
ON CONFLICT (app_id) DO UPDATE SET version_rule = EXCLUDED.version_rule;

-- name: SetArtifactVersionRule :exec
UPDATE artifacts SET version_rule = sqlc.narg(version_rule) WHERE id = sqlc.arg(id);

-- name: SetArtifactEverApproved :exec
UPDATE artifacts SET ever_approved = $2 WHERE id = $1;

-- name: InsertApprovalRequest :one
INSERT INTO approval_requests (item_kind, app_id, artifact_id, reason, draft_id, app_version,
    artifact_version_id, target_area_id, risk, risk_added, channel, tested, status, requested_by, closed_at)
VALUES (sqlc.arg(item_kind), sqlc.narg(app_id), sqlc.narg(artifact_id), sqlc.arg(reason), sqlc.narg(draft_id),
    sqlc.narg(app_version), sqlc.narg(artifact_version_id), sqlc.narg(target_area_id), sqlc.arg(risk),
    sqlc.arg(risk_added), sqlc.arg(channel), sqlc.arg(tested), sqlc.arg(status), sqlc.narg(requested_by),
    CASE WHEN sqlc.arg(status)::text = 'open' THEN NULL ELSE clock_timestamp() END)
RETURNING *;

-- name: InsertApprovalStep :exec
INSERT INTO approval_steps (request_id, step, decision, decided_by, decided_at, on_behalf)
VALUES (sqlc.arg(request_id), sqlc.arg(step), sqlc.narg(decision), sqlc.narg(decided_by),
    CASE WHEN sqlc.narg(decision)::text IS NULL THEN NULL ELSE clock_timestamp() END, sqlc.arg(on_behalf));

-- name: GetOpenRequestForApp :one
SELECT * FROM approval_requests WHERE app_id = $1 AND status = 'open';

-- name: GetOpenRequestForArtifact :one
SELECT * FROM approval_requests WHERE artifact_id = $1 AND status = 'open';

-- name: GetApprovalRequestForUpdate :one
SELECT * FROM approval_requests WHERE id = $1 FOR UPDATE;

-- name: GetApprovalRequest :one
SELECT * FROM approval_requests WHERE id = $1;

-- name: ListApprovalSteps :many
SELECT s.*, COALESCE(p.name, '') AS decided_by_name
FROM approval_steps s LEFT JOIN people p ON p.id = s.decided_by
WHERE s.request_id = $1 ORDER BY CASE s.step WHEN 'leader' THEN 0 ELSE 1 END;

-- name: DecideApprovalStep :execrows
UPDATE approval_steps
SET decision = sqlc.arg(decision), decided_by = sqlc.arg(decided_by), decided_at = clock_timestamp(),
    reason = sqlc.arg(reason), on_behalf = sqlc.arg(on_behalf)
WHERE request_id = sqlc.arg(request_id) AND step = sqlc.arg(step) AND decision IS NULL;

-- name: AddMissingITStep :exec
-- Turning TI on adds its step to every open request.
INSERT INTO approval_steps (request_id, step)
SELECT r.id, 'it' FROM approval_requests r WHERE r.status = 'open' AND r.reason <> 'area'
ON CONFLICT DO NOTHING;

-- name: CloseApprovalRequest :exec
UPDATE approval_requests SET status = sqlc.arg(status), closed_at = clock_timestamp() WHERE id = sqlc.arg(id);

-- name: ReplaceOpenAppRequest :many
UPDATE approval_requests SET status = 'replaced', closed_at = clock_timestamp()
WHERE app_id = $1 AND status = 'open' RETURNING *;

-- name: ReplaceOpenArtifactRequest :many
UPDATE approval_requests SET status = 'replaced', closed_at = clock_timestamp()
WHERE artifact_id = $1 AND status = 'open' RETURNING *;

-- name: ListOpenRequests :many
SELECT * FROM approval_requests WHERE status = 'open' ORDER BY created_at;

-- name: ListAutoRequestsSince :many
SELECT * FROM approval_requests WHERE status = 'auto' AND created_at > $1 ORDER BY created_at DESC;

-- name: ListAppRequests :many
SELECT r.*, COALESCE(p.name, '') AS requested_by_name
FROM approval_requests r LEFT JOIN people p ON p.id = r.requested_by
WHERE r.app_id = $1 AND r.status <> 'replaced' ORDER BY r.id DESC LIMIT 200;

-- name: ListArtifactRequests :many
SELECT r.*, COALESCE(p.name, '') AS requested_by_name
FROM approval_requests r LEFT JOIN people p ON p.id = r.requested_by
WHERE r.artifact_id = $1 AND r.status <> 'replaced' ORDER BY r.id DESC LIMIT 200;

-- name: ListRecentRequests :many
SELECT r.*, COALESCE(p.name, '') AS requested_by_name
FROM approval_requests r LEFT JOIN people p ON p.id = r.requested_by
WHERE r.status <> 'replaced' AND r.created_at > now() - interval '90 days'
ORDER BY r.id DESC LIMIT 200;

-- name: InsertGovernanceEvent :exec
INSERT INTO governance_events (item_kind, app_id, artifact_id, kind, actor_id, detail)
VALUES (sqlc.narg(item_kind), sqlc.narg(app_id), sqlc.narg(artifact_id), sqlc.arg(kind), sqlc.narg(actor_id), sqlc.arg(detail));

-- name: ListGovernanceEvents :many
SELECT e.*, COALESCE(p.name, '') AS actor_name
FROM governance_events e LEFT JOIN people p ON p.id = e.actor_id
WHERE (sqlc.narg(app_id)::bigint IS NULL OR e.app_id = sqlc.narg(app_id))
  AND (sqlc.narg(artifact_id)::bigint IS NULL OR e.artifact_id = sqlc.narg(artifact_id))
ORDER BY e.id DESC LIMIT 200;

-- name: LatestApprovedArtifactVersion :one
SELECT * FROM artifact_versions WHERE artifact_id = $1 AND approved ORDER BY number DESC LIMIT 1;

-- name: SetArtifactVersionApproved :exec
UPDATE artifact_versions SET approved = $2 WHERE id = $1;

-- name: AreaHasLeader :one
SELECT EXISTS (SELECT 1 FROM area_leaders WHERE area_id = $1);

-- name: ListAreaLeaderIDs :many
SELECT person_id FROM area_leaders WHERE area_id = $1;
