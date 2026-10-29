package sessionlog

import (
	"fmt"
	"path/filepath"
	"strings"
)

// File and folder names inside a session folder.
const (
	HistoryFile    = "session.json"
	TranscriptFile = "transcript.json"
	LeaseFile      = "lease.json"
	ChatFolder     = "chat"
	JevFolder      = "jev"
	// UnattributedFolder holds calls that carry neither an agent scope nor a
	// broker purpose. Spend that reaches a provider unattributed is a wiring
	// bug and stays visible as a folder rather than vanishing.
	UnattributedFolder = "unattributed"
)

// SessionsDir is where every session folder lives.
func SessionsDir(workspace string) string {
	return filepath.Join(workspace, ".nerd", "sessions")
}

// ValidateID refuses a session id that would escape the sessions directory.
// The id arrives from user input (`/load-session <id>`), so `../` must fail
// closed here.
func ValidateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("session id is required")
	}
	if id == "." || id != filepath.Base(id) || strings.Contains(id, "..") || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("invalid session id %q: must be a plain folder name", id)
	}
	return nil
}

// SessionDir resolves a session's folder.
func SessionDir(workspace, id string) (string, error) {
	if err := ValidateID(id); err != nil {
		return "", err
	}
	return filepath.Join(SessionsDir(workspace), id), nil
}

// HistoryPath resolves a session's session.json.
func HistoryPath(workspace, id string) (string, error) {
	dir, err := SessionDir(workspace, id)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, HistoryFile), nil
}

// SafeSegment makes s usable as one path element: letters, digits, '-', '_'
// and '.' survive, everything else becomes '-'. It never returns "" or a name
// that is only dots.
func SafeSegment(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-.")
	if out == "" {
		return "x"
	}
	if len(out) > 64 {
		out = out[:64]
	}
	return out
}

// AgentFolder names an agent's folder: "<role>-<id>", or "<role>" alone when
// there is no id.
func AgentFolder(role, id string) string {
	role = SafeSegment(role)
	if strings.TrimSpace(id) == "" {
		return role
	}
	id = SafeSegment(id)
	// Shard ids already lead with their type ("coder-1a2b"); repeating the
	// role would name the folder "coder-coder-1a2b".
	if strings.HasPrefix(id, role+"-") || strings.HasPrefix(id, role+"_") {
		return id
	}
	return role + "-" + id
}
