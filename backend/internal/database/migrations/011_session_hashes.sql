-- Preserve existing browser sessions, but stop storing bearer tokens in clear.
-- This migration runs exactly once in each workspace.
UPDATE sessions SET token=encode(sha256(convert_to(token,'UTF8')),'hex');
