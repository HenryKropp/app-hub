-- 010_versions_without_secrets.sql: files with passwords (.env, prod.env,
-- .env.local…) no longer enter the history. Take them out of the versions
-- already saved and drop the contents nobody points to any more.
-- Same rule as versions.SecretFile; examples (.env.example…) stay.
UPDATE app_versions v
SET files = COALESCE((
    SELECT jsonb_agg(f ORDER BY n)
    FROM jsonb_array_elements(v.files) WITH ORDINALITY AS e(f, n)
    WHERE NOT (lower(regexp_replace(f->>'path', '^.*/', '')) ~ '^(\.env|.*\.env|\.env\..*)$'
               AND lower(f->>'path') !~ '\.(example|sample|template)$')
), '[]'::jsonb)
WHERE EXISTS (
    SELECT 1 FROM jsonb_array_elements(v.files) f
    WHERE lower(regexp_replace(f->>'path', '^.*/', '')) ~ '^(\.env|.*\.env|\.env\..*)$'
      AND lower(f->>'path') !~ '\.(example|sample|template)$'
);

DELETE FROM code_blobs b
WHERE NOT EXISTS (
    SELECT 1 FROM app_versions v, jsonb_array_elements(v.files) f
    WHERE f->>'sha256' = b.sha256
);
