-- Parar: the Hub asks the agent to close an app it opened.
ALTER TABLE agent_jobs DROP CONSTRAINT IF EXISTS agent_jobs_kind_check;
ALTER TABLE agent_jobs ADD CONSTRAINT agent_jobs_kind_check
    CHECK (kind IN ('run', 'choose_folder', 'send_file', 'update', 'stop'));

ALTER TABLE runs DROP CONSTRAINT IF EXISTS runs_status_check;
ALTER TABLE runs ADD CONSTRAINT runs_status_check
    CHECK (status IN ('waiting', 'running', 'done', 'no_files', 'error', 'interrupted', 'stopped'));

-- 0.3.1+: understands stop jobs.
ALTER TABLE computers ADD COLUMN IF NOT EXISTS can_stop BOOLEAN NOT NULL DEFAULT false;
