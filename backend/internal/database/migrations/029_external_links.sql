-- External links (ADR 045): an HTML artifact opened by someone outside the
-- company, without a Ziv login, at link.<domain>/<company>/<token>.
-- The token is stored as is: whoever reads this database already reads the
-- artifact's HTML itself.
CREATE TABLE IF NOT EXISTS external_links (
    id              BIGSERIAL PRIMARY KEY,
    token           TEXT NOT NULL UNIQUE,
    artifact_id     BIGINT NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    version_id      BIGINT REFERENCES artifact_versions(id) ON DELETE CASCADE, -- NULL = always the latest
    kind            TEXT NOT NULL CHECK (kind IN ('open', 'email')),
    recipient_email TEXT,
    created_by      BIGINT REFERENCES people(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    cancelled_at    TIMESTAMPTZ,
    cancelled_by    BIGINT REFERENCES people(id) ON DELETE SET NULL,
    CHECK ((kind = 'email') = (recipient_email IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS external_links_artifact_idx ON external_links(artifact_id);

CREATE TABLE IF NOT EXISTS external_link_codes (
    id          BIGSERIAL PRIMARY KEY,
    link_id     BIGINT NOT NULL REFERENCES external_links(id) ON DELETE CASCADE,
    code_hash   TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    wrong_tries INT NOT NULL DEFAULT 0,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX IF NOT EXISTS external_link_codes_link_idx ON external_link_codes(link_id, id DESC);

CREATE TABLE IF NOT EXISTS external_link_sessions (
    id          BIGSERIAL PRIMARY KEY,
    link_id     BIGINT NOT NULL REFERENCES external_links(id) ON DELETE CASCADE,
    secret_hash TEXT NOT NULL UNIQUE,
    email       TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL
);

-- One row per load of the content. No IP and no browser (LGPD).
CREATE TABLE IF NOT EXISTS external_link_opens (
    id        BIGSERIAL PRIMARY KEY,
    link_id   BIGINT NOT NULL REFERENCES external_links(id) ON DELETE CASCADE,
    opened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    email     TEXT
);
CREATE INDEX IF NOT EXISTS external_link_opens_link_idx ON external_link_opens(link_id);

UPDATE roles SET permissions = array_append(permissions, 'share_external')
WHERE key = 'admin' AND NOT ('share_external' = ANY(permissions));
