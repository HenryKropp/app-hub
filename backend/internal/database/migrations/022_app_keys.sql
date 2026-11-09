-- Cofre do app: the Agente fetches a run's keys once, right after taking the
-- job. The values never go into agent_jobs.payload; this marks the delivery.
ALTER TABLE agent_jobs ADD COLUMN IF NOT EXISTS keys_delivered_at TIMESTAMPTZ;
