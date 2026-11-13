package problemlog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A parked candidate is a self-repair change that ran out of its chain's
// budget green on its own gates, with a slower gate unfinished. The chain
// undoes it as it undoes any change it did not install; the candidate keeps
// what it changed -- each path's content before (as a hash: the tree it
// applies to) and after -- and the evidence: which gates passed, which had not
// finished. Finishing applies the change only to a tree that still matches,
// then runs only the gates that had not finished.

// Gate names a candidate records.
const (
	// GatePackageTests: the change's own package tests passed in the chain.
	GatePackageTests = "package_tests"
	// GateSelfRebuild: the rebuilt binary built, self-checked and installed.
	GateSelfRebuild = "self_rebuild"
)

// ParkedPath is one path the candidate changes.
type ParkedPath struct {
	Path string `json:"path"` // workspace-relative, slash form
	// BaseExisted and BaseHash are what the path held when the chain started
	// (and holds again after the undo): the tree the change applies to.
	BaseExisted bool   `json:"base_existed"`
	BaseHash    string `json:"base_hash,omitempty"`
	// After is what the chain left there; AfterExists false removes it.
	AfterExists bool   `json:"after_exists"`
	After       string `json:"after,omitempty"`
}

// ParkedCandidate is a parked change with its evidence.
type ParkedCandidate struct {
	Job          string       `json:"job"`
	Paths        []ParkedPath `json:"paths"`
	GatesPassed  []string     `json:"gates_passed"`
	GatesPending []string     `json:"gates_pending"`
	ParkedUnix   int64        `json:"parked_unix"`
}

// ParkedDir is where candidates are kept.
func ParkedDir(workspace string) string {
	return filepath.Join(workspace, ".nerd", "self_repair", "parked")
}

// ContentHash is the hash a candidate records for a path's content.
func ContentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// SaveParkedCandidate writes c under ParkedDir and returns its
// workspace-relative path.
func SaveParkedCandidate(workspace string, c ParkedCandidate) (string, error) {
	if c.Job == "" || len(c.Paths) == 0 {
		return "", errors.New("problemlog: a parked candidate needs a job and a path")
	}
	sort.Slice(c.Paths, func(i, j int) bool { return c.Paths[i].Path < c.Paths[j].Path })
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	dir := ParkedDir(workspace)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, c.Job+".json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	rel, err := filepath.Rel(workspace, path)
	if err != nil {
		return path, nil
	}
	return filepath.ToSlash(rel), nil
}

// LoadParkedCandidate reads the candidate at the workspace-relative path.
func LoadParkedCandidate(workspace, rel string) (ParkedCandidate, error) {
	var c ParkedCandidate
	data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(rel)))
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("problemlog: parked candidate %s: %w", rel, err)
	}
	return c, nil
}

// Stale names the paths whose content no longer matches the tree the
// candidate applies to; empty means it still applies.
func (c ParkedCandidate) Stale(workspace string) []string {
	var stale []string
	for _, p := range c.Paths {
		data, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(p.Path)))
		exists := err == nil
		switch {
		case exists != p.BaseExisted:
			stale = append(stale, p.Path)
		case exists && ContentHash(string(data)) != p.BaseHash:
			stale = append(stale, p.Path)
		}
	}
	return stale
}

// Apply writes the candidate's change. It refuses a stale tree.
func (c ParkedCandidate) Apply(workspace string) error {
	if stale := c.Stale(workspace); len(stale) > 0 {
		return fmt.Errorf("problemlog: the tree changed under the parked candidate: %s", strings.Join(stale, ", "))
	}
	for _, p := range c.Paths {
		full := filepath.Join(workspace, filepath.FromSlash(p.Path))
		if !p.AfterExists {
			if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(p.After), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ChangedPaths are the candidate's paths.
func (c ParkedCandidate) ChangedPaths() []string {
	out := make([]string, 0, len(c.Paths))
	for _, p := range c.Paths {
		out = append(out, p.Path)
	}
	return out
}
