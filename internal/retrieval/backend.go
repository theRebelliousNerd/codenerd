package retrieval

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"codenerd/internal/logging"
)

// =============================================================================
// SCAN BACKENDS
// =============================================================================
//
// parseRipgrepOutput existed, was tested, was documented as the ripgrep path —
// and had no caller. The package name, the config field "number of parallel
// ripgrep processes" and the type comment all described a tool that was never
// executed. That is resolved here by giving the parser a real backend rather
// than deleting it, so the choice between engines is a config decision instead
// of a comment that lies.
//
// Native remains the DEFAULT, deliberately:
//
//   - it needs no external binary, so behavior does not change with the host;
//   - it is the only path that carries the in-process bounds (maxScanFileSize,
//     maxHitsPerFile, maxHitsPerKeyword, the binary sniff) as code rather than
//     as flags a future edit could drop;
//   - it uses the AVX2 scanner in scanner_amd64.go.
//
// Ripgrep is worth selecting on very large trees, where its own walker and
// mmap'd search beat a Go worker pool by a wide margin. The backend mirrors the
// native bounds onto rg flags so the two engines answer the same question.

// ScanBackend performs the literal keyword scan for one keyword.
type ScanBackend interface {
	// Name identifies the backend in logs.
	Name() string

	// Search returns hits for a single literal keyword beneath root, honoring
	// the caller's exclusion globs. Implementations must respect ctx.
	Search(ctx context.Context, root, keyword string, exclude []string) ([]KeywordHit, error)
}

// RipgrepBackend shells out to ripgrep.
type RipgrepBackend struct {
	// Binary is the resolved path to rg.
	Binary string
}

// NewRipgrepBackend resolves ripgrep on PATH.
func NewRipgrepBackend() (*RipgrepBackend, error) {
	path, err := exec.LookPath("rg")
	if err != nil {
		return nil, fmt.Errorf("ripgrep backend unavailable: %w", err)
	}
	return &RipgrepBackend{Binary: path}, nil
}

// AutoBackend returns a ripgrep backend when rg is installed, or nil to mean
// "use the native scan". A nil ScanBackend is the documented native selector, so
// callers can assign the result straight into SparseRetrieverConfig.Backend.
func AutoBackend() ScanBackend {
	rg, err := NewRipgrepBackend()
	if err != nil {
		logging.Context("SparseRetriever: ripgrep not found, using native scan (%v)", err)
		return nil
	}
	logging.Context("SparseRetriever: using ripgrep backend at %s", rg.Binary)
	return rg
}

// Name implements ScanBackend.
func (b *RipgrepBackend) Name() string { return "ripgrep" }

// Search implements ScanBackend.
func (b *RipgrepBackend) Search(ctx context.Context, root, keyword string, exclude []string) ([]KeywordHit, error) {
	if strings.TrimSpace(keyword) == "" {
		return nil, nil
	}
	if root == "" {
		root = "."
	}

	args := []string{
		"--vimgrep",       // path:line:col:content, which parseRipgrepOutput reads
		"--fixed-strings", // the native scanner is a literal byte scan
		"--word-regexp",   // mirrors isWordBoundary
		"--ignore-case",   // mirrors the native case folding
		"--no-messages",
		"--max-filesize", strconv.Itoa(maxScanFileSize),
		"--max-count", strconv.Itoa(maxHitsPerFile),
	}
	for _, pattern := range exclude {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		args = append(args, "--glob", "!"+pattern)
	}
	args = append(args, "--", keyword, root)

	var stdout bytes.Buffer
	cmd := exec.CommandContext(ctx, b.Binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = nil

	err := cmd.Run()
	if ctx.Err() != nil {
		return parseRipgrepOutput(stdout.String(), keyword), fmt.Errorf("ripgrep search for %q: %w", keyword, ctx.Err())
	}
	if err != nil {
		// rg exits 1 when nothing matched. That is an empty result, not a
		// failure, and treating it as one made every miss look like a broken
		// installation.
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("ripgrep search for %q: %w", keyword, err)
		}
		return nil, nil
	}

	hits := parseRipgrepOutput(stdout.String(), keyword)
	if len(hits) > maxHitsPerKeyword {
		hits = hits[:maxHitsPerKeyword]
	}
	return hits, nil
}
