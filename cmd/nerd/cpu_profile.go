package main

import (
	"fmt"
	"os"
	"runtime/pprof"
)

// cpuProfilePath is --cpuprofile: write a pprof CPU profile of the whole
// command to this file (inspect with `go tool pprof`). Empty means none.
var cpuProfilePath string

// cpuProfileFile is the open profile while one is running.
var cpuProfileFile *os.File

// startCPUProfile begins the profile --cpuprofile asked for, if any.
func startCPUProfile() error {
	if cpuProfilePath == "" || cpuProfileFile != nil {
		return nil
	}
	f, err := os.Create(cpuProfilePath)
	if err != nil {
		return fmt.Errorf("--cpuprofile: %w", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		return fmt.Errorf("--cpuprofile: %w", err)
	}
	cpuProfileFile = f
	return nil
}

// stopCPUProfile flushes and closes a running profile.
func stopCPUProfile() {
	if cpuProfileFile == nil {
		return
	}
	pprof.StopCPUProfile()
	_ = cpuProfileFile.Close()
	cpuProfileFile = nil
}
