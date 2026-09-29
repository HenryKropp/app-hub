CREATE TABLE IF NOT EXISTS apps (
    id           BIGSERIAL PRIMARY KEY,
    slug         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    kind         TEXT NOT NULL,
    language     TEXT NOT NULL DEFAULT '',
    area         TEXT NOT NULL DEFAULT '',
    folder_path  TEXT NOT NULL UNIQUE,
    owner_id     BIGINT NOT NULL REFERENCES people(id),
    backup_id    BIGINT REFERENCES people(id),
    manifest     JSONB NOT NULL,
    created_by   BIGINT NOT NULL REFERENCES people(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
