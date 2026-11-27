// Package keyswap re-seals everything the vault (HUB_SECRET_KEY) protects
// under the current key, in every company: the secret app settings
// (app_config.value_cipher) and the app database copies on disk
// (shared_backups). After it runs, the old key can leave the server
// (Fase 0 de segurança).
package keyswap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

// Report counts what was re-sealed.
type Report struct {
	Values int // secret settings
	Copies int // app database copies
}

// Rewrap re-seals every value and copy not yet under v's current key. It is
// safe to run again: what is already current is skipped.
func Rewrap(ctx context.Context, pool *pgxpool.Pool, v *vault.Vault) (Report, error) {
	var rep Report
	schemas, err := companySchemas(ctx, pool)
	if err != nil {
		return rep, err
	}
	for _, schema := range schemas {
		n, err := rewrapValues(ctx, pool, v, schema)
		rep.Values += n
		if err != nil {
			return rep, fmt.Errorf("%s: %w", schema, err)
		}
		n, err = rewrapCopies(ctx, pool, v, schema)
		rep.Copies += n
		if err != nil {
			return rep, fmt.Errorf("%s: %w", schema, err)
		}
	}
	return rep, nil
}

func companySchemas(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `SELECT schema_name FROM public.workspace_registry WHERE ready=TRUE ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func rewrapValues(ctx context.Context, pool *pgxpool.Pool, v *vault.Vault, schema string) (int, error) {
	t := pgx.Identifier{schema, "app_config"}.Sanitize()
	type row struct {
		AppID  int64
		Name   string
		Cipher []byte
	}
	rows, err := pool.Query(ctx, `SELECT app_id, name, value_cipher FROM `+t+` WHERE value_cipher IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	all, err := pgx.CollectRows(rows, pgx.RowToStructByPos[row])
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range all {
		if v.Current(r.Cipher) {
			continue
		}
		plain, err := v.Open(r.Cipher)
		if err != nil {
			return n, fmt.Errorf("app %d, %s: %w", r.AppID, r.Name, err)
		}
		sealed, err := v.Seal(plain)
		if err != nil {
			return n, err
		}
		// Only this exact old value is replaced, so a change made meanwhile wins.
		tag, err := pool.Exec(ctx, `UPDATE `+t+` SET value_cipher=$3 WHERE app_id=$1 AND name=$2 AND value_cipher=$4`, r.AppID, r.Name, sealed, r.Cipher)
		if err != nil {
			return n, err
		}
		n += int(tag.RowsAffected())
	}
	return n, nil
}

func rewrapCopies(ctx context.Context, pool *pgxpool.Pool, v *vault.Vault, schema string) (int, error) {
	rows, err := pool.Query(ctx, `SELECT path FROM `+pgx.Identifier{schema, "shared_backups"}.Sanitize())
	if err != nil {
		return 0, err
	}
	paths, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	n := 0
	for _, p := range paths {
		done, err := rewrapFile(v, p)
		if err != nil {
			return n, fmt.Errorf("%s: %w", p, err)
		}
		if done {
			n++
		}
	}
	return n, nil
}

// rewrapFile re-seals one copy through a temporary file, so the copy is
// either the old one or the new one, never half of each. A missing file
// (already expired) or a copy never sealed is left alone; the server seals
// plain copies itself when it starts.
func rewrapFile(v *vault.Vault, path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer f.Close()
	head := make([]byte, 12)
	k, _ := io.ReadFull(f, head)
	if !vault.Sealed(head[:k]) || v.StreamCurrent(head[:k]) {
		return false, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return false, err
	}
	plain, err := v.OpenStream(f)
	if err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".trocarchave-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	w, err := v.SealStream(tmp)
	if err == nil {
		_, err = io.Copy(w, plain)
	}
	if err == nil {
		err = w.Close()
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return false, err
	}
	f.Close()
	return true, os.Rename(tmp.Name(), path)
}
