-- "Código" connections to Claude now last 90 days (Fase 0 de segurança).
-- Codes made before this version get 90 days from the upgrade.
UPDATE mcp_connections SET access_expires_at = now() + interval '90 days'
WHERE kind = 'code' AND access_expires_at IS NULL AND revoked_at IS NULL;
