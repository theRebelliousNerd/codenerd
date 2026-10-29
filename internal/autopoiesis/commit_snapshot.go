package autopoiesis

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"codenerd/internal/atomicfile"
	"codenerd/internal/logging"
)

// generatedFileSnapshot keeps a recoverable copy of one committed artifact
// while a replacement is being compiled and registered.
type generatedFileSnapshot struct {
	path       string
	backupPath string
	existed    bool
}

func snapshotGeneratedToolFiles(paths ...string) ([]generatedFileSnapshot, error) {
	snapshots := make([]generatedFileSnapshot, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}

		snapshot := generatedFileSnapshot{path: path}
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			snapshots = append(snapshots, snapshot)
			continue
		}
		if err != nil {
			cleanupGeneratedToolSnapshots(snapshots)
			return nil, fmt.Errorf("inspect %s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			cleanupGeneratedToolSnapshots(snapshots)
			return nil, fmt.Errorf("generated artifact is not a regular file: %s", path)
		}

		source, err := os.Open(path)
		if err != nil {
			cleanupGeneratedToolSnapshots(snapshots)
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		backup, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".rollback-*")
		if err != nil {
			_ = source.Close()
			cleanupGeneratedToolSnapshots(snapshots)
			return nil, fmt.Errorf("create rollback copy for %s: %w", path, err)
		}
		backupPath := backup.Name()
		_, copyErr := io.Copy(backup, source)
		sourceErr := source.Close()
		syncErr := backup.Sync()
		modeErr := backup.Chmod(info.Mode().Perm())
		closeErr := backup.Close()
		if err := errors.Join(copyErr, sourceErr, syncErr, modeErr, closeErr); err != nil {
			_ = os.Remove(backupPath)
			cleanupGeneratedToolSnapshots(snapshots)
			return nil, fmt.Errorf("copy rollback state for %s: %w", path, err)
		}
		snapshot.existed = true
		snapshot.backupPath = backupPath
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, nil
}

func restoreGeneratedToolFiles(snapshots []generatedFileSnapshot) error {
	var failures []error
	for i := len(snapshots) - 1; i >= 0; i-- {
		snapshot := &snapshots[i]
		if snapshot.existed {
			if err := atomicfile.Replace(snapshot.backupPath, snapshot.path); err != nil {
				failures = append(failures, fmt.Errorf("restore %s from %s: %w", snapshot.path, snapshot.backupPath, err))
				continue
			}
			snapshot.backupPath = ""
			continue
		}
		if err := os.Remove(snapshot.path); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("remove uncommitted %s: %w", snapshot.path, err))
		}
	}
	return errors.Join(failures...)
}

func cleanupGeneratedToolSnapshots(snapshots []generatedFileSnapshot) {
	for _, snapshot := range snapshots {
		if snapshot.backupPath == "" {
			continue
		}
		if err := os.Remove(snapshot.backupPath); err != nil && !os.IsNotExist(err) {
			// Keep the primary compile/registration result authoritative. A
			// remaining hidden backup is safer than converting a successful
			// commit into a reported failure after registry publication.
			logging.Get(logging.CategoryAutopoiesis).Warn("Failed to remove generated tool rollback copy %s: %v", snapshot.backupPath, err)
		}
	}
}
