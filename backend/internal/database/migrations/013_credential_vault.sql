-- Separate from shared_config: no value-read API or agent injection.
CREATE TABLE credential_vault (
    id TEXT PRIMARY KEY,
    app_id BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    environment TEXT NOT NULL CHECK (environment IN ('development','staging','production')),
    role_id BIGINT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    envelope JSONB,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((revoked AND envelope IS NULL) OR (NOT revoked AND envelope IS NOT NULL))
);
CREATE INDEX credential_vault_app_idx ON credential_vault(app_id);

-- Only identifiers/outcome codes; never endpoint, headers or upstream content.
CREATE TABLE credential_audit (
    id BIGSERIAL PRIMARY KEY,
    credential_id TEXT NOT NULL,
    actor_id BIGINT NOT NULL,
    revision INTEGER NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('created','rotated','revoked','use.started','use.completed','use.failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
