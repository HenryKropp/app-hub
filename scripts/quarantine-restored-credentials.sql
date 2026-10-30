-- Mandatory after a database restore and BEFORE restarting the portal.
-- Deliberately wipes ALL restored credential envelopes, including credentials
-- that appeared active in an old snapshot. Recreate tokens after upstream
-- revocation/rotation; never restore a historical authorization as active.
BEGIN;
DO $$
DECLARE tenant RECORD;
BEGIN
    FOR tenant IN SELECT schema_name FROM public.workspace_registry WHERE ready = TRUE LOOP
        EXECUTE format(
            'WITH quarantined AS (
                UPDATE %I.credential_vault SET revoked=TRUE,envelope=NULL,updated_at=now()
                RETURNING id,revision
             ) INSERT INTO %I.credential_audit(credential_id,actor_id,revision,action)
               SELECT id,0,revision,''revoked'' FROM quarantined',
            tenant.schema_name, tenant.schema_name
        );
    END LOOP;
END $$;
COMMIT;
