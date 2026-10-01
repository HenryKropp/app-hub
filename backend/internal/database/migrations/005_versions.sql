-- 005_versions.sql: code versions, ficha changes, and runs linked to a version.
-- clock_timestamp() instead of now(): several rows written in one transaction
-- (tests) still get distinct, ordered times for the timeline.
CREATE TABLE IF NOT EXISTS code_blobs (
    sha256  TEXT PRIMARY KEY,
    content BYTEA NOT NULL,
    size    BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_versions (
    id               BIGSERIAL PRIMARY KEY,
    app_id           BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    number           INT NOT NULL,
    origin           TEXT NOT NULL CHECK (origin IN ('original', 'folder', 'restore')),
    restored_from    INT,
    author_id        BIGINT REFERENCES people(id),
    author_probable  BOOLEAN NOT NULL DEFAULT false,
    description      TEXT NOT NULL DEFAULT '',
    described_by     BIGINT REFERENCES people(id),
    described_at     TIMESTAMPTZ,
    files            JSONB NOT NULL,
    summary          JSONB NOT NULL,
    files_changed_at TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (app_id, number)
);
CREATE INDEX IF NOT EXISTS app_versions_app_idx ON app_versions(app_id, created_at DESC);

CREATE TABLE IF NOT EXISTS app_changes (
    id         BIGSERIAL PRIMARY KEY,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id),
    changes    JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS app_changes_app_idx ON app_changes(app_id, created_at DESC);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS version_id BIGINT REFERENCES app_versions(id);
