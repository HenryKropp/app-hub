-- Apps compartilhados: rodam uma vez só no servidor do Hub (desenho 2026-10-06).

UPDATE roles SET permissions = array_append(permissions, 'manage_server')
WHERE key = 'admin' AND NOT ('manage_server' = ANY(permissions));

-- Eventos do Hub (o vigia marcou "caiu") não têm pessoa.
ALTER TABLE app_changes ALTER COLUMN person_id DROP NOT NULL;

CREATE TABLE IF NOT EXISTS shared_apps (
    app_id                 BIGINT PRIMARY KEY REFERENCES apps(id) ON DELETE CASCADE,
    slug                   TEXT NOT NULL UNIQUE,
    status                 TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft','starting','running','sleeping','crashed','stopped')),
    always_on              BOOLEAN NOT NULL DEFAULT false,
    idle_minutes           INT NOT NULL DEFAULT 30 CHECK (idle_minutes BETWEEN 5 AND 1440),
    memory_mb              INT NOT NULL DEFAULT 512 CHECK (memory_mb BETWEEN 128 AND 16384),
    cpus                   NUMERIC(4,2) NOT NULL DEFAULT 1 CHECK (cpus > 0 AND cpus <= 16),
    health_path            TEXT NOT NULL DEFAULT '/',
    port                   INT NOT NULL DEFAULT 0,
    wants_db               BOOLEAN NOT NULL DEFAULT false,
    db_name                TEXT NOT NULL DEFAULT '',
    current_deployment_id  BIGINT,
    previous_deployment_id BIGINT,
    restart_count          INT NOT NULL DEFAULT 0,
    restart_window_start   TIMESTAMPTZ,
    last_access_at         TIMESTAMPTZ,
    status_changed_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    status_message         TEXT NOT NULL DEFAULT '',
    acks                   TEXT[] NOT NULL DEFAULT '{}', -- avisos da checagem marcados "estou ciente"
    stopped_by             BIGINT REFERENCES people(id),
    unshared_at            TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS shared_deployments (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version_id    BIGINT REFERENCES app_versions(id),
    kind          TEXT NOT NULL CHECK (kind IN ('test','live')),
    recipe        TEXT NOT NULL DEFAULT '',
    image_id      TEXT NOT NULL DEFAULT '',
    container_id  TEXT NOT NULL DEFAULT '',
    test_db_name  TEXT NOT NULL DEFAULT '',
    port          INT NOT NULL DEFAULT 0, -- porta dentro da caixa desta versão
    status        TEXT NOT NULL DEFAULT 'building'
                  CHECK (status IN ('building','failed','ready','live','retired')),
    build_log     TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    expires_at    TIMESTAMPTZ,
    created_by    BIGINT REFERENCES people(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS shared_deployments_app_idx ON shared_deployments(app_id, id DESC);
-- Uma publicação em andamento por app.
CREATE UNIQUE INDEX IF NOT EXISTS shared_deployments_one_building
    ON shared_deployments(app_id) WHERE status = 'building';

ALTER TABLE shared_apps
    ADD CONSTRAINT shared_apps_current_fk  FOREIGN KEY (current_deployment_id)  REFERENCES shared_deployments(id),
    ADD CONSTRAINT shared_apps_previous_fk FOREIGN KEY (previous_deployment_id) REFERENCES shared_deployments(id);

CREATE TABLE IF NOT EXISTS app_config (
    app_id       BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    is_secret    BOOLEAN NOT NULL DEFAULT false,
    value_plain  TEXT NOT NULL DEFAULT '',
    value_cipher BYTEA,
    updated_by   BIGINT REFERENCES people(id),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (app_id, name)
);

CREATE TABLE IF NOT EXISTS shared_backups (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('daily','pre_deploy','pre_restore','manual')),
    path          TEXT NOT NULL,
    size_bytes    BIGINT NOT NULL DEFAULT 0,
    deployment_id BIGINT REFERENCES shared_deployments(id),
    created_by    BIGINT REFERENCES people(id),
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS shared_backups_app_idx ON shared_backups(app_id, created_at DESC);

CREATE TABLE IF NOT EXISTS shared_usage (
    app_id    BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    at        TIMESTAMPTZ NOT NULL,
    mem_bytes BIGINT NOT NULL,
    cpu_pct   REAL NOT NULL,
    PRIMARY KEY (app_id, at)
);

CREATE TABLE IF NOT EXISTS server_settings (
    id                 INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    monthly_cost_cents BIGINT NOT NULL DEFAULT 0 CHECK (monthly_cost_cents >= 0),
    updated_by         BIGINT REFERENCES people(id),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO server_settings (id) VALUES (1) ON CONFLICT DO NOTHING;
