CREATE TABLE IF NOT EXISTS computers (
    id            BIGSERIAL PRIMARY KEY,
    person_id     BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    token_hash    TEXT NOT NULL UNIQUE,
    agent_version TEXT NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS computers_person_idx ON computers(person_id);

CREATE TABLE IF NOT EXISTS pairings (
    code          TEXT PRIMARY KEY,
    secret_hash   TEXT NOT NULL UNIQUE,
    computer_name TEXT NOT NULL,
    person_id     BIGINT REFERENCES people(id) ON DELETE CASCADE,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS runs (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id     BIGINT NOT NULL REFERENCES people(id),
    computer_id   BIGINT NOT NULL REFERENCES computers(id),
    output_folder TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'waiting'
                  CHECK (status IN ('waiting', 'running', 'done', 'no_files', 'error', 'interrupted')),
    exit_code     INT,
    error_message TEXT NOT NULL DEFAULT '',
    app_version   TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS runs_app_idx ON runs(app_id, id DESC);

CREATE TABLE IF NOT EXISTS agent_jobs (
    id          BIGSERIAL PRIMARY KEY,
    computer_id BIGINT NOT NULL REFERENCES computers(id) ON DELETE CASCADE,
    run_id      BIGINT REFERENCES runs(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('run', 'choose_folder', 'send_file')),
    payload     JSONB NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'taken', 'done', 'failed')),
    result      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    taken_at    TIMESTAMPTZ,
    done_at     TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS agent_jobs_pending_idx ON agent_jobs(computer_id, id) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS run_files (
    id          BIGSERIAL PRIMARY KEY,
    run_id      BIGINT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    rel_path    TEXT NOT NULL,
    size        BIGINT NOT NULL,
    modified_at TIMESTAMPTZ NOT NULL,
    change      TEXT NOT NULL CHECK (change IN ('new', 'changed')),
    group_index INT NOT NULL,
    group_label TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS run_files_run_idx ON run_files(run_id);
