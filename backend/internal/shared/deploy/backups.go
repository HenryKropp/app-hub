package deploy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
)

var keep = map[string]time.Duration{"daily": 14 * 24 * time.Hour, "pre_deploy": 30 * 24 * time.Hour,
	"pre_restore": 30 * 24 * time.Hour, "manual": 30 * 24 * time.Hour}

func (s *Service) BackupPath(b sqlc.SharedBackup) string { return b.Path }

// Backup copies the app's database to DataDir/backups/<app>/. A failed copy
// leaves no file and no row.
func (s *Service) Backup(ctx context.Context, appID int64, kind string, depID, by *int64) (sqlc.SharedBackup, error) {
	ttl, ok := keep[kind]
	if !ok {
		return sqlc.SharedBackup{}, fmt.Errorf("backup kind %q", kind)
	}
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return sqlc.SharedBackup{}, ErrNotShared
	}
	if sa.DbName == "" {
		return sqlc.SharedBackup{}, errors.New("Este app não tem banco para copiar.")
	}
	dir := filepath.Join(s.opt.DataDir, "backups", strconv.FormatInt(appID, 10))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return sqlc.SharedBackup{}, err
	}
	now := s.now()
	// pg_dump may write part of the copy before failing: write to a temporary
	// file and give it its name only when the copy is complete.
	tmp, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return sqlc.SharedBackup{}, err
	}
	defer os.Remove(tmp.Name())
	err = s.dbs.Dump(ctx, sa.DbName, tmp)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return sqlc.SharedBackup{}, err
	}
	path, err := linkUnique(tmp.Name(), filepath.Join(dir, now.Format("20060102-150405")+"-"+kind), ".dump")
	if err != nil {
		return sqlc.SharedBackup{}, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return sqlc.SharedBackup{}, err
	}
	p := sqlc.InsertBackupParams{AppID: appID, Kind: kind, Path: path, SizeBytes: st.Size(),
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(ttl), Valid: true}, CreatedAt: pgtype.Timestamptz{Time: now, Valid: true}}
	if depID != nil {
		p.DeploymentID = pgtype.Int8{Int64: *depID, Valid: true}
	}
	if by != nil {
		p.CreatedBy = pgtype.Int8{Int64: *by, Valid: true}
	}
	b, err := s.q.InsertBackup(ctx, p)
	if err != nil {
		os.Remove(path)
		return sqlc.SharedBackup{}, err
	}
	return b, nil
}

// linkUnique gives tmp the name base+ext, or base-2+ext, base-3+ext… if taken
// (two copies in the same second). Link never overwrites.
func linkUnique(tmp, base, ext string) (string, error) {
	for i := 1; i < 100; i++ {
		p := base + ext
		if i > 1 {
			p = base + "-" + strconv.Itoa(i) + ext
		}
		err := os.Link(tmp, p)
		if err == nil {
			return p, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	return "", fmt.Errorf("no free name for %s", base)
}

// Restore loads a copy into the app's database (after copying the current
// data) and turns the app back on unless it was stopped.
func (s *Service) Restore(ctx context.Context, appID, backupID, by int64) error {
	unlock, err := s.lock(appID)
	if err != nil {
		return err
	}
	defer unlock()
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return ErrNotShared
	}
	b, err := s.q.GetBackup(ctx, sqlc.GetBackupParams{ID: backupID, AppID: appID})
	if err != nil {
		return errors.New("Cópia não encontrada.")
	}
	if err := s.restoreFile(ctx, sa, b, by); err != nil {
		if _, up := s.Target(appID, false); !up && (sa.Status == "running" || sa.Status == "starting") {
			if rerr := s.Run(ctx, appID); rerr != nil { // restoreFile may have stopped it
				slog.Warn("shared restore: restart", "app", appID, "err", rerr)
			}
		}
		return err
	}
	s.event(ctx, appID, &by, "Restaurou a cópia de "+b.CreatedAt.Time.Local().Format("02/01/2006 15:04"))
	if sa.CurrentDeploymentID.Valid && sa.Status != "stopped" {
		if err := s.Run(ctx, appID); err != nil {
			return err
		}
	}
	return nil
}

// restoreFile copies the current data first ("antes de restaurar"), stops
// the app and loads the file into its database. It does not start the app.
func (s *Service) restoreFile(ctx context.Context, sa sqlc.SharedApp, b sqlc.SharedBackup, by int64) error {
	f, err := os.Open(b.Path)
	if err != nil {
		return errors.New("O arquivo da cópia não foi encontrado no servidor.")
	}
	defer f.Close()
	if _, err := s.Backup(ctx, sa.AppID, "pre_restore", nil, &by); err != nil {
		return errors.New("Não foi possível copiar os dados atuais antes de restaurar. Nada mudou.")
	}
	if err := s.removeLive(ctx, sa.AppID); err != nil {
		return err
	}
	if err := s.dbs.Restore(ctx, sa.DbName, f); err != nil {
		slog.Warn("shared restore", "app", sa.AppID, "backup", b.ID, "err", err)
		return errors.New("Não foi possível restaurar a cópia. Os dados de antes estão guardados na cópia \"antes de restaurar\".")
	}
	return nil
}
