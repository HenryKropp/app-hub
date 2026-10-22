-- Source bundles make browser-imported scripts independent of the Hub's disk.
CREATE TABLE script_sources (
    app_id BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    files JSONB NOT NULL
);
