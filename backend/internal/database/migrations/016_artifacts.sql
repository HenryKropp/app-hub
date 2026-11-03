-- Artifacts: links and single HTML files registered in the portal.
-- No approval; same area/owner/access model as apps.

CREATE TABLE IF NOT EXISTS artifacts (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    summary     TEXT NOT NULL DEFAULT '',
    kind        TEXT NOT NULL CHECK (kind IN ('link', 'file')),
    tags        TEXT[] NOT NULL DEFAULT '{}',
    area_id     BIGINT NOT NULL REFERENCES areas(id),
    owner_id    BIGINT NOT NULL REFERENCES people(id),
    backup_id   BIGINT REFERENCES people(id),
    restricted  BOOLEAN NOT NULL DEFAULT false,
    origin      TEXT NOT NULL DEFAULT 'manual' CHECK (origin IN ('manual', 'mcp')),
    created_by  BIGINT NOT NULL REFERENCES people(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS artifacts_area_idx ON artifacts(area_id);

CREATE TABLE IF NOT EXISTS artifact_versions (
    id            BIGSERIAL PRIMARY KEY,
    artifact_id   BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    number        INT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('link', 'file')),
    url           TEXT,
    blob_sha256   TEXT REFERENCES code_blobs(sha256),
    file_name     TEXT NOT NULL DEFAULT '',
    size          BIGINT NOT NULL DEFAULT 0,
    note          TEXT NOT NULL DEFAULT '',
    restored_from INT,
    author_id     BIGINT REFERENCES people(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (artifact_id, number),
    CHECK ((kind = 'link') = (url IS NOT NULL) AND (kind = 'file') = (blob_sha256 IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS artifact_versions_artifact_idx ON artifact_versions(artifact_id, number DESC);

CREATE TABLE IF NOT EXISTS artifact_viewers (
    id          BIGSERIAL PRIMARY KEY,
    artifact_id BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    area_id     BIGINT REFERENCES areas(id) ON DELETE CASCADE,
    person_id   BIGINT REFERENCES people(id) ON DELETE CASCADE,
    CHECK ((area_id IS NULL) <> (person_id IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS artifact_viewers_area_idx ON artifact_viewers(artifact_id, area_id) WHERE area_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS artifact_viewers_person_idx ON artifact_viewers(artifact_id, person_id) WHERE person_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS artifact_grants (
    artifact_id BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    person_id   BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    permission  TEXT NOT NULL CHECK (permission IN ('edit', 'restore')),
    PRIMARY KEY (artifact_id, person_id, permission)
);

-- New companies start with app approval off (existing rows keep their value).
ALTER TABLE security_settings ALTER COLUMN require_app_approval SET DEFAULT FALSE;
