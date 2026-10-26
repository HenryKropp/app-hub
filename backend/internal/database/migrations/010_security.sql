CREATE TABLE security_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    retention_days INTEGER NOT NULL DEFAULT 30 CHECK (retention_days BETWEEN 1 AND 365),
    require_app_approval BOOLEAN NOT NULL DEFAULT TRUE,
    block_embedded_secrets BOOLEAN NOT NULL DEFAULT TRUE,
    local_execution_enabled BOOLEAN NOT NULL DEFAULT TRUE
);
INSERT INTO security_settings (id) VALUES (TRUE);

CREATE TABLE app_security (
    app_id BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    classification TEXT NOT NULL DEFAULT 'internal' CHECK (classification IN ('internal','confidential','restricted')),
    purpose TEXT NOT NULL DEFAULT '',
    external_domains TEXT[] NOT NULL DEFAULT '{}',
    approved BOOLEAN NOT NULL DEFAULT FALSE,
    reviewed_manifest_hash TEXT NOT NULL DEFAULT '',
    reviewed_version INTEGER NOT NULL DEFAULT 0,
    suspended BOOLEAN NOT NULL DEFAULT FALSE,
    reviewed_by BIGINT REFERENCES people(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ
);

CREATE TABLE security_audit (
    id BIGSERIAL PRIMARY KEY,
    actor_id BIGINT,
    action TEXT NOT NULL,
    route TEXT NOT NULL DEFAULT '',
    status INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX security_audit_created_idx ON security_audit (created_at DESC);
