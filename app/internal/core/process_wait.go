package core

import "time"

func WaitStopped(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !ProcessRunning(pid) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return !ProcessRunning(pid)
}
