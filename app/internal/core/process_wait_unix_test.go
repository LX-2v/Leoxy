//go:build unix

package core

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestWaitStopped(t *testing.T) {
	process := exec.Command("sleep", "30")
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.Process.Kill()
		_ = process.Wait()
	})

	if WaitStopped(process.Process.Pid, 200*time.Millisecond) {
		t.Fatal("WaitStopped returned true for a running process")
	}
	if err := process.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := process.Wait(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		status, statusOK := exitErr.Sys().(syscall.WaitStatus)
		if !statusOK || !status.Signaled() || status.Signal() != syscall.SIGKILL {
			t.Fatal(err)
		}
	}
	if !WaitStopped(process.Process.Pid, time.Second) {
		t.Fatal("WaitStopped returned false for a stopped process")
	}
}
