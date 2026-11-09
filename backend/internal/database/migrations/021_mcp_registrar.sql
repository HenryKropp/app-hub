-- MCP Etapa 2: code sent through Claude. A version can come from an upload,
-- and records whether it came through Claude (channel 'mcp'). mcp_uploads
-- holds a one-time zip upload until registrar_app uses it.
ALTER TABLE app_versions DROP CONSTRAINT IF EXISTS app_versions_origin_check;
ALTER TABLE app_versions ADD CONSTRAINT app_versions_origin_check CHECK (origin IN ('original', 'folder', 'restore', 'upload'));
ALTER TABLE app_versions ADD COLUMN IF NOT EXISTS channel TEXT NOT NULL DEFAULT 'manual' CHECK (channel IN ('manual', 'mcp'));

CREATE TABLE IF NOT EXISTS mcp_uploads (
    id          BIGSERIAL PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,
    person_id   BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ,
    used_at     TIMESTAMPTZ,
    size        INT NOT NULL DEFAULT 0,
    content     BYTEA
);
CREATE INDEX IF NOT EXISTS mcp_uploads_person ON mcp_uploads (person_id);
