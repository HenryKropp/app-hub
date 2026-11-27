-- Where each recorded action came from (Fase 0 de segurança): the visitor's
-- real address and browser name. Older rows keep empty values.
ALTER TABLE security_audit ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT '';
ALTER TABLE security_audit ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '';
