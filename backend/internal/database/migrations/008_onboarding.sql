CREATE TABLE hub_settings (
    id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    company_name TEXT NOT NULL DEFAULT '' CHECK (char_length(company_name) <= 120)
);
INSERT INTO hub_settings (id) VALUES (TRUE);
