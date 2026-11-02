-- 014: empresas criadas pelo cadastro ficaram sem runs.dismissed_at, porque a
-- 006 procurava a coluna em qualquer schema. Repara só o schema desta empresa,
-- com a mesma regra da 006: execuções já existentes não viram faixa.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'runs' AND column_name = 'dismissed_at') THEN
        ALTER TABLE runs ADD COLUMN dismissed_at TIMESTAMPTZ;
        UPDATE runs SET dismissed_at = now();
    END IF;
END $$;
