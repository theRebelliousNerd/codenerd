package main

import (
	"fmt"
	"os"
	"runtime"
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

// memProfilePath is --memprofile: write a pprof heap profile (live objects
// after a final GC, plus allocations) when the command ends. Empty means none.
var memProfilePath string

// writeMemProfile writes the heap profile --memprofile asked for, if any.
func writeMemProfile() error {
	if memProfilePath == "" {
		return nil
	}
	f, err := os.Create(memProfilePath)
	if err != nil {
		return fmt.Errorf("--memprofile: %w", err)
	}
	runtime.GC() // the live heap, not garbage awaiting collection
	if err := pprof.WriteHeapProfile(f); err != nil {
		_ = f.Close()
		return fmt.Errorf("--memprofile: %w", err)
	}
	return f.Close()
}
