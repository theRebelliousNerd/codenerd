package specs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// StorePath is where a workspace keeps its link confirmations, relative to the
// workspace root. It sits beside the machine-checkable spec files and is as
// local as the world cache: .nerd is not committed, so each checkout confirms
// the links it finds on its first scan and judges drift from there.
const StorePath = ".nerd/specs/links.json"

const storeVersion = 1

// Store persists a workspace's Confirmations. One Store serves every holder of
// the same file in the process (OpenStore), so the world scan and the review
// tool never overwrite each other's write.
type Store struct {
	path string
	mu   sync.Mutex
}

var (
	storesMu sync.Mutex
	stores   = map[string]*Store{}
)

// OpenStore returns the process's store for the confirmations file under root.
func OpenStore(root string) *Store {
	path := filepath.Join(root, filepath.FromSlash(StorePath))
	storesMu.Lock()
	defer storesMu.Unlock()
	s, ok := stores[path]
	if !ok {
		s = &Store{path: path}
		stores[path] = s
	}
	return s
}

// Path is the file the store reads and writes.
func (s *Store) Path() string { return s.path }

type storedLink struct {
	Spec          string `json:"spec_ref"`
	Code          string `json:"code_ref"`
	SpecPrint     string `json:"spec_fingerprint"`
	CodePrint     string `json:"code_fingerprint"`
	ConfirmedAt   int64  `json:"confirmed_at"`
	SpecChangedAt int64  `json:"spec_changed_at,omitempty"`
	CodeChangedAt int64  `json:"code_changed_at,omitempty"`
}

type storedFile struct {
	Version int          `json:"version"`
	Links   []storedLink `json:"links"`
}

// Load reads the confirmations. A missing file is no confirmations; a file that
// cannot be read or parsed is an error, so a reviewer's decisions are never
// silently replaced by a fresh measurement.
func (s *Store) Load() (Confirmations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() (Confirmations, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Confirmations{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("spec link confirmations %s: %w", s.path, err)
	}
	var file storedFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("spec link confirmations %s are not valid JSON (fix or delete the file; every link is confirmed again on the next scan): %w", s.path, err)
	}
	if file.Version != storeVersion {
		return nil, fmt.Errorf("spec link confirmations %s have version %d, this build reads version %d", s.path, file.Version, storeVersion)
	}
	out := make(Confirmations, len(file.Links))
	for _, l := range file.Links {
		out[LinkKey{Spec: l.Spec, Code: l.Code}] = Confirmation{
			SpecFingerprint: l.SpecPrint, CodeFingerprint: l.CodePrint, ConfirmedAt: l.ConfirmedAt,
			SpecChangedAt: l.SpecChangedAt, CodeChangedAt: l.CodeChangedAt,
		}
	}
	return out, nil
}

// Update reads the confirmations, lets change produce the next set from them,
// and writes it when it differs. The read and the write are one step under the
// store's lock. change's error leaves the file as it was.
func (s *Store) Update(change func(Confirmations) (Confirmations, error)) (Confirmations, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	next, err := change(prev)
	if err != nil {
		return nil, err
	}
	if next.Equal(prev) {
		return prev, nil
	}
	return next, s.saveLocked(next)
}

func (s *Store) saveLocked(c Confirmations) error {
	file := storedFile{Version: storeVersion, Links: make([]storedLink, 0, len(c))}
	for _, k := range c.Keys() {
		v := c[k]
		file.Links = append(file.Links, storedLink{
			Spec: k.Spec, Code: k.Code, SpecPrint: v.SpecFingerprint, CodePrint: v.CodeFingerprint,
			ConfirmedAt: v.ConfirmedAt, SpecChangedAt: v.SpecChangedAt, CodeChangedAt: v.CodeChangedAt,
		})
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("spec link confirmations: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".links-*.tmp")
	if err != nil {
		return fmt.Errorf("spec link confirmations: %w", err)
	}
	_, werr := tmp.Write(append(data, '\n'))
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("spec link confirmations: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("spec link confirmations: %w", err)
	}
	return nil
}
