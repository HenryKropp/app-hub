ALTER TABLE computers ADD COLUMN IF NOT EXISTS can_update BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE agent_jobs DROP CONSTRAINT IF EXISTS agent_jobs_kind_check;
ALTER TABLE agent_jobs ADD CONSTRAINT agent_jobs_kind_check
    CHECK (kind IN ('run', 'choose_folder', 'send_file', 'update'));

-- At most one update in flight per computer.
CREATE UNIQUE INDEX IF NOT EXISTS agent_jobs_one_update_idx ON agent_jobs(computer_id)
    WHERE kind = 'update' AND status IN ('pending', 'taken');
