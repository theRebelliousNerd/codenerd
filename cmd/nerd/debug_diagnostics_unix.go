//go:build unix

package main

import (
	"os"
	"os/signal"
	"syscall"

	"codenerd/internal/logging"
)

// installGoroutineDumpSignal makes SIGUSR1 write every goroutine's stack to
// the logs directory without stopping the process: `kill -USR1 <pid>` on a
// stalled nerd. SIGQUIT would kill it, and a TUI's raw terminal loses most of
// that dump.
func installGoroutineDumpSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			path, err := logging.WriteGoroutineDump("SIGUSR1")
			if err != nil {
				logging.Get(logging.CategoryBoot).Warn("SIGUSR1 goroutine dump failed: %v", err)
				continue
			}
			logging.Boot("SIGUSR1: goroutines written to %s", path)
		}
	}()
}
