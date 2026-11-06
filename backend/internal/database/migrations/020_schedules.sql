-- Agendamentos (ADR 024): a person's app runs by itself on a chosen
-- computer at chosen times.
CREATE TABLE IF NOT EXISTS schedules (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id     BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    computer_id   BIGINT NOT NULL REFERENCES computers(id) ON DELETE CASCADE,
    name          TEXT NOT NULL DEFAULT '',
    frequency     JSONB NOT NULL,
    inputs        JSONB NOT NULL DEFAULT '{}',
    files         JSONB NOT NULL DEFAULT '{}',
    output_folder TEXT NOT NULL,
    active        BOOLEAN NOT NULL DEFAULT true,
    paused_reason TEXT NOT NULL DEFAULT '',
    situation     TEXT NOT NULL DEFAULT '',
    failures      INT NOT NULL DEFAULT 0,
    email         JSONB NOT NULL DEFAULT '{}',
    next_run_at   TIMESTAMPTZ NOT NULL,
    last_run_at   TIMESTAMPTZ,
    last_run_id   BIGINT REFERENCES runs(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS schedules_due_idx ON schedules (active, next_run_at);
CREATE INDEX IF NOT EXISTS schedules_app_idx ON schedules (app_id);

ALTER TABLE runs ADD COLUMN IF NOT EXISTS schedule_id BIGINT REFERENCES schedules(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS runs_schedule_idx ON runs (schedule_id, id DESC);

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_status_check;
ALTER TABLE runs ADD CONSTRAINT runs_status_check
    CHECK (status IN ('waiting', 'running', 'done', 'no_files', 'error', 'interrupted', 'stopped', 'missed'));

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_origin_check;
ALTER TABLE runs ADD CONSTRAINT runs_origin_check CHECK (origin IN ('manual', 'mcp', 'schedule'));
