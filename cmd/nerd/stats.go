package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func computeStats(ctx context.Context, workspace, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" || strings.EqualFold(target, "none") {
		return "", fmt.Errorf("stats requires a file or directory target")
	}

	full := target
	if !filepath.IsAbs(full) {
		full = filepath.Join(workspace, target)
	}

	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", full)
	}

	if !info.IsDir() {
		lines, err := countFileLines(full)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s: %d lines", target, lines), nil
	}

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

		lines, err := countFileLines(path)
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

func countFileLines(path string) (int64, error) {
	f, err := os.Open(path)
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
