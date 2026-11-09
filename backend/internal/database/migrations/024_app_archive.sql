-- Arquivar app (ADR 031): an archived app leaves the catalog, cannot run and
-- is deleted for good 30 days after archived_at. Restoring clears both columns.
ALTER TABLE apps ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE apps ADD COLUMN IF NOT EXISTS archived_by BIGINT REFERENCES people(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS apps_archived_idx ON apps (archived_at) WHERE archived_at IS NOT NULL;
