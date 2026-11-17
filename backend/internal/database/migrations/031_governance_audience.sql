-- Governança (ADR 046): the audience the leader chose, kept until the last
-- step approves and the request is applied.
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS audience JSONB;
