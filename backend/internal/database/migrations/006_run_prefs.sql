-- 006: pasta lembrada por pessoa e app, e faixas de execução dispensáveis.
CREATE TABLE IF NOT EXISTS run_folders (
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    folder     TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (person_id, app_id)
);

DO $$
BEGIN
    -- Só no schema desta empresa: sem o filtro, a coluna da empresa principal
    -- fazia as empresas novas pularem esta etapa (corrigido pela 014).
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'runs' AND column_name = 'dismissed_at') THEN
        ALTER TABLE runs ADD COLUMN dismissed_at TIMESTAMPTZ;
        -- Execuções de antes desta etapa não viram faixa.
        UPDATE runs SET dismissed_at = now();
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS runs_person_idx ON runs(person_id, id DESC);
