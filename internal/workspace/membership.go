package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/config"
)

// ErrNotMember is Gate's result for a file that is not a workspace member.
// Directories that are not members return fs.SkipDir instead, so a walk can
// return the error unchanged and the directory is not opened.
var ErrNotMember = errors.New("workspace: path is not a member")

// Membership is the member set of one workspace root.
// For and Open cache one value per canonical root and pattern list.
// The value is safe for concurrent use.
type Membership struct {
	root     string
	patterns patternSet

	refreshMu sync.Mutex
	mu        sync.Mutex

	git        bool
	files      map[string]struct{}
	boundaries map[string]struct{}
	dirs       map[string]struct{}
	decided    map[string]bool
	fileList   []string
	stamps     snapStamps
	gitPaths   gitPaths
	gitPathsOK bool
	refName    string
	refPath    string

	// readDir is os.ReadDir except in tests, which count the directories a
	// walk actually opens. An ignored directory is never passed to it.
	readDir func(string) ([]os.DirEntry, error)
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*Membership{}
)

// For returns the membership of root, with world.ignore_patterns from that
// workspace's .nerd/config.json (or the default list when the file does not
// say). The same canonical root and pattern list share one cached value.
func For(root string) (*Membership, error) {
	root, err := canonicalRoot(root)
	if err != nil {
		return nil, err
	}
	return openCanonical(root, loadIgnorePatterns(root))
}

// Open returns the membership of root using patterns as the extra exclusions.
// A nil slice reads the workspace file, the same as For. An empty non-nil
// slice adds no extra patterns: .gitignore (in a work tree) and the
// always-excluded .git and .nerd directories still apply.
func Open(root string, patterns []string) (*Membership, error) {
	root, err := canonicalRoot(root)
	if err != nil {
		return nil, err
	}
	if patterns == nil {
		return For(root)
	}
	return openCanonical(root, patterns)
}

func openCanonical(root string, patterns []string) (*Membership, error) {
	key := cacheKey(root, patterns)
	cacheMu.Lock()
	if m, ok := cache[key]; ok {
		cacheMu.Unlock()
		return m, nil
	}
	cacheMu.Unlock()

	m, err := newMembership(root, patterns)
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if old, ok := cache[key]; ok {
		return old, nil
	}
	cache[key] = m
	return m, nil
}

func cacheKey(root string, patterns []string) string {
	h := sha256.New()
	for _, p := range patterns {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
	}
	return root + "\x00" + hex.EncodeToString(h.Sum(nil))
}

func canonicalRoot(root string) (string, error) {
	if root == "" || root == "." {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		root = wd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func newMembership(root string, patterns []string) (*Membership, error) {
	m := &Membership{
		root:     root,
		patterns: compilePatterns(patterns),
		decided:  map[string]bool{},
		readDir:  os.ReadDir,
	}
	inside, err := workTree(root)
	if err != nil {
		if !gitMissing(err) {
			return nil, err
		}
		inside = false
	}
	if inside {
		snap, err := loadSnapshot(root)
		if err != nil {
			return nil, err
		}
		m.git = true
		m.storeSnap(snap)
	}
	return m, nil
}

// storeSnap replaces the member set. The caller holds mu, or has not
// published m yet.
func (m *Membership) storeSnap(snap gitSnapshot) {
	m.files = snap.files
	m.boundaries = snap.boundaries
	m.dirs = ancestorDirs(snap.files)
	m.stamps = snap.stamps
	m.gitPaths = snap.paths
	m.gitPathsOK = snap.pathsOK
	m.refName = snap.refName
	m.refPath = snap.refPath
	m.decided = map[string]bool{}
	m.fileList = nil
}

// Root is the canonical workspace root this membership was opened on.
func (m *Membership) Root() string { return m.root }

// Refresh re-reads the git snapshot, or drops the non-git file list so the
// next Files call walks again. Scan ticks call this. Pattern changes are a
// different cache entry; Refresh does not re-read .nerd/config.json.
func (m *Membership) Refresh() error {
	if m == nil {
		return errors.New("workspace: nil membership")
	}
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	if !m.git {
		m.mu.Lock()
		m.fileList = nil
		m.decided = map[string]bool{}
		m.mu.Unlock()
		return nil
	}
	snap, err := loadSnapshot(m.root)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.storeSnap(snap)
	m.mu.Unlock()
	return nil
}

func (m *Membership) ensureFresh() {
	if m == nil || !m.git {
		return
	}
	m.mu.Lock()
	gp, ok := m.gitPaths, m.gitPathsOK
	refName, refPath := m.refName, m.refPath
	m.mu.Unlock()
	if !ok {
		resolved, err := resolveGitPaths(m.root)
		if err != nil {
			return
		}
		m.mu.Lock()
		if !m.gitPathsOK {
			m.gitPaths = resolved
			m.gitPathsOK = true
			gp = resolved
		} else {
			gp = m.gitPaths
		}
		refName, refPath = m.refName, m.refPath
		m.mu.Unlock()
	}
	st, newRef, newPath := stampCached(m.root, gp, refName, refPath)
	if newRef != refName || newPath != refPath {
		m.mu.Lock()
		if m.refName != newRef || m.refPath == "" {
			m.refName = newRef
			m.refPath = newPath
		}
		m.mu.Unlock()
	}
	m.mu.Lock()
	same := st == m.stamps
	m.mu.Unlock()
	if same {
		return
	}
	if err := m.Refresh(); err != nil {
		log.Printf("workspace: refresh %s: %v", m.root, err)
	}
}

// Includes reports whether the workspace-relative file is a member.
// The root itself is a directory, not a file. A gitlink or nested-repo
// boundary is a member (git listed it) but not a file; Files omits it.
func (m *Membership) Includes(rel string) bool {
	if m == nil {
		return false
	}
	m.ensureFresh()
	rel, ok := cleanRel(rel)
	if !ok || rel == "" || alwaysExcluded(rel) {
		return false
	}
	self, under := m.boundaryOf(rel)
	if under {
		return false
	}
	if m.patterns.excluded(rel, false) {
		return false
	}
	if self {
		return true
	}
	if !m.git {
		return true
	}
	m.mu.Lock()
	if _, ok := m.files[rel]; ok {
		m.mu.Unlock()
		return true
	}
	if v, ok := m.decided[decideKey(rel, false)]; ok {
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()
	return m.ask(rel, false)
}

// IncludesDir reports whether a walk should open the workspace-relative
// directory. False means SkipDir: the directory's children are not read.
// The root is always a directory member.
func (m *Membership) IncludesDir(rel string) bool {
	if m == nil {
		return false
	}
	m.ensureFresh()
	rel, ok := cleanRel(rel)
	if !ok {
		return false
	}
	if rel == "" {
		return true
	}
	if alwaysExcluded(rel) {
		return false
	}
	self, under := m.boundaryOf(rel)
	if self || under {
		return false
	}
	if m.patterns.blocksDir(rel) {
		return false
	}
	if !m.git {
		return true
	}
	m.mu.Lock()
	if _, ok := m.dirs[rel]; ok {
		m.mu.Unlock()
		return true
	}
	if v, ok := m.decided[decideKey(rel, true)]; ok {
		m.mu.Unlock()
		return v
	}
	m.mu.Unlock()
	return m.ask(rel, true)
}

// Files is the snapshot's member files, sorted, with forward slashes.
// Gitlinks, nested-repo boundaries, always-excluded paths, and user-pattern
// exclusions are omitted. A file created after the snapshot is in Includes
// and Walk once check-ignore admits it, and in Files only after Refresh.
// Outside git, Files walks the tree and caches the list until Refresh.
func (m *Membership) Files() []string {
	if m == nil {
		return nil
	}
	m.ensureFresh()
	if m.git {
		return m.snapshotFiles()
	}
	m.mu.Lock()
	if m.fileList != nil {
		out := append([]string(nil), m.fileList...)
		m.mu.Unlock()
		return out
	}
	m.mu.Unlock()
	var list []string
	err := m.Walk(context.Background(), func(rel string, d os.DirEntry) error {
		if rel != "" && !d.IsDir() {
			list = append(list, rel)
		}
		return nil
	})
	if err != nil {
		return nil
	}
	sort.Strings(list)
	m.mu.Lock()
	m.fileList = list
	m.mu.Unlock()
	return append([]string(nil), list...)
}

func (m *Membership) snapshotFiles() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fileList != nil {
		return append([]string(nil), m.fileList...)
	}
	out := make([]string, 0, len(m.files))
	for f := range m.files {
		if alwaysExcluded(f) || m.patterns.excluded(f, false) {
			continue
		}
		if _, bound := m.boundaries[f]; bound {
			continue
		}
		out = append(out, f)
	}
	sort.Strings(out)
	m.fileList = out
	return append([]string(nil), out...)
}

// Gate decides one absolute path for a walk.
// A directory that is not a member returns fs.SkipDir. A file that is not a
// member returns ErrNotMember. A path outside the root returns ErrNotMember.
func (m *Membership) Gate(abs string, isDir bool) error {
	if m == nil {
		return ErrNotMember
	}
	rel, ok := m.relOf(abs)
	if !ok {
		return ErrNotMember
	}
	if isDir {
		if !m.IncludesDir(rel) {
			return fs.SkipDir
		}
		return nil
	}
	if !m.Includes(rel) {
		return ErrNotMember
	}
	return nil
}

// Admit is Gate in the shape a walk callback uses.
// SkipDir is returned as the error so the caller can return it unchanged.
// A file that is not a member is process=false and a nil error, so the walk
// continues with the next sibling.
func (m *Membership) Admit(abs string, isDir bool) (bool, error) {
	err := m.Gate(abs, isDir)
	if err == nil {
		return true, nil
	}
	if isDir && errors.Is(err, fs.SkipDir) {
		return false, fs.SkipDir
	}
	if errors.Is(err, ErrNotMember) {
		return false, nil
	}
	return false, err
}

func (m *Membership) relOf(abs string) (string, bool) {
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(m.root, abs)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		if resolved, err := filepath.EvalSymlinks(abs); err == nil && resolved == m.root {
			return "", true
		}
		// Resolve the parent so an aliased root shares membership, without
		// following a member file's symlink into another repository.
		parent, err := filepath.EvalSymlinks(filepath.Dir(abs))
		if err != nil {
			return "", false
		}
		rel, err = filepath.Rel(m.root, filepath.Join(parent, filepath.Base(abs)))
		if err != nil {
			return "", false
		}
		rel = filepath.ToSlash(rel)
	}
	return cleanRel(rel)
}

func (m *Membership) boundaryOf(rel string) (self, under bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for g := range m.boundaries {
		if rel == g {
			self = true
		} else if strings.HasPrefix(rel, g+"/") {
			under = true
		}
	}
	return self, under
}

func (m *Membership) ask(rel string, asDir bool) bool {
	return m.askBatch([]string{rel}, asDir)[rel]
}

// askBatch classifies rels that are not in the snapshot. A git failure marks
// them excluded so an unanswered tree is not opened.
func (m *Membership) askBatch(rels []string, asDir bool) map[string]bool {
	if !m.git || len(rels) == 0 {
		return nil
	}
	// A check-ignore answer belongs to the snapshot it started with.
	// Refresh cannot publish a replacement while that answer is in flight.
	m.refreshMu.Lock()
	defer m.refreshMu.Unlock()
	need := make([]string, 0, len(rels))
	answers := make(map[string]bool, len(rels))
	m.mu.Lock()
	for _, r := range rels {
		if value, ok := m.decided[decideKey(r, asDir)]; ok {
			answers[r] = value
		} else {
			need = append(need, r)
		}
	}
	m.mu.Unlock()
	if len(need) == 0 {
		return answers
	}
	ignored, err := checkIgnore(m.root, need, asDir)
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		log.Printf("workspace: check-ignore %s: %v", m.root, err)
	}
	for _, r := range need {
		value := err == nil && !ignored[r]
		m.decided[decideKey(r, asDir)] = value
		answers[r] = value
	}
	return answers
}

func decideKey(rel string, asDir bool) string {
	if asDir {
		return rel + "\x00d"
	}
	return rel + "\x00f"
}

func ancestorDirs(files map[string]struct{}) map[string]struct{} {
	dirs := make(map[string]struct{})
	for f := range files {
		dir := parentRel(f)
		for dir != "" {
			dirs[dir] = struct{}{}
			dir = parentRel(dir)
		}
	}
	return dirs
}

func parentRel(rel string) string {
	i := strings.LastIndex(rel, "/")
	if i <= 0 {
		return ""
	}
	return rel[:i]
}

func alwaysExcluded(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".git" || seg == ".nerd" {
			return true
		}
	}
	return false
}

// cleanRel returns a slash-separated relative path.
// The root ("", ".", "./") is "" with ok. A path that escapes the root is
// not ok, so IncludesDir does not treat "../x" as the root.
func cleanRel(rel string) (string, bool) {
	rel = filepath.ToSlash(rel)
	if strings.IndexByte(rel, 0) >= 0 {
		return "", false
	}
	if rel == "" || rel == "." {
		return "", true
	}
	if strings.HasPrefix(rel, "/") {
		return "", false
	}
	if vol := filepath.VolumeName(filepath.FromSlash(rel)); vol != "" {
		return "", false
	}
	c := pathClean(rel)
	if c == "" || c == "." {
		return "", true
	}
	if c == ".." || strings.HasPrefix(c, "../") || strings.HasPrefix(c, "/") {
		return "", false
	}
	if vol := filepath.VolumeName(filepath.FromSlash(c)); vol != "" {
		return "", false
	}
	return c, true
}

// loadIgnorePatterns reads only world.ignore_patterns. Invalid JSON, an
// absent key, and an empty array all use the default list. The file body is
// not logged. This is not LoadUserConfig: that call has process-wide effects.
func loadIgnorePatterns(root string) []string {
	defaults := append([]string(nil), config.DefaultWorldConfig().IgnorePatterns...)
	p := filepath.Join(root, ".nerd", "config.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("workspace: ignore_patterns: cannot read %s: %v", p, err)
		}
		return defaults
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		log.Printf("workspace: ignore_patterns: %s is not valid JSON", p)
		return defaults
	}
	raw, ok := top["world"]
	if !ok {
		return defaults
	}
	var world map[string]json.RawMessage
	if err := json.Unmarshal(raw, &world); err != nil {
		log.Printf("workspace: ignore_patterns: %s world is not an object", p)
		return defaults
	}
	ip, ok := world["ignore_patterns"]
	if !ok || string(ip) == "null" {
		return defaults
	}
	var list []string
	if err := json.Unmarshal(ip, &list); err != nil {
		log.Printf("workspace: ignore_patterns: %s world.ignore_patterns is not a string array", p)
		return defaults
	}
	if len(list) == 0 {
		return defaults
	}
	return list
}
