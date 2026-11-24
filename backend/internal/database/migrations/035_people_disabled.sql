-- Fase 0 de segurança: an administrator can disable a person (no sign-in,
-- sessions and connections end), and a session falls after 7 idle days.
ALTER TABLE people ADD COLUMN IF NOT EXISTS disabled_at TIMESTAMPTZ;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ NOT NULL DEFAULT now();
