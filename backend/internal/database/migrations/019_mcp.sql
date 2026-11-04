-- MCP: connections that let Claude use the Ziv as a person. Tokens are stored
-- only as SHA-256 hashes.
CREATE TABLE IF NOT EXISTS mcp_clients (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    redirect_uris TEXT[] NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS mcp_auth_requests (
    id              TEXT PRIMARY KEY,
    client_id       TEXT REFERENCES mcp_clients(id) ON DELETE CASCADE,
    redirect_uri    TEXT,
    code_challenge  TEXT,
    state           TEXT,
    person_id       BIGINT NULL REFERENCES people(id) ON DELETE CASCADE,
    code_hash       TEXT UNIQUE NULL,
    code_expires_at TIMESTAMPTZ NULL,
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS mcp_connections (
    id                 BIGSERIAL PRIMARY KEY,
    person_id          BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    kind               TEXT NOT NULL CHECK (kind IN ('oauth', 'code')),
    name               TEXT NOT NULL,
    client_id          TEXT NULL,
    token_hash         TEXT UNIQUE NOT NULL,
    access_expires_at  TIMESTAMPTZ NULL,
    refresh_hash       TEXT UNIQUE NULL,
    refresh_expires_at TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at       TIMESTAMPTZ NULL,
    revoked_at         TIMESTAMPTZ NULL
);
CREATE INDEX IF NOT EXISTS mcp_connections_person_idx ON mcp_connections (person_id);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual', 'mcp'));
