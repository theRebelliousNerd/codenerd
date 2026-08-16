package browser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	browsersecurity "codenerd/internal/browser/security"
)

// Repository tracing is bounded on every axis. A repository is
// attacker-influenced (it can contain a generated file of any size) and this
// runs inside an audit the agent is waiting on. Capping how many files are
// opened, how much of each file is read, how many matches are returned, and
// how deep the walk may descend keeps a single scan from reading arbitrary
// disk, stalling the agent on a large file, or flooding the kernel with facts.
const (
	// maxRepoTraceFiles caps files opened per scan so a generated repo with
	// thousands of files cannot exhaust descriptors or stall the audit.
	maxRepoTraceFiles = 2000 // files opened per scan
	// maxRepoTraceFileBytes caps bytes read per file so a single generated
	// file of arbitrary size cannot consume unbounded memory or time.
	maxRepoTraceFileBytes = 512 << 10 // bytes read per file
	// maxRepoTraceMatches caps matches returned so a pathological needle
	// cannot flood the kernel with derived facts.
	maxRepoTraceMatches = 100 // matches returned
	// maxRepoTraceDepth caps directory depth so a deeply nested attacker
	// tree cannot cause excessive recursion or path-length blowup.
	maxRepoTraceDepth = 12 // directory depth below root
	// maxRepoSnippetBytes caps bytes of context per match so results remain
	// likely locations rather than unbounded source dumps.
	maxRepoSnippetBytes = 200 // bytes of context per match
)

// RepoTraceLimits bounds one scan. A zero field means "use the default"
// rather than "unlimited" - an unbounded scan must not be reachable by
// omitting a value.
type RepoTraceLimits struct {
	MaxFiles     int
	MaxFileBytes int
	MaxMatches   int
	MaxDepth     int
}

// RepoMatch is a likely location, not a source dump. Path is relative to
// the scan root so results never disclose where the repository lives.
type RepoMatch struct {
	Path    string
	Line    int
	Needle  string
	Snippet string
}

// RepoTraceResult holds the bounded output of a scan.
type RepoTraceResult struct {
	Matches      []RepoMatch
	Notes        []string
	Truncated    bool
	FilesScanned int
}

// TraceRepository searches root for the given needles and returns likely
// locations. It never mutates anything and never returns an error for a
// per-file problem: an unreadable file becomes a Note so one bad file
// cannot lose the findings from the rest of the tree.
func TraceRepository(ctx context.Context, root string, needles []string, limits RepoTraceLimits) (RepoTraceResult, error) {
	initRes := RepoTraceResult{Matches: []RepoMatch{}, Notes: []string{}}
	effective, clampNotes := normalizeRepoTraceLimits(limits)
	initRes.Notes = append(initRes.Notes, clampNotes...)
	cleanRoot, effNeedles, err := validateTraceInputs(root, needles)
	if err != nil {
		return RepoTraceResult{Matches: []RepoMatch{}}, err
	}
	if ctx != nil && ctx.Err() != nil {
		initRes.Notes = append(initRes.Notes, fmt.Sprintf("context cancelled: %v", ctx.Err()))
		initRes.Truncated = true
		return initRes, nil
	}
	return runTraceScan(ctx, cleanRoot, effNeedles, effective, initRes)
}

func makeLowerNeedles(needles []string) []string {
	lower := make([]string, len(needles))
	for i, n := range needles {
		lower[i] = strings.ToLower(n)
	}
	return lower
}

func validateTraceInputs(root string, needles []string) (string, []string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", nil, fmt.Errorf("root must not be empty")
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", nil, fmt.Errorf("root %q: %w", root, err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("root %q is not a directory", root)
	}
	var eff []string
	for _, n := range needles {
		if strings.TrimSpace(n) == "" {
			continue
		}
		eff = append(eff, n)
	}
	if len(eff) == 0 {
		return "", nil, fmt.Errorf("at least one needle is required")
	}
	return root, eff, nil
}

func normalizeRepoTraceLimits(in RepoTraceLimits) (RepoTraceLimits, []string) {
	var notes []string
	out := in
	out.MaxFiles, notes = clampLimit(in.MaxFiles, maxRepoTraceFiles, "MaxFiles", notes)
	out.MaxFileBytes, notes = clampLimit(in.MaxFileBytes, maxRepoTraceFileBytes, "MaxFileBytes", notes)
	out.MaxMatches, notes = clampLimit(in.MaxMatches, maxRepoTraceMatches, "MaxMatches", notes)
	out.MaxDepth, notes = clampLimit(in.MaxDepth, maxRepoTraceDepth, "MaxDepth", notes)
	return out, notes
}

func clampLimit(value, ceiling int, name string, notes []string) (int, []string) {
	if value <= 0 {
		return ceiling, notes
	}
	if value > ceiling {
		notes = append(notes, fmt.Sprintf("%s %d exceeds limit %d: clamped to %d", name, value, ceiling, ceiling))
		return ceiling, notes
	}
	return value, notes
}

func isSkippedRepoDir(base string) bool {
	switch base {
	case ".git", "node_modules", "vendor", ".nerd", "dist", "build", ".cache":
		return true
	default:
		return false
	}
}

func repoDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return 0
	}
	parts := strings.Split(rel, string(filepath.Separator))
	count := 0
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == "." {
			continue
		}
		count++
	}
	return count
}

func snippetWindow(lineLen, matchIdx, needleLen int) (int, int) {
	if lineLen <= maxRepoSnippetBytes {
		return 0, lineLen
	}
	center := matchIdx + needleLen/2
	half := maxRepoSnippetBytes / 2
	start := center - half
	if start < 0 {
		start = 0
	}
	end := start + maxRepoSnippetBytes
	if end > lineLen {
		end = lineLen
		start = end - maxRepoSnippetBytes
	}
	if start < 0 {
		start = 0
	}
	return start, end
}

func ensureValidSnippet(s string) string {
	for len(s) > 0 {
		if utf8.ValidString(s) {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func fixedSnippet(line string, start, end int) string {
	for start < end {
		if start <= 0 {
			break
		}
		if start >= len(line) {
			break
		}
		b := line[start]
		if (b & 0xC0) != 0x80 {
			break
		}
		start++
	}
	s := line[start:end]
	for len(s) > 0 {
		if utf8.ValidString(s) {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func boundedSnippet(line string, matchIdx, needleLen int) string {
	if len(line) <= maxRepoSnippetBytes {
		return ensureValidSnippet(line)
	}
	start, end := snippetWindow(len(line), matchIdx, needleLen)
	return fixedSnippet(line, start, end)
}

type traceState struct {
	root                string
	needles             []string
	lowerNeedles        []string
	effective           RepoTraceLimits
	redactor            *browsersecurity.Redactor
	result              RepoTraceResult
	matches             []RepoMatch
	depthNoteAdded    bool
	fileLimitNoteAdded  bool
	matchLimitNoteAdded bool
	ctx                 context.Context
}

func runTraceScan(ctx context.Context, root string, needles []string, eff RepoTraceLimits, initRes RepoTraceResult) (RepoTraceResult, error) {
	lower := makeLowerNeedles(needles)
	st := &traceState{
		root:         root,
		needles:      needles,
		lowerNeedles: lower,
		effective:    eff,
		redactor:     browsersecurity.NewRedactor(nil),
		result:       initRes,
		matches:      []RepoMatch{},
		ctx:          ctx,
	}
	walkErr := filepath.WalkDir(root, st.walkFunc)
	_ = walkErr
	sort.Slice(st.matches, func(i, j int) bool {
		if st.matches[i].Path != st.matches[j].Path {
			return st.matches[i].Path < st.matches[j].Path
		}
		if st.matches[i].Line != st.matches[j].Line {
			return st.matches[i].Line < st.matches[j].Line
		}
		return st.matches[i].Needle < st.matches[j].Needle
	})
	st.result.Matches = st.matches
	if st.result.Matches == nil {
		st.result.Matches = []RepoMatch{}
	}
	return st.result, nil
}

func (s *traceState) walkFunc(path string, d fs.DirEntry, walkErr error) error {
	if s.ctx != nil && s.ctx.Err() != nil {
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("context cancelled: %v", s.ctx.Err()))
		s.result.Truncated = true
		return fs.SkipAll
	}
	if walkErr != nil {
		rel := relForNote(s.root, path)
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("walk error at %q: %v", rel, walkErr))
		if d != nil && d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if _, cerr := browsersecurity.ConfineToRoot(s.root, path); cerr != nil {
		rel := relForNote(s.root, path)
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("skipped path outside root %q: %v", rel, cerr))
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if d.IsDir() {
		return s.handleDir(path)
	}
	return s.handleFile(path)
}

func (s *traceState) handleDir(path string) error {
	if path == s.root {
		return nil
	}
	base := filepath.Base(path)
	if isSkippedRepoDir(base) {
		return fs.SkipDir
	}
	rel, _ := filepath.Rel(s.root, path)
	depth := repoDepth(rel)
	if depth > s.effective.MaxDepth {
		if !s.depthNoteAdded {
			s.result.Notes = append(s.result.Notes, fmt.Sprintf("skipped deep directory %q: depth %d exceeds limit %d", filepath.ToSlash(rel), depth, s.effective.MaxDepth))
			s.result.Truncated = true
			s.depthNoteAdded = true
		}
		return fs.SkipDir
	}
	return nil
}

func (s *traceState) handleFile(path string) error {
	if s.result.FilesScanned >= s.effective.MaxFiles {
		if !s.fileLimitNoteAdded {
			s.result.Notes = append(s.result.Notes, fmt.Sprintf("truncated files: limit %d reached", s.effective.MaxFiles))
			s.result.Truncated = true
			s.fileLimitNoteAdded = true
		}
		return fs.SkipAll
	}
	if s.ctx != nil && s.ctx.Err() != nil {
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("context cancelled: %v", s.ctx.Err()))
		s.result.Truncated = true
		return fs.SkipAll
	}
	relPath, _ := filepath.Rel(s.root, path)
	relSlash := filepath.ToSlash(relPath)
	f, err := os.Open(path)
	if err != nil {
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("unreadable file %q: %v", relSlash, err))
		s.result.FilesScanned++
		return nil
	}
	truncNote := fileTruncNote(f, relSlash, s.effective.MaxFileBytes)
	buf := make([]byte, s.effective.MaxFileBytes)
	n, rErr := io.ReadFull(f, buf)
	_ = f.Close()
	if rErr != nil && rErr != io.ErrUnexpectedEOF && rErr != io.EOF {
		s.result.Notes = append(s.result.Notes, fmt.Sprintf("unreadable file %q: %v", relSlash, rErr))
		s.result.FilesScanned++
		return nil
	}
	buf = buf[:n]
	s.result.FilesScanned++
	if truncNote != "" {
		s.result.Notes = append(s.result.Notes, truncNote)
	}
	if isBinary(buf) {
		return nil
	}
	s.collectMatches(relSlash, string(buf))
	if s.matchLimitNoteAdded {
		return fs.SkipAll
	}
	return nil
}

func fileTruncNote(f *os.File, relSlash string, limit int) string {
	stat, err := f.Stat()
	if err != nil {
		return ""
	}
	if stat.Size() > int64(limit) {
		return fmt.Sprintf("truncated file %q: %d bytes exceeds limit %d", relSlash, stat.Size(), limit)
	}
	return ""
}

func isBinary(buf []byte) bool {
	check := 512
	if len(buf) < check {
		check = len(buf)
	}
	return bytes.Contains(buf[:check], []byte{0})
}

func (s *traceState) collectMatches(relSlash, content string) {
	lines := strings.Split(content, "\n")
	for idx, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		s.processLine(relSlash, idx, line)
		if s.matchLimitNoteAdded {
			return
		}
	}
}

func (s *traceState) processLine(relSlash string, idx int, line string) {
	lowerLine := strings.ToLower(line)
	for ni, lowerNeedle := range s.lowerNeedles {
		if !strings.Contains(lowerLine, lowerNeedle) {
			continue
		}
		s.addMatch(relSlash, idx, line, lowerLine, ni)
		if s.matchLimitNoteAdded {
			return
		}
	}
}

func (s *traceState) addMatch(relSlash string, idx int, line, lowerLine string, ni int) {
	matchIdx := strings.Index(lowerLine, s.lowerNeedles[ni])
	if matchIdx < 0 {
		return
	}
	raw := boundedSnippet(line, matchIdx, len(s.needles[ni]))
	sanitized := s.redactor.SanitizeString(raw)
	s.matches = append(s.matches, RepoMatch{
		Path:    relSlash,
		Line:    idx + 1,
		Needle:  s.needles[ni],
		Snippet: sanitized,
	})
	if len(s.matches) >= s.effective.MaxMatches {
		if !s.matchLimitNoteAdded {
			s.result.Notes = append(s.result.Notes, fmt.Sprintf("truncated matches: limit %d reached", s.effective.MaxMatches))
			s.result.Truncated = true
			s.matchLimitNoteAdded = true
		}
	}
}

func relForNote(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.Base(path)
	}
	return filepath.ToSlash(rel)
}
