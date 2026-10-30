package processutil

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Identity names where and under what environment cmd runs: its working
// directory, absolute and with symlinks resolved, and a digest of the
// environment it gets (cmd.Environ: the inherited one when Env is unset).
// Two commands with equal argv and equal identity are the same invocation;
// equal argv alone is not -- `cargo test` in a nested project selects that
// project's manifest, not the root's.
//
// The digest stays inside the host: it compares environments without
// carrying their values, and it is never published as evidence.
func Identity(cmd *exec.Cmd) (dir, envDigest string) {
	if cmd == nil {
		return "", ""
	}
	dir = cmd.Dir
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		}
	}
	if dir != "" {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			dir = real
		}
		dir = filepath.Clean(dir)
	}
	env := append([]string(nil), cmd.Environ()...)
	sort.Strings(env)
	sum := sha256.Sum256([]byte(strings.Join(env, "\x00")))
	return dir, hex.EncodeToString(sum[:])
}
