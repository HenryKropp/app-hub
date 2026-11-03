-- A link version remembers whether its site lets the Ziv show it inside the
-- page (checked once when the link is saved).
ALTER TABLE artifact_versions ADD COLUMN IF NOT EXISTS embeddable BOOLEAN NOT NULL DEFAULT FALSE;
