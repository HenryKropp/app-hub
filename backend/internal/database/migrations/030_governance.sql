-- Governança (ADR 046): approval requests with steps (the area leader always;
-- TI when the company turns it on), a version rule per company and per item,
-- the channel code may arrive by, and who may hold an MCP key.

ALTER TABLE security_settings ADD COLUMN IF NOT EXISTS it_approval BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE security_settings ADD COLUMN IF NOT EXISTS version_rule TEXT NOT NULL DEFAULT 'risk'
    CHECK (version_rule IN ('every', 'risk', 'first'));
ALTER TABLE security_settings ADD COLUMN IF NOT EXISTS code_channel TEXT NOT NULL DEFAULT 'any'
    CHECK (code_channel IN ('any', 'mcp'));
ALTER TABLE security_settings ADD COLUMN IF NOT EXISTS mcp_restricted BOOLEAN NOT NULL DEFAULT FALSE;

-- What of each app is approved. approved_version NULL = nothing approved yet.
CREATE TABLE IF NOT EXISTS app_governance (
    app_id           BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    approved_version INT,
    approved_risk    JSONB NOT NULL DEFAULT '{}',
    version_rule     TEXT CHECK (version_rule IN ('every', 'risk', 'first')),
    ever_approved    BOOLEAN NOT NULL DEFAULT FALSE
);

ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS version_rule TEXT CHECK (version_rule IN ('every', 'risk', 'first'));
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS ever_approved BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE artifact_versions ADD COLUMN IF NOT EXISTS approved BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE artifact_versions ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'manual'
    CHECK (channel IN ('manual', 'mcp'));

CREATE TABLE IF NOT EXISTS approval_requests (
    id                  BIGSERIAL PRIMARY KEY,
    item_kind           TEXT NOT NULL CHECK (item_kind IN ('app', 'artifact')),
    app_id              BIGINT REFERENCES apps(id) ON DELETE CASCADE,
    artifact_id         BIGINT REFERENCES artifacts(id) ON DELETE CASCADE,
    reason              TEXT NOT NULL CHECK (reason IN ('new', 'version', 'ficha', 'area')),
    draft_id            BIGINT REFERENCES app_drafts(id) ON DELETE SET NULL,
    app_version         INT,
    artifact_version_id BIGINT REFERENCES artifact_versions(id) ON DELETE CASCADE,
    target_area_id      BIGINT REFERENCES areas(id) ON DELETE CASCADE,
    risk                JSONB NOT NULL DEFAULT '{}',
    risk_added          JSONB NOT NULL DEFAULT '[]',
    channel             TEXT NOT NULL DEFAULT 'manual' CHECK (channel IN ('manual', 'mcp')),
    tested              BOOLEAN NOT NULL DEFAULT FALSE,
    status              TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'rejected', 'replaced', 'auto')),
    requested_by        BIGINT REFERENCES people(id) ON DELETE SET NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    closed_at           TIMESTAMPTZ,
    CHECK ((item_kind = 'app') = (app_id IS NOT NULL) AND (item_kind = 'artifact') = (artifact_id IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS approval_requests_one_open
    ON approval_requests (item_kind, COALESCE(app_id, artifact_id)) WHERE status = 'open';
CREATE INDEX IF NOT EXISTS approval_requests_app_idx ON approval_requests (app_id, id DESC);
CREATE INDEX IF NOT EXISTS approval_requests_artifact_idx ON approval_requests (artifact_id, id DESC);

CREATE TABLE IF NOT EXISTS approval_steps (
    request_id BIGINT NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    step       TEXT NOT NULL CHECK (step IN ('leader', 'it')),
    decision   TEXT CHECK (decision IN ('approved', 'rejected')),
    decided_by BIGINT REFERENCES people(id) ON DELETE SET NULL,
    decided_at TIMESTAMPTZ,
    reason     TEXT NOT NULL DEFAULT '',
    on_behalf  TEXT NOT NULL DEFAULT '' CHECK (on_behalf IN ('', 'admin_no_leader', 'self')),
    PRIMARY KEY (request_id, step)
);

-- Audience, area, rule, settings and MCP-key changes: who, when, before/after.
CREATE TABLE IF NOT EXISTS governance_events (
    id          BIGSERIAL PRIMARY KEY,
    item_kind   TEXT CHECK (item_kind IN ('app', 'artifact')),
    app_id      BIGINT REFERENCES apps(id) ON DELETE CASCADE,
    artifact_id BIGINT REFERENCES artifacts(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('audience', 'area', 'rule', 'settings', 'mcp')),
    actor_id    BIGINT REFERENCES people(id) ON DELETE SET NULL,
    detail      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS governance_events_created_idx ON governance_events (created_at DESC);

ALTER TABLE app_drafts DROP CONSTRAINT IF EXISTS app_drafts_status_check;
ALTER TABLE app_drafts ADD CONSTRAINT app_drafts_status_check
    CHECK (status IN ('rascunho', 'testando', 'aprovada', 'reprovada', 'no_ar', 'cancelada', 'aguardando'));

-- Carry today's state over. A company that required approval stays strict.
UPDATE security_settings
SET it_approval = require_app_approval,
    version_rule = CASE WHEN require_app_approval THEN 'every' ELSE 'risk' END
WHERE NOT EXISTS (SELECT 1 FROM app_governance);

UPDATE roles SET permissions = array_append(permissions, 'approve_it')
WHERE key = 'admin' AND NOT ('approve_it' = ANY(permissions))
  AND (SELECT require_app_approval FROM security_settings);

UPDATE roles SET permissions = array_append(permissions, 'use_mcp')
WHERE NOT ('use_mcp' = ANY(permissions));

INSERT INTO app_governance (app_id, approved_version, ever_approved)
SELECT a.id,
       CASE WHEN NOT st.require_app_approval
              OR COALESCE(s.approved AND s.reviewed_manifest_hash = encode(sha256(convert_to(a.manifest::text, 'UTF8')), 'hex')
                          AND s.reviewed_version = COALESCE((SELECT max(number) FROM app_versions WHERE app_id = a.id), 0), FALSE)
            THEN COALESCE((SELECT max(number) FROM app_versions WHERE app_id = a.id), 0) END,
       NOT st.require_app_approval OR COALESCE(s.approved AND s.reviewed_at IS NOT NULL, FALSE)
FROM apps a
CROSS JOIN security_settings st
LEFT JOIN app_security s ON s.app_id = a.id
ON CONFLICT (app_id) DO NOTHING;

WITH waiting AS (
    INSERT INTO approval_requests (item_kind, app_id, reason, app_version, status)
    SELECT 'app', g.app_id, CASE WHEN g.ever_approved THEN 'version' ELSE 'new' END,
           (SELECT max(number) FROM app_versions WHERE app_id = g.app_id), 'open'
    FROM app_governance g
    WHERE g.approved_version IS NULL
      AND NOT EXISTS (SELECT 1 FROM approval_requests r WHERE r.app_id = g.app_id AND r.status = 'open')
    RETURNING id
)
INSERT INTO approval_steps (request_id, step)
SELECT w.id, st.step
FROM waiting w
CROSS JOIN (VALUES ('leader'), ('it')) AS st(step)
WHERE st.step = 'leader' OR (SELECT it_approval FROM security_settings);
