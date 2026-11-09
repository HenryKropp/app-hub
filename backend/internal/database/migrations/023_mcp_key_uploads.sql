-- Cofre pelo Claude: a one-time address where Claude sends an app's .env
-- straight from the person's computer. Only the address (as a hash) is kept;
-- the values go sealed into app_config and never into this table.
CREATE TABLE IF NOT EXISTS mcp_key_uploads (
    id          BIGSERIAL PRIMARY KEY,
    token_hash  TEXT NOT NULL UNIQUE,
    person_id   BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    app_id      BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS mcp_key_uploads_person ON mcp_key_uploads (person_id);
