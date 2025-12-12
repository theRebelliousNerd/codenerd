package coder

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// FileTransaction provides best-effort atomicity for multi-file edit batches.
// Stage each file before mutation; Commit removes backups; Rollback restores originals.
type FileTransaction struct {
	backups map[string]string      // original path -> temp backup path
	modes   map[string]fs.FileMode // original path -> original mode
	creates map[string]struct{}    // files that did not exist at stage time
}

func NewFileTransaction() *FileTransaction {
	return &FileTransaction{
		backups: make(map[string]string),
		modes:   make(map[string]fs.FileMode),
		creates: make(map[string]struct{}),
	}
}

// Stage snapshots the current state of path before mutation.
// If the file does not exist, it is tracked as a create so rollback can delete it.
func (tx *FileTransaction) Stage(path string) error {
	if _, ok := tx.backups[path]; ok {
		return nil
	}
	if _, ok := tx.creates[path]; ok {
		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			tx.creates[path] = struct{}{}
			return nil
		}
		return err
	}
	tx.modes[path] = info.Mode()

	backup, err := os.CreateTemp("", "codenerd_backup_*")
	if err != nil {
		return err
	}
	defer backup.Close()

	original, err := os.Open(path)
	if err != nil {
		return err
	}
	defer original.Close()

	if _, err := io.Copy(backup, original); err != nil {
		return err
	}
	tx.backups[path] = backup.Name()
	return nil
}

// Commit removes all backups.
func (tx *FileTransaction) Commit() {
	for _, backup := range tx.backups {
		_ = os.Remove(backup)
	}
	tx.backups = nil
	tx.modes = nil
	tx.creates = nil
}

// Rollback restores backups and deletes any created files.
func (tx *FileTransaction) Rollback() {
	for originalPath, backupPath := range tx.backups {
		data, err := os.ReadFile(backupPath)
		if err == nil {
			_ = os.MkdirAll(filepath.Dir(originalPath), 0755)
			_ = os.WriteFile(originalPath, data, 0644)
			if mode, ok := tx.modes[originalPath]; ok {
				_ = os.Chmod(originalPath, mode)
			}
		}
		_ = os.Remove(backupPath)
	}

	for createdPath := range tx.creates {
		_ = os.Remove(createdPath)
	}

	tx.backups = nil
	tx.modes = nil
	tx.creates = nil
}

