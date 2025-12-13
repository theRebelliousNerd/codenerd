package coder

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// FILE CONTEXT READING
// =============================================================================

// readFileContext reads a file and returns its content with injected facts.
func (c *CoderShard) readFileContext(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	fullPath := c.resolvePath(path)

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // New file
		}
		return "", err
	}

	// Handle directories: list files instead of reading
	if info.IsDir() {
		return c.readDirectoryContext(fullPath)
	}

	// Skip large files (> 100KB)
	if info.Size() > 100*1024 {
		return fmt.Sprintf("// File too large (%d bytes), reading first 100KB\n", info.Size()), nil
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	// If we have VirtualStore, use it and inject facts
	if c.virtualStore != nil {
		actionID := fmt.Sprintf("coder_read_%s", hashContent(fullPath))
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{actionID, "/read_file", fullPath},
		}
		_, _ = c.virtualStore.RouteAction(ctx, action)
	}

	// Inject file topology fact
	if c.kernel != nil {
		hash := hashContent(string(content))
		lang := detectLanguage(path)
		isTest := isTestFile(path)
		_ = c.kernel.Assert(core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{path, hash, "/" + lang, info.ModTime().Unix(), isTest},
		})
	}

	return string(content), nil
}

// readDirectoryContext reads a directory and returns an intelligent summary.
// Extracts package docs, exported types, and function signatures from ALL Go files.
// Uses a context budget to ensure we don't exceed reasonable limits.
func (c *CoderShard) readDirectoryContext(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	const maxContextBytes = 32 * 1024 // 32KB total context budget

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Directory: %s\n", dirPath))
	sb.WriteString(fmt.Sprintf("// Contains %d entries:\n\n", len(entries)))

	// Group by type and prioritize
	var dirs []string
	var mainFiles, regularFiles, testFiles []string
	var otherFiles []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else if strings.HasSuffix(name, ".go") {
			if strings.HasSuffix(name, "_test.go") {
				testFiles = append(testFiles, name)
			} else if name == "main.go" || strings.Contains(name, "cmd") {
				mainFiles = append(mainFiles, name)
			} else {
				regularFiles = append(regularFiles, name)
			}
		} else {
			otherFiles = append(otherFiles, name)
		}
	}

	// List structure
	if len(dirs) > 0 {
		sb.WriteString("// Subdirectories:\n")
		for _, d := range dirs {
			sb.WriteString(fmt.Sprintf("//   %s\n", d))
		}
		sb.WriteString("\n")
	}

	allGoFiles := append(append(mainFiles, regularFiles...), testFiles...)
	if len(allGoFiles) > 0 {
		sb.WriteString(fmt.Sprintf("// Go files (%d total):\n", len(allGoFiles)))
		for _, f := range allGoFiles {
			sb.WriteString(fmt.Sprintf("//   %s\n", f))
		}
		sb.WriteString("\n")
	}

	if len(otherFiles) > 0 {
		sb.WriteString("// Other files:\n")
		for _, f := range otherFiles {
			sb.WriteString(fmt.Sprintf("//   %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Read and extract from ALL Go files (prioritized order)
	// Main/cmd files first, then regular, then tests
	bytesUsed := sb.Len()
	for _, f := range allGoFiles {
		if bytesUsed >= maxContextBytes {
			sb.WriteString(fmt.Sprintf("\n// ... context budget exhausted, remaining files not shown\n"))
			break
		}

		filePath := filepath.Join(dirPath, f)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Extract intelligent summary based on file size
		contentStr := string(content)
		var summary string
		if len(contentStr) <= 4096 {
			// Small file: include full content
			summary = contentStr
		} else {
			// Large file: extract key parts
			summary = c.extractGoFileSummary(contentStr)
		}

		// Check if we have budget
		fileHeader := fmt.Sprintf("\n// === %s ===\n", f)
		if bytesUsed+len(fileHeader)+len(summary) > maxContextBytes {
			// Truncate to fit remaining budget
			remaining := maxContextBytes - bytesUsed - len(fileHeader) - 50
			if remaining > 500 {
				summary = summary[:remaining] + "\n// ... truncated to fit context budget\n"
			} else {
				continue
			}
		}

		sb.WriteString(fileHeader)
		sb.WriteString(summary)
		bytesUsed = sb.Len()
	}

	return sb.String(), nil
}

// extractGoFileSummary extracts the most important parts of a Go file:
// package doc, imports, type definitions, and function signatures.
func (c *CoderShard) extractGoFileSummary(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	inDocComment := false
	inImportBlock := false
	inTypeBlock := false
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Package declaration and doc comments
		if strings.HasPrefix(trimmed, "// Package ") || strings.HasPrefix(trimmed, "/*") {
			inDocComment = true
		}
		if inDocComment {
			sb.WriteString(line + "\n")
			if strings.HasPrefix(trimmed, "package ") || strings.HasSuffix(trimmed, "*/") {
				inDocComment = false
			}
			continue
		}

		// Package line
		if strings.HasPrefix(trimmed, "package ") {
			sb.WriteString(line + "\n\n")
			continue
		}

		// Import block
		if strings.HasPrefix(trimmed, "import ") {
			inImportBlock = true
			sb.WriteString(line + "\n")
			if strings.Contains(line, ")") || !strings.Contains(line, "(") {
				inImportBlock = false
				sb.WriteString("\n")
			}
			continue
		}
		if inImportBlock {
			sb.WriteString(line + "\n")
			if strings.Contains(trimmed, ")") {
				inImportBlock = false
				sb.WriteString("\n")
			}
			continue
		}

		// Type definitions (struct, interface)
		if strings.HasPrefix(trimmed, "type ") {
			inTypeBlock = true
			braceDepth = 0
		}
		if inTypeBlock {
			sb.WriteString(line + "\n")
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 && strings.Contains(line, "}") {
				inTypeBlock = false
				sb.WriteString("\n")
			}
			continue
		}

		// Function/method signatures (exported only)
		if strings.HasPrefix(trimmed, "func ") {
			// Check if exported (starts with uppercase after "func " or "func (receiver) ")
			funcPart := trimmed[5:]
			if strings.HasPrefix(funcPart, "(") {
				// Method - find the function name
				if idx := strings.Index(funcPart, ") "); idx != -1 {
					funcName := funcPart[idx+2:]
					if len(funcName) > 0 && funcName[0] >= 'A' && funcName[0] <= 'Z' {
						// Find end of signature
						sigEnd := strings.Index(line, "{")
						if sigEnd > 0 {
							sb.WriteString(line[:sigEnd] + "{ ... }\n")
						} else if i+1 < len(lines) && strings.Contains(lines[i+1], "{") {
							sb.WriteString(line + " { ... }\n")
						}
					}
				}
			} else if len(funcPart) > 0 && funcPart[0] >= 'A' && funcPart[0] <= 'Z' {
				// Exported function
				sigEnd := strings.Index(line, "{")
				if sigEnd > 0 {
					sb.WriteString(line[:sigEnd] + "{ ... }\n")
				} else if i+1 < len(lines) && strings.Contains(lines[i+1], "{") {
					sb.WriteString(line + " { ... }\n")
				}
			}
		}

		// Const/var blocks (exported only)
		if strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "var ") {
			word := trimmed[strings.Index(trimmed, " ")+1:]
			if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
				sb.WriteString(line + "\n")
			}
		}
	}

	return sb.String()
}
