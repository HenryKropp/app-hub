package agent

import (
	"context"
	"os/exec"
)

// runTree runs cmd and waits for it. When ctx ends it closes the program and
// everything the program opened (exec.CommandContext would only close the
// program itself).
func runTree(ctx context.Context, cmd *exec.Cmd) error {
	prepareTree(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	t := attachTree(cmd)
	defer t.close()
	exited := make(chan struct{})
	defer close(exited)
	go func() {
		select {
		case <-ctx.Done():
			t.kill()
		case <-exited:
		}
	}()
	return cmd.Wait()
}
