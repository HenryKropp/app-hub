-- Where the test plan was confirmed: on the site or in the Claude Code chat.
ALTER TABLE app_test_plans ADD COLUMN IF NOT EXISTS confirmed_via TEXT NOT NULL DEFAULT 'site';
