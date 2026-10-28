package sessionlog

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionInfo summarises one session folder.
type SessionInfo struct {
	ID     string
	Start  time.Time
	Last   time.Time
	Agents []string // sub-folders: chat, jev, <role>-<id> ...
	Bytes  int64
	Files  int
}

// List returns every session folder, oldest first.
func List(workspace string) ([]SessionInfo, error) {
	root := SessionsDir(workspace)
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionInfo
	for _, e := range entries {
		if !e.IsDir() || ValidateID(e.Name()) != nil {
			continue
		}
		info := SessionInfo{ID: e.Name()}
		dir := filepath.Join(root, e.Name())
		first := time.Time{}
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || path == dir {
				return nil
			}
			fi, err := d.Info()
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if filepath.Dir(path) == dir {
					info.Agents = append(info.Agents, d.Name())
				}
				return nil
			}
			info.Files++
			info.Bytes += fi.Size()
			if first.IsZero() || fi.ModTime().Before(first) {
				first = fi.ModTime()
			}
			if fi.ModTime().After(info.Last) {
				info.Last = fi.ModTime()
			}
			return nil
		})
		info.Start = historyStart(dir)
		if info.Start.IsZero() {
			info.Start = first
		}
		sort.Strings(info.Agents)
		out = append(out, info)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out, nil
}

// historyStart reads created_at from a session's session.json.
func historyStart(dir string) time.Time {
	data, err := os.ReadFile(filepath.Join(dir, HistoryFile))
	if err != nil {
		return time.Time{}
	}
	var h struct {
		CreatedAt time.Time `json:"created_at"`
	}
	if json.Unmarshal(data, &h) != nil {
		return time.Time{}
	}
	return h.CreatedAt
}

// SearchOptions narrows a search.
type SearchOptions struct {
	// Agent keeps only this role: a folder named exactly Agent or "<Agent>-...".
	Agent string
	// Session keeps only this session id.
	Session string
}

// Hit is one matching line.
type Hit struct {
	Session string
	Agent   string // the folder: chat, coder-ab12, jev ...
	File    string // relative to the agent folder, or session.json
	Turn    int
	Line    string
}

// Search finds every line of every session file that contains text
// (case-insensitive). Long text is stored as arrays of lines, so a hit is a
// real line of what an agent said, was told or ran.
func Search(workspace, text string, opt SearchOptions) ([]Hit, error) {
	needle := strings.ToLower(text)
	if needle == "" {
		return nil, nil
	}
	sessions, err := List(workspace)
	if err != nil {
		return nil, err
	}
	var hits []Hit
	for _, s := range sessions {
		if opt.Session != "" && s.ID != opt.Session {
			continue
		}
		root := filepath.Join(SessionsDir(workspace), s.ID)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".json") || d.Name() == LeaseFile {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			agent, file := ChatFolder, rel
			if dir := filepath.Dir(rel); dir != "." {
				agent, file = dir, d.Name()
			}
			if !agentMatches(agent, opt.Agent) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			tree, err := DecodeOrdered(data)
			if err != nil {
				return nil
			}
			w := &searchWalk{needle: needle, session: s.ID, agent: agent, file: file, history: rel == HistoryFile}
			w.walk(tree, exchangeTurn(tree))
			hits = append(hits, w.hits...)
			return nil
		})
	}
	return hits, nil
}

func agentMatches(folder, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return true
	}
	f := strings.ToLower(folder)
	return f == want || strings.HasPrefix(f, want+"-")
}

type searchWalk struct {
	needle  string
	session string
	agent   string
	file    string
	history bool
	users   int
	hits    []Hit
}

func (w *searchWalk) walk(v any, turn int) {
	switch t := v.(type) {
	case string:
		for _, line := range strings.Split(t, "\n") {
			if strings.Contains(strings.ToLower(line), w.needle) {
				w.hits = append(w.hits, Hit{Session: w.session, Agent: w.agent, File: w.file, Turn: turn, Line: line})
			}
		}
	case []any:
		for _, e := range t {
			w.walk(e, turn)
		}
	case *Object:
		if n, ok := t.Values["turn"].(json.Number); ok {
			if i, err := n.Int64(); err == nil {
				turn = int(i)
			}
		}
		if w.history {
			if role, _ := t.Values["role"].(string); role == "user" {
				w.users++
			}
			if _, ok := t.Values["role"]; ok {
				turn = w.users
			}
		}
		for _, k := range t.Keys {
			if k == "text_encoding" || k == "schema" {
				continue
			}
			w.walk(t.Values[k], turn)
		}
	}
}

// exchangeTurn is the turn an exchange file belongs to: its correlation block
// names it, and the lines of the request and response beneath it inherit it.
func exchangeTurn(tree any) int {
	root, ok := tree.(*Object)
	if !ok {
		return 0
	}
	corr, ok := root.Values["correlation"].(*Object)
	if !ok {
		return 0
	}
	if n, ok := corr.Values["turn"].(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
	}
	return 0
}
