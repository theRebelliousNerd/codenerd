package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/tools"
)

// fileSearchOutcome describes acquisition, not semantic references or file-body
// delivery. FailedFiles counts observed acquisition failures; FirstFailure is
// explicitly one sample, rather than an unbounded collection of diagnostics.
// No retained search handle or automatic continuation is created.
type fileSearchOutcome struct {
	Query          string
	Root           string
	Limit          int
	Paths          []string
	WalkComplete   bool
	AtLimit        bool
	FailedFiles    int
	FirstFailure   string
	TerminalError  string
	FilesAttempted int
	BytesRead      int64
}

func (s fileSearchOutcome) complete() bool {
	return s.WalkComplete && s.FailedFiles == 0 && s.TerminalError == ""
}

// searchInFiles preserves literal, case-sensitive matching and the shared
// membership gate. Reaching the count alone does not prove truncation: stop at
// the next admitted file, which leaves its contents and further matches unknown.
func searchInFiles(ctx context.Context, root, pattern string, maxHits int) fileSearchOutcome {
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	out := fileSearchOutcome{Query: pattern, Root: root, Limit: maxHits}
	if err := ctx.Err(); err != nil {
		out.TerminalError = err.Error()
		return out
	}
	if maxHits < 1 {
		out.TerminalError = "matching-file limit must be positive"
		return out
	}
	mem, err := openMembership(root)
	if err != nil {
		out.TerminalError = err.Error()
		return out
	}
	out.Root = mem.Root()
	needle := newLiteralNeedle(pattern)
	err = filepath.Walk(root, func(path string, info fs.FileInfo, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return walkErr
		}
		member, err := mem.Admit(path, info.IsDir())
		if err != nil {
			return err
		}
		if !member || info.IsDir() {
			return nil
		}
		if len(out.Paths) == maxHits {
			out.AtLimit = true
			return filepath.SkipAll
		}
		out.FilesAttempted++
		found, n, err := searchMember(ctx, mem.Root(), path, needle)
		out.BytesRead += n
		if found {
			out.Paths = append(out.Paths, path)
		}
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			out.FailedFiles++
			if out.FirstFailure == "" {
				out.FirstFailure = fmt.Sprintf("%s: %v", path, err)
			}
		}
		return nil
	})
	if err != nil {
		out.TerminalError = err.Error()
	}
	out.WalkComplete = err == nil && !out.AtLimit
	return out
}

func searchMember(ctx context.Context, root, path string, needle literalNeedle) (bool, int64, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, 0, err
	}
	full, err := tools.ResolveWorkspacePath(ctx, root, abs)
	if err != nil {
		return false, 0, err
	}
	// Refuse devices, directories and pipes before open, including symlink
	// targets. A pipe must not block acquisition waiting for a writer.
	info, err := os.Stat(full)
	if err != nil {
		return false, 0, err
	}
	if !info.Mode().IsRegular() {
		return false, 0, fmt.Errorf("not a regular file")
	}
	f, err := os.Open(full)
	if err != nil {
		return false, 0, err
	}
	defer f.Close()
	return needle.contains(ctx, f)
}

// KMP carries a literal match across read boundaries without retaining a whole
// file or a whole line. This buffer is an IO implementation detail, not a scan
// budget. There is no total acquisition-byte limit.
type literalNeedle struct {
	pattern string
	failure []int
}

func newLiteralNeedle(pattern string) literalNeedle {
	n := literalNeedle{pattern: pattern, failure: make([]int, len(pattern))}
	for i, j := 1, 0; i < len(pattern); i++ {
		for j > 0 && pattern[i] != pattern[j] {
			j = n.failure[j-1]
		}
		if pattern[i] == pattern[j] {
			j++
		}
		n.failure[i] = j
	}
	return n
}

func (n literalNeedle) contains(ctx context.Context, r io.Reader) (bool, int64, error) {
	var buf [32 * 1024]byte
	var total int64
	j := 0
	for {
		if err := ctx.Err(); err != nil {
			return false, total, err
		}
		count, err := r.Read(buf[:])
		total += int64(count)
		if err := ctx.Err(); err != nil {
			return false, total, err
		}
		if n.pattern == "" {
			return true, total, nil
		}
		for _, b := range buf[:count] {
			for j > 0 && b != n.pattern[j] {
				j = n.failure[j-1]
			}
			if b == n.pattern[j] {
				j++
			}
			if j == len(n.pattern) {
				return true, total, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return false, total, nil
		}
		if err != nil {
			return false, total, err
		}
	}
}

// renderSearchEvidence is shared by slash history and the answering request.
// Query and paths are quoted data; file contents are never represented here.
func renderSearchEvidence(s fileSearchOutcome) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### Literal search %q\nWorkspace: %q; domain: admitted workspace members under configured ignore patterns and containment checks.\n", s.Query, s.Root)
	fmt.Fprintf(&b, "Found %d matching files (limit %d). File paths only; file bodies are not supplied.\n", len(s.Paths), s.Limit)
	if s.complete() {
		b.WriteString("Coverage: COMPLETE within the searched domain at acquisition time.\n")
		if len(s.Paths) == 0 {
			b.WriteString("No matching files found in that domain.\n")
		}
	} else {
		b.WriteString("Coverage: PARTIAL; absence of further matches is not established.\n")
		if s.AtLimit {
			b.WriteString("At the result limit; additional admitted files were not searched. Further match count is unknown.\n")
		}
		if s.FailedFiles > 0 {
			fmt.Fprintf(&b, "Failed to acquire %d member files; first failure (sample): %q.\n", s.FailedFiles, s.FirstFailure)
		}
		if s.TerminalError != "" {
			fmt.Fprintf(&b, "Search stopped: %q.\n", s.TerminalError)
		}
		b.WriteString("Search remains unresolved; no continuation handle was created.\n")
	}
	fmt.Fprintf(&b, "Acquisition: %d files attempted; %d bytes read.\n", s.FilesAttempted, s.BytesRead)
	for _, path := range s.Paths {
		fmt.Fprintf(&b, "- %q\n", path)
	}
	return b.String()
}
