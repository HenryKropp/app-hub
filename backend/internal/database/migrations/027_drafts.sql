-- Rascunho de versão (ADR 043): a version sent to Ziv can wait as a draft
-- without touching what runs. Only when it is published does it get a number
-- in app_versions, so "the latest version" keeps meaning "the one live".
CREATE TABLE IF NOT EXISTS app_drafts (
    id             BIGSERIAL PRIMARY KEY,
    app_id         BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    author_id      BIGINT REFERENCES people(id),
    channel        TEXT NOT NULL CHECK (channel IN ('manual', 'mcp')),
    files          JSONB NOT NULL,
    ficha          JSONB,
    note           TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'rascunho'
                   CHECK (status IN ('rascunho', 'testando', 'aprovada', 'reprovada', 'no_ar', 'cancelada')),
    status_reason  TEXT NOT NULL DEFAULT '',
    version_number INT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX IF NOT EXISTS app_drafts_app_created ON app_drafts (app_id, created_at DESC);

-- NULL for versions older than this column; false = "não testada"; true = "testada".
ALTER TABLE app_versions ADD COLUMN IF NOT EXISTS tested BOOLEAN;
