package deliberation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"codenerd/internal/sessionlog"
)

func (s *Service) path(id string) (string, error) {
	if err := sessionlog.ValidateID(id); err != nil {
		return "", err
	}
	dir, err := sessionlog.SessionDir(s.Workspace, s.Session)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "deliberation", id+".json"), nil
}

// Save publishes a complete snapshot atomically, independently of debug logging.
func (s *Service) Save(t *Thread) error {
	path, err := s.path(t.ID)
	if err != nil {
		return err
	}
	if t.Session != s.Session {
		return fmt.Errorf("deliberation belongs to another session")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".thread-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

func (s *Service) Load(id string) (*Thread, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Thread
	if err = json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	if t.ID != id || t.Session != s.Session {
		return nil, fmt.Errorf("deliberation identity mismatch")
	}
	return &t, nil
}

// Pending restores the single values question for a loaded session. Completed
// threads are history and cannot hijack the next request.
func (s *Service) Pending() (*Thread, error) {
	dir, err := sessionlog.SessionDir(s.Workspace, s.Session)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(filepath.Join(dir, "deliberation"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var pending *Thread
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		t, err := s.Load(e.Name()[:len(e.Name())-len(".json")])
		if err != nil {
			return nil, err
		}
		if t.Status == "awaiting_values" {
			if pending != nil {
				return nil, fmt.Errorf("multiple pending deliberations in session")
			}
			pending = t
		}
	}
	return pending, nil
}
