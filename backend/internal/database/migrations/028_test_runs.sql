-- Etapa 2 (ADR 044): a draft is tested on a PC by the Agente before it goes
-- live. The draft walks stages stored here so a restart picks up where it was.
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS stage TEXT NOT NULL DEFAULT '';
ALTER TABLE app_drafts DROP CONSTRAINT IF EXISTS app_drafts_stage_check;
ALTER TABLE app_drafts ADD CONSTRAINT app_drafts_stage_check
    CHECK (stage IN ('', 'esperando_confirmacao', 'esperando_pc', 'rodando', 'publicando'));
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS stage_since TIMESTAMPTZ;
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_job_id BIGINT;
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_started_at TIMESTAMPTZ;
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_exit_code INT;
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_log TEXT NOT NULL DEFAULT '';
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_checks JSONB;
ALTER TABLE app_drafts ADD COLUMN IF NOT EXISTS test_files JSONB;

-- The confirmed test plan of each app: one person confirms once; a new
-- confirmation is asked only when the fingerprint changes.
CREATE TABLE IF NOT EXISTS app_test_plans (
    app_id       BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    fingerprint  TEXT NOT NULL,
    computer_id  BIGINT REFERENCES computers(id) ON DELETE SET NULL,
    confirmed_by BIGINT REFERENCES people(id),
    confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Files the test produced, kept 7 days; seen only by the app owner and admins.
CREATE TABLE IF NOT EXISTS draft_evidence (
    id         BIGSERIAL PRIMARY KEY,
    draft_id   BIGINT NOT NULL REFERENCES app_drafts(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    size       BIGINT NOT NULL,
    content    BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (draft_id, path)
);

-- "<Nome> versão N no ar — testada: ..." — the future bell reads this table.
CREATE TABLE IF NOT EXISTS notices (
    id         BIGSERIAL PRIMARY KEY,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version    INT NOT NULL,
    text       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS notices_app_created ON notices (app_id, created_at DESC);

-- A new app sent by enviar_versao waits hidden and idle until its first test passes.
ALTER TABLE apps ADD COLUMN IF NOT EXISTS in_test BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_jobs ADD COLUMN IF NOT EXISTS draft_id BIGINT REFERENCES app_drafts(id) ON DELETE CASCADE;
ALTER TABLE agent_jobs DROP CONSTRAINT IF EXISTS agent_jobs_kind_check;
ALTER TABLE agent_jobs ADD CONSTRAINT agent_jobs_kind_check
    CHECK (kind IN ('run', 'choose_folder', 'send_file', 'update', 'stop', 'test'));
