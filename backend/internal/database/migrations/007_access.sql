-- Cargos personalizáveis (um por pessoa), áreas com líderes e membros,
-- apps restritos e acessos extras (Etapa 2).

CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL PRIMARY KEY,
    key         TEXT UNIQUE,              -- 'admin', 'criador', 'usuario' nos prontos; NULL nos criados
    name        TEXT NOT NULL UNIQUE,
    permissions TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO roles (key, name, permissions) VALUES
    ('admin', 'Administrador', ARRAY['register_apps','edit_any_app','restore_any_app','see_tech_any_app','see_all_runs','see_all_apps','manage_people','manage_areas','manage_roles']),
    ('criador', 'Criador', ARRAY['register_apps']),
    ('usuario', 'Usuário', ARRAY[]::TEXT[])
ON CONFLICT (key) DO NOTHING;

ALTER TABLE people ADD COLUMN IF NOT EXISTS role_id BIGINT REFERENCES roles(id);
UPDATE people p SET role_id = r.id FROM roles r WHERE r.key = p.role AND p.role_id IS NULL;
ALTER TABLE people ALTER COLUMN role_id SET NOT NULL;
ALTER TABLE people DROP COLUMN IF EXISTS role;

CREATE TABLE IF NOT EXISTS areas (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO areas (name) SELECT DISTINCT area FROM apps WHERE area <> '' ON CONFLICT (name) DO NOTHING;
INSERT INTO areas (name) VALUES ('Investimentos'), ('Operações'), ('Relações com Investidores'), ('Research'), ('Compliance'), ('Administrativo')
ON CONFLICT (name) DO NOTHING;
INSERT INTO areas (name) SELECT 'Sem área' WHERE EXISTS (SELECT 1 FROM apps WHERE area = '') ON CONFLICT (name) DO NOTHING;

ALTER TABLE apps ADD COLUMN IF NOT EXISTS area_id BIGINT REFERENCES areas(id);
UPDATE apps a SET area_id = ar.id,
                  area = ar.name,
                  manifest = jsonb_set(a.manifest, '{area}', to_jsonb(ar.name))
FROM areas ar
WHERE a.area_id IS NULL AND ar.name = CASE WHEN a.area = '' THEN 'Sem área' ELSE a.area END;
ALTER TABLE apps ALTER COLUMN area_id SET NOT NULL;
ALTER TABLE apps ADD COLUMN IF NOT EXISTS restricted BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS area_leaders (
    area_id   BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
);

CREATE TABLE IF NOT EXISTS area_members (
    area_id   BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    PRIMARY KEY (area_id, person_id)
);

-- Who sees a restricted app: whole areas and/or single people.
CREATE TABLE IF NOT EXISTS app_viewers (
    id        BIGSERIAL PRIMARY KEY,
    app_id    BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    area_id   BIGINT REFERENCES areas(id) ON DELETE CASCADE,
    person_id BIGINT REFERENCES people(id) ON DELETE CASCADE,
    CHECK ((area_id IS NULL) <> (person_id IS NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS app_viewers_area_idx ON app_viewers(app_id, area_id) WHERE area_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS app_viewers_person_idx ON app_viewers(app_id, person_id) WHERE person_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS app_grants (
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('edit', 'restore', 'tech', 'runs')),
    PRIMARY KEY (app_id, person_id, permission)
);

-- "Todos os apps da área": vale também para apps que entrarem depois.
CREATE TABLE IF NOT EXISTS area_grants (
    area_id    BIGINT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
    person_id  BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('edit', 'restore', 'tech', 'runs')),
    PRIMARY KEY (area_id, person_id, permission)
);
