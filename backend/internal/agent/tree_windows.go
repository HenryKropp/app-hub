//go:build windows

package agent

import (
	"log/slog"
	"os/exec"

	"golang.org/x/sys/windows"
)

func prepareTree(*exec.Cmd) {}

// tree is a Windows job object holding the app and every process it opens,
// so Parar closes them all. The job does not kill on close: if the Agente
// quits, the apps it opened stay open, as before.
type tree struct {
	job windows.Handle
	cmd *exec.Cmd
}

func attachTree(cmd *exec.Cmd) tree {
	t := tree{cmd: cmd}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		slog.Warn("job object", "err", err)
		return t
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, p)
		windows.CloseHandle(p)
	}
	if err != nil {
		slog.Warn("assign job object", "err", err)
		windows.CloseHandle(job)
		return t
	}
	t.job = job
	return t
}

func (t tree) kill() {
	if t.job != 0 {
		_ = windows.TerminateJobObject(t.job, 1)
	}
	_ = t.cmd.Process.Kill()
}

func (t tree) close() {
	if t.job != 0 {
		windows.CloseHandle(t.job)
	}
}
