-- 010_security started requiring approval for every app, which blocked the
-- apps a company was already using. Approve, once, the current version of
-- each app that nobody has reviewed yet. Apps a reviewer has already seen
-- (approved, rejected or suspended) keep their decision; new apps and new
-- versions still need approval. Companies created later have no apps here.
INSERT INTO app_security (app_id, approved, reviewed_manifest_hash, reviewed_version, reviewed_at)
SELECT a.id, TRUE,
       encode(sha256(convert_to(a.manifest::text, 'UTF8')), 'hex'),
       COALESCE((SELECT max(number) FROM app_versions WHERE app_id = a.id), 0),
       now()
FROM apps a
ON CONFLICT (app_id) DO UPDATE
SET approved = TRUE,
    reviewed_manifest_hash = EXCLUDED.reviewed_manifest_hash,
    reviewed_version = EXCLUDED.reviewed_version,
    reviewed_at = EXCLUDED.reviewed_at
WHERE app_security.reviewed_at IS NULL AND NOT app_security.approved AND NOT app_security.suspended;
