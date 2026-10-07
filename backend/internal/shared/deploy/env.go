package deploy

import (
	"context"
	"strconv"
	"strings"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/manifest"
	"github.com/HenryKropp/app-hub/backend/internal/shared/appdb"
)

// dbPasswordName is the app_config row (sealed) with the app database's
// password. Names starting with "_" are the Hub's own and never reach the app.
const dbPasswordName = "_db_password"

// MissingSettings lists required settings without a value.
type MissingSettings struct{ Names []string }

func (e MissingSettings) Error() string {
	return "Falta preencher: " + strings.Join(e.Names, ", ") + "."
}

// values reads app_config, opening secrets with the vault.
func (s *Service) values(ctx context.Context, appID int64) (map[string]string, error) {
	rows, err := s.q.ListAppConfig(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		if r.IsSecret {
			v, err := s.vault.Open(r.ValueCipher)
			if err != nil {
				return nil, err
			}
			out[r.Name] = v
		} else {
			out[r.Name] = r.ValuePlain
		}
	}
	return out, nil
}

func (s *Service) missingSettings(ctx context.Context, appID int64, m manifest.Manifest) ([]string, error) {
	if m.Shared == nil || len(m.Shared.Env) == 0 {
		return nil, nil
	}
	vals, err := s.values(ctx, appID)
	if err != nil {
		return nil, err
	}
	var miss []string
	for _, e := range m.Shared.Env {
		if e.Required && vals[e.Name] == "" && e.Default == "" {
			label := e.Label
			if label == "" {
				label = e.Name
			}
			miss = append(miss, label)
		}
	}
	return miss, nil
}

// dbPassword returns the app database's password, creating (and sealing) it
// the first time.
func (s *Service) dbPassword(ctx context.Context, appID int64) (string, error) {
	vals, err := s.values(ctx, appID)
	if err != nil {
		return "", err
	}
	if pw := vals[dbPasswordName]; pw != "" {
		return pw, nil
	}
	pw := appdb.NewPassword()
	sealed, err := s.vault.Seal(pw)
	if err != nil {
		return "", err
	}
	return pw, s.q.UpsertAppConfig(ctx, sqlc.UpsertAppConfigParams{AppID: appID, Name: dbPasswordName, IsSecret: true, ValueCipher: sealed})
}

// env is what the container receives: manifest defaults, then the app's
// settings, then the Hub's own variables (which always win).
func (s *Service) env(ctx context.Context, appID int64, m manifest.Manifest, sa sqlc.SharedApp, dbName string, port int, test bool) (map[string]string, error) {
	vals, err := s.values(ctx, appID)
	if err != nil {
		return nil, err
	}
	env := map[string]string{}
	if m.Shared != nil {
		for _, e := range m.Shared.Env {
			if e.Default != "" {
				env[e.Name] = e.Default
			}
		}
	}
	for k, v := range vals {
		if k != "" && k[0] != '_' && v != "" {
			env[k] = v
		}
	}
	env["PORT"] = strconv.Itoa(port)
	env["APPHUB_DATA_DIR"] = "/data"
	env["BASE_URL"] = s.AppURL(sa.Slug)
	if test {
		env["BASE_URL"] = s.TestURL(sa.Slug)
	}
	delete(env, "DATABASE_URL")
	if dbName != "" {
		pw := vals[dbPasswordName]
		if test {
			pw = s.testPassword(dbName) // guardada ao criar o banco de teste
		}
		env["DATABASE_URL"] = s.dbs.URL(dbName, pw)
	}
	return env, nil
}
