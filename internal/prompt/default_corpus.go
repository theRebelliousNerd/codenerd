package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/core/defaults"
)

// MaterializeDefaultPromptCorpus writes the embedded default prompt corpus DB to dstPath
// if (and only if) dstPath does not already exist.
//
// Returns (true, nil) if the file was written, (false, nil) if no write occurred.
func MaterializeDefaultPromptCorpus(dstPath string) (bool, error) {
	if strings.TrimSpace(dstPath) == "" {
		return false, fmt.Errorf("dstPath is required")
	}

	// Never clobber an existing corpus DB.
	if _, err := os.Stat(dstPath); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("stat dstPath: %w", err)
	}

	if !defaults.PromptCorpusAvailable() {
		return false, nil
	}

	data, err := defaults.PromptCorpusDB.ReadFile("prompt_corpus.db")
	if err != nil {
		return false, fmt.Errorf("read embedded prompt corpus: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return false, fmt.Errorf("mkdir prompts dir: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0644); err != nil {
		return false, fmt.Errorf("write corpus: %w", err)
	}

	return true, nil
}
