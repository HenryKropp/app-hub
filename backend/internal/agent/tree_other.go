//go:build !windows

package agent

import (
	"os/exec"
	"syscall"
)

// The app gets its own process group, so stopping it reaches what it opened.
func prepareTree(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

type tree struct{ pid int }

func attachTree(cmd *exec.Cmd) tree { return tree{pid: cmd.Process.Pid} }

func (t tree) kill() { _ = syscall.Kill(-t.pid, syscall.SIGKILL) }

func (t tree) close() {}
