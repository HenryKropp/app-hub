-- Home: each person's starred apps and artifacts, and a log of who opened which
-- artifact (feeds "Usados recentemente" and the team's activity).
CREATE TABLE IF NOT EXISTS favorites (
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('app', 'artifact')),
    item_id    BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (person_id, kind, item_id)
);

CREATE TABLE IF NOT EXISTS artifact_opens (
    id          BIGSERIAL PRIMARY KEY,
    artifact_id BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    person_id   BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    opened_at   TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS artifact_opens_person_idx ON artifact_opens (person_id, opened_at DESC);
CREATE INDEX IF NOT EXISTS artifact_opens_at_idx ON artifact_opens (opened_at);
