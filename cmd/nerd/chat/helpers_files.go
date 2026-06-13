package chat

import (
	"bufio"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func appendFileContent(workspace, path, content string) error {
	full := resolvePath(workspace, path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(content)
	return err
}

func applyPatchResult(workspace, patch string) string {
	fullPatch := patch
	if !strings.HasPrefix(strings.TrimSpace(patch), "*** Begin Patch") {
		fullPatch = "*** Begin Patch\n" + patch + "\n*** End Patch\n"
	}
	tmpPath := filepath.Join(workspace, ".nerd", "last_patch.txt")
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0755); err == nil {
		if err := os.WriteFile(tmpPath, []byte(fullPatch), 0644); err != nil {
			logging.Routing("[helpers] failed to write patch file: %v", err)
		}
	}
	cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Content -Path $args[0] -Value $args[1]", filepath.Join(workspace, ".nerd", "patch.ps1"), fullPatch)
	if err := cmd.Run(); err != nil {
		logging.Routing("[helpers] failed to run patch command: %v", err)
	}
	if err := runApplyPatch(fullPatch); err != nil {
		return fmt.Sprintf("Patch failed: %v", err)
	}
	return "Patch applied."
}

func runApplyPatch(patch string) error {
	// Try git apply first, fallback to 'patch' if available
	cmd := exec.Command("git", "apply", "--whitespace=nowarn")
	cmd.Stdin = strings.NewReader(patch)
	if err := cmd.Run(); err == nil {
		return nil
	}
	if _, err := exec.LookPath("patch"); err == nil {
		cmd = exec.Command("patch", "-p0")
		cmd.Stdin = strings.NewReader(patch)
		return cmd.Run()
	}
	return fmt.Errorf("git apply and patch both unavailable")
}

func resolvePath(workspace, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(workspace, path)
}

func readFileContent(workspace, path string, maxBytes int) (string, error) {
	full := resolvePath(workspace, path)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if len(data) > maxBytes {
		data = data[:maxBytes]
	}
	return string(data), nil
}

func countFileLines(workspace, path string) (int64, error) {
	full := resolvePath(workspace, path)
	f, err := os.Open(full)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 256*1024)
	var lines int64
	var sawAnyByte bool
	lastByteWasNewline := false

	for {
		chunk, err := reader.ReadBytes('\n')
		if len(chunk) > 0 {
			sawAnyByte = true
			lastByteWasNewline = chunk[len(chunk)-1] == '\n'
			if lastByteWasNewline {
				lines++
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	if !sawAnyByte {
		return 0, nil
	}
	if !lastByteWasNewline {
		lines++
	}
	return lines, nil
}

func (m *Model) handleStatsIntent(ctx context.Context, intent perception.Intent) (string, error) {
	target := strings.TrimSpace(intent.Target)
	if target == "" || strings.EqualFold(target, "none") {
		return "", fmt.Errorf("stats requires a file or directory target")
	}

	full := resolvePath(m.workspace, target)
	info, err := os.Stat(full)
	if err != nil {
		// If the target looks like a path, surface a clear error; otherwise the /stats key isn't wired yet.
		if strings.ContainsAny(target, `/\`) || filepath.Ext(target) != "" {
			return "", fmt.Errorf("file not found: %s", full)
		}
		return "", fmt.Errorf("unsupported stats target %q (try a file path)", target)
	}

	if info.IsDir() {
		// Directory LOC: count lines across code-ish files under the directory.
		allowedExt := map[string]bool{
			".go":     true,
			".mg":     true,
			".mangle": true,
			".dl":     true,
			".py":     true,
			".js":     true,
			".jsx":    true,
			".ts":     true,
			".tsx":    true,
			".rs":     true,
			".java":   true,
			".kt":     true,
			".c":      true,
			".cc":     true,
			".cpp":    true,
			".h":      true,
			".hpp":    true,
			".cs":     true,
			".sh":     true,
			".ps1":    true,
		}

		var totalLines int64
		var countedFiles int64
		var skippedFiles int64
		const maxFileSize = 5 * 1024 * 1024 // 5MB

		walkErr := filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			name := d.Name()
			if d.IsDir() {
				// Skip hidden and dependency/cache directories.
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
					return filepath.SkipDir
				}
				if name == "bin" || name == "build" || name == "tmp" {
					return filepath.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(name))
			if !allowedExt[ext] {
				return nil
			}

			if st, statErr := os.Stat(path); statErr == nil && st.Size() > maxFileSize {
				skippedFiles++
				return nil
			}

			lines, err := countFileLines(m.workspace, path)
			if err != nil {
				skippedFiles++
				return nil
			}
			totalLines += lines
			countedFiles++
			return nil
		})
		if walkErr != nil {
			return "", walkErr
		}

		resp := fmt.Sprintf("%s: %d total lines across %d files", target, totalLines, countedFiles)
		if skippedFiles > 0 {
			resp += fmt.Sprintf(" (%d skipped)", skippedFiles)
		}
		return resp, nil
	}

	lines, err := countFileLines(m.workspace, target)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: %d lines", target, lines), nil
}

func writeFileContent(workspace, path, content string) error {
	full := resolvePath(workspace, path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0644)
}

func makeDir(workspace, path string) error {
	full := resolvePath(workspace, path)
	return os.MkdirAll(full, 0755)
}

func searchInFiles(root, pattern string, maxHits int) ([]string, error) {
	matches := make([]string, 0)
	err := filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= maxHits {
			return filepath.SkipDir
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if strings.Contains(string(data), pattern) {
			matches = append(matches, path)
		}
		return nil
	})
	return matches, err
}
