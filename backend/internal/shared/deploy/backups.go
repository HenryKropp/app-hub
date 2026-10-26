package deploy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/HenryKropp/app-hub/backend/internal/database/sqlc"
	"github.com/HenryKropp/app-hub/backend/internal/vault"
)

var keep = map[string]time.Duration{"daily": 14 * 24 * time.Hour, "pre_deploy": 30 * 24 * time.Hour,
	"pre_restore": 30 * 24 * time.Hour, "manual": 30 * 24 * time.Hour}

func (s *Service) BackupPath(b sqlc.SharedBackup) string { return b.Path }

// ErrBackupFailed is what a person sees when a copy fails for a technical
// reason (errors.Is matches it; the technical error stays underneath).
var ErrBackupFailed = errors.New("Não foi possível fazer a cópia agora. Tente de novo em alguns minutos.")

// plainError shows a plain sentence and keeps the technical error underneath
// (errors.Is/As and the logs still see it).
type plainError struct {
	msg string
	err error
}

func (e *plainError) Error() string { return e.msg }
func (e *plainError) Unwrap() error { return e.err }
func (e *plainError) Is(target error) bool {
	return target == ErrBackupFailed && e.msg == ErrBackupFailed.Error()
}

// detail is the technical text behind an error, for logs and "Ver detalhes".
func detail(err error) string {
	var p *plainError
	if errors.As(err, &p) && p.err != nil {
		return p.err.Error()
	}
	return err.Error()
}

// Backup copies the app's database to DataDir/backups/<app>/. A failed copy
// leaves no file and no row. Technical failures are logged and answered with
// a plain sentence.
func (s *Service) Backup(ctx context.Context, appID int64, kind string, depID, by *int64) (sqlc.SharedBackup, error) {
	b, err := s.backup(ctx, appID, kind, depID, by)
	if err != nil && !errors.Is(err, ErrNotShared) && !errors.Is(err, errNoDB) {
		slog.Error("shared backup", "app", appID, "kind", kind, "err", err)
		return b, &plainError{msg: ErrBackupFailed.Error(), err: err}
	}
	return b, err
}

var errNoDB = errors.New("Este app não tem banco para copiar.")

func (s *Service) backup(ctx context.Context, appID int64, kind string, depID, by *int64) (sqlc.SharedBackup, error) {
	ttl, ok := keep[kind]
	if !ok {
		return sqlc.SharedBackup{}, fmt.Errorf("backup kind %q", kind)
	}
	sa, err := s.q.GetSharedApp(ctx, appID)
	if err != nil {
		return sqlc.SharedBackup{}, ErrNotShared
	}
	if sa.DbName == "" {
		return sqlc.SharedBackup{}, errNoDB
	}
	if s.vault == nil {
		return sqlc.SharedBackup{}, vault.ErrNoKey
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
	// The copy is encrypted with the vault key: whoever reads the server's
	// disk does not read the app's data.
	err = s.sealDump(ctx, sa.DbName, tmp)
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
	wasUp := sa.Status == "running" || sa.Status == "starting"
	if _, err := s.restoreFile(ctx, sa, b, by); err != nil {
		s.bringBack(ctx, appID, wasUp) // restoreFile may have stopped it
		return err
	}
	s.event(ctx, appID, &by, "Restaurou a cópia de "+b.CreatedAt.Time.Local().Format("02/01/2006 15:04"))
	if sa.CurrentDeploymentID.Valid && sa.Status != "stopped" {
		if err := s.run(ctx, appID); err != nil {
			return err
		}
	}
	return nil
}

// restoreFile stops the app, copies the current data ("antes de restaurar",
// returned, so nothing written before the stop is lost) and loads the file
// into its database. It does not start the app (the caller brings it back on
// error). If loading fails it puts the current data back.
func (s *Service) restoreFile(ctx context.Context, sa sqlc.SharedApp, b sqlc.SharedBackup, by int64) (sqlc.SharedBackup, error) {
	if _, err := os.Stat(b.Path); err != nil {
		return sqlc.SharedBackup{}, errors.New("O arquivo da cópia não foi encontrado no servidor.")
	}
	if err := s.removeLive(ctx, sa.AppID); err != nil {
		slog.Warn("shared restore: stop app", "app", sa.AppID, "err", err)
		return sqlc.SharedBackup{}, errors.New("Não foi possível desligar o app para restaurar. Nada mudou.")
	}
	before, err := s.Backup(ctx, sa.AppID, "pre_restore", nil, &by)
	if err != nil {
		slog.Warn("shared restore: copy current data", "app", sa.AppID, "err", err)
		return sqlc.SharedBackup{}, errors.New("Não foi possível copiar os dados atuais antes de restaurar. Nada mudou.")
	}
	if err := s.loadFile(ctx, sa.DbName, b.Path); err != nil {
		slog.Warn("shared restore", "app", sa.AppID, "backup", b.ID, "err", err)
		if rerr := s.loadFile(ctx, sa.DbName, before.Path); rerr != nil {
			slog.Error("shared restore: put current data back", "app", sa.AppID, "backup", before.ID, "err", rerr)
			return before, errors.New("Não foi possível restaurar a cópia, e os dados podem ter ficado pela metade. Os dados de antes estão na cópia \"antes de restaurar\" em Cópias; restaure-a.")
		}
		return before, errors.New("Não foi possível restaurar a cópia. Nada mudou nos dados.")
	}
	return before, nil
}

func (s *Service) sealDump(ctx context.Context, db string, w io.Writer) error {
	sw, err := s.vault.SealStream(w)
	if err != nil {
		return err
	}
	if err := s.dbs.Dump(ctx, db, sw); err != nil {
		return err
	}
	return sw.Close()
}

// OpenCopy opens a copy file for reading its plain contents (restore and
// download). Copies made before encryption existed are read as they are.
func (s *Service) OpenCopy(path string) (io.Reader, io.Closer, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	br := bufio.NewReader(f)
	head, _ := br.Peek(16)
	if !vault.Sealed(head) {
		return br, f, nil
	}
	if s.vault == nil {
		f.Close()
		return nil, nil, vault.ErrNoKey
	}
	r, err := s.vault.OpenStream(br)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return r, f, nil
}

// loadFile loads a copy file into a database.
func (s *Service) loadFile(ctx context.Context, db, path string) error {
	r, c, err := s.OpenCopy(path)
	if err != nil {
		return err
	}
	defer c.Close()
	return s.dbs.Restore(ctx, db, r)
}

// SealOldBackups encrypts the copies written before encryption existed, in
// place. A copy that fails stays as it was and is tried again next start.
func (s *Service) SealOldBackups(ctx context.Context) {
	if s.vault == nil {
		return
	}
	all, err := s.q.ListAllBackups(ctx)
	if err != nil {
		slog.Warn("seal old backups: list", "err", err)
		return
	}
	for _, b := range all {
		if err := s.sealOld(ctx, b); err != nil {
			slog.Warn("seal old backups", "backup", b.ID, "err", err)
		}
	}
}

func (s *Service) sealOld(ctx context.Context, b sqlc.SharedBackup) error {
	f, err := os.Open(b.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	head := make([]byte, 16)
	n, _ := io.ReadFull(f, head)
	if vault.Sealed(head[:n]) {
		return nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(b.Path), ".partial-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	sw, err := s.vault.SealStream(tmp)
	if err == nil {
		_, err = io.Copy(sw, f)
	}
	if err == nil {
		err = sw.Close()
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	st, err := os.Stat(tmp.Name())
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), b.Path); err != nil {
		return err
	}
	return s.q.UpdateBackupSize(ctx, sqlc.UpdateBackupSizeParams{ID: b.ID, SizeBytes: st.Size()})
}
