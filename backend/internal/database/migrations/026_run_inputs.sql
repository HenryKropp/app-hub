-- Execuções visíveis (ADR 033): what was filled in the form of each run, so
-- the run page can show it after the job's payload is cleared. Only the typed
-- values and the names of the files sent: never file contents, and inputs that
-- look like passwords are kept as "masked" without the value. NULL means the
-- run is older than this column.
ALTER TABLE runs ADD COLUMN IF NOT EXISTS inputs JSONB;
