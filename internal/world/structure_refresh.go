package world

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/workspace"
	"codenerd/internal/world/codemodel"
)

// =============================================================================
// STRUCTURE INDEX REFRESH AND ITS OBSERVATION RECORD
// =============================================================================
// A refresh is an observation of the tree, and an observation can fail: a
// directory that cannot be listed, a file that cannot be stat'ed or read. A
// failed observation is not evidence that the file is gone. Before 2026-10-27
// the walk skipped those paths and then deleted every entry it had not seen,
// so an unreadable caller directory made callers_of answer "no call site
// anywhere" and let delete_element remove a used declaration
// (Docs/audits/structure-coverage-and-refactor-preconditions.md, S1).
//
// Now each such path is a deficit. The entries it covers stay as they were
// last observed, and every answer carries the deficits that intersect its
// domain, so no consumer can read an inability to observe as absence.
// Membership exclusions, secret paths and files that are genuinely gone are
// still removed: those are observations, not failures.

// StructDeficit is one path the last refresh, or a query's own read, could
// not observe. Reason names the failure, never the file's contents.
type StructDeficit struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// StructureStats describes the index after a refresh.
type StructureStats struct {
	Files int `json:"files"`
	// SourceBytes is the byte length of the sources the index holds: the
	// material a whole-index query is answered over.
	SourceBytes  int64         `json:"source_bytes"`
	Symbols      int           `json:"symbols"`
	Calls        int           `json:"calls"`
	Reparsed     int           `json:"reparsed"`
	Broken       int           `json:"broken"`
	RefreshTook  time.Duration `json:"-"`
	RefreshTookS string        `json:"refresh_took"`
	// Generation is the index state the answer was computed from.
	Generation uint64 `json:"generation"`
	// Deficits are the paths inside the answer's domain that could not be
	// observed. An answer with deficits establishes no absence.
	Deficits []StructDeficit `json:"deficits,omitempty"`
}

// Line renders the stats as the footer every structural answer carries.
func (s StructureStats) Line() string {
	line := fmt.Sprintf("%d files, %d declarations, %d call sites; %d reparsed for this answer (%s); generation %d",
		s.Files, s.Symbols, s.Calls, s.Reparsed, s.RefreshTookS, s.Generation)
	if s.Broken > 0 {
		line += fmt.Sprintf("; %d files do not parse (get_elements shows their errors)", s.Broken)
	}
	return line
}

// scoped keeps the deficits that intersect a path-scoped query's domain: a
// deficit inside the target, or one on a directory holding it. A local
// question stays answerable while an unrelated directory is unreadable.
func (s StructureStats) scoped(target string) StructureStats {
	if target == "" || len(s.Deficits) == 0 {
		return s
	}
	var kept []StructDeficit
	for _, d := range s.Deficits {
		if pathWithin(d.Path, target) || pathWithin(target, d.Path) {
			kept = append(kept, d)
		}
	}
	s.Deficits = kept
	return s
}

// pathWithin reports whether path is dir or lies under it. The empty path is
// the workspace root.
func pathWithin(path, dir string) bool {
	dir = strings.Trim(dir, "/")
	path = strings.Trim(path, "/")
	return dir == "" || dir == "." || path == dir || strings.HasPrefix(path, dir+"/")
}

// structReadFile is the read seam of every source read the index makes after
// the content hash: the parse of a changed file, and the reads behind Uses
// and FindText. Tests replace it to fail one path deterministically.
var structReadFile = os.ReadFile

// contentDigest is the content identity the index and the edit tools share:
// the hex SHA-256 of the raw file bytes, the hash calculateHash computes.
func contentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// fingerprintDigest is the content hash a stored fingerprint carries, ""
// when the entry was indexed without a trustworthy stamp.
func fingerprintDigest(fp string) string {
	st, ok := parseContentFingerprint(fp)
	if !ok {
		return ""
	}
	return st.hash
}

// ioReason is a failure without the host path the error embeds: the
// operation and the system's reason.
func ioReason(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Op + ": " + pe.Err.Error()
	}
	return err.Error()
}

// deficitSet collects one observation's deficits by canonical path.
type deficitSet map[string]string

func (d deficitSet) add(path, reason string) {
	if _, ok := d[path]; !ok {
		d[path] = reason
	}
}

// covers reports whether an indexed file lies in a path the observation
// could not see, and so must be kept as it was last observed.
func (d deficitSet) covers(canonical string) bool {
	for path := range d {
		if pathWithin(canonical, path) {
			return true
		}
	}
	return false
}

func (d deficitSet) list() []StructDeficit {
	out := make([]StructDeficit, 0, len(d))
	for path, reason := range d {
		out = append(out, StructDeficit{Path: path, Reason: reason})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func sameDeficits(a, b []StructDeficit) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Refresh brings the index up to date with the tree. Caller holds no lock.
func (s *StructureIndex) Refresh(ctx context.Context) (StructureStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refreshLocked(ctx)
}

func (s *StructureIndex) refreshLocked(ctx context.Context) (StructureStats, error) {
	started := time.Now()
	type candidate struct{ fsPath, modelPath, canonical, fingerprint, hash string }
	var stale []candidate
	seen := make(map[string]struct{}, len(s.files))
	fingerprintUpdates := make(map[string]string)
	deficits := make(deficitSet)
	secretFiles := make(map[string]int)
	var generations dirGenSnapshot
	now := time.Now().UnixNano()

	mem, err := workspace.For(s.root)
	if err != nil {
		return StructureStats{}, err
	}
	if err := mem.Refresh(); err != nil {
		return StructureStats{}, err
	}

	err = filepath.WalkDir(s.root, func(path string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if d == nil || path == s.root {
				return fmt.Errorf("structure index cannot read the workspace root: %w", walkErr)
			}
			// WalkDir reports a directory it could not list after admitting
			// it. Its entries were not observed; they are a deficit, not gone.
			member, admErr := mem.Admit(path, d.IsDir())
			if errors.Is(admErr, fs.SkipDir) {
				return nil
			}
			if admErr != nil {
				return admErr
			}
			canonical := types.CanonicalPath(s.root, path)
			if member && !tools.IsSecretPath(canonical) {
				deficits.add(canonical, "directory not listed: "+ioReason(walkErr))
			}
			return nil
		}
		name := d.Name()
		member, admErr := mem.Admit(path, d.IsDir())
		if admErr != nil {
			return admErr
		}
		if !member {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if codemodel.LanguageOf(name) == "" {
			return nil
		}
		canonical := types.CanonicalPath(s.root, path)
		// A secret file (execution.secret_paths) is never parsed: its
		// signatures, doc lines and literals would reach the model through
		// find_symbol, package_outline and find_text as surely as a read.
		if tools.IsSecretPath(canonical) {
			secretFiles[codemodel.LanguageOf(name)]++
			return nil
		}
		info, err := d.Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				deficits.add(canonical, "stat failed: "+ioReason(err))
			}
			return nil
		}
		fsPath := path
		if info.Mode()&fs.ModeSymlink != 0 {
			// WalkDir admits the link by its workspace path, but DirEntry.Info
			// reports link metadata while parsing follows the link. Resolve and
			// gate the target before reading so the cache stamp describes the
			// bytes we index without opening a path outside workspace membership.
			resolved, resolveErr := tools.ResolveWorkspacePath(ctx, s.root, path)
			if resolveErr != nil {
				return nil
			}
			targetCanonical := types.CanonicalPath(s.root, resolved)
			if tools.IsSecretPath(targetCanonical) {
				secretFiles[codemodel.LanguageOf(name)]++
				return nil
			}
			targetMember, admitErr := mem.Admit(resolved, false)
			if admitErr != nil {
				return admitErr
			}
			if !targetMember {
				return nil
			}
			targetInfo, statErr := os.Stat(resolved)
			if statErr != nil {
				if !errors.Is(statErr, fs.ErrNotExist) {
					deficits.add(canonical, "stat failed: "+ioReason(statErr))
				}
				return nil
			}
			if targetInfo.IsDir() {
				return nil
			}
			fsPath, info = resolved, targetInfo
		}
		var previous contentStamp
		prev, exists := s.files[canonical]
		if exists {
			previous, _ = parseContentFingerprint(prev.fingerprint)
			if prev.sourcePath != fsPath {
				// A link can be retargeted without changing its own metadata.
				// Do not trust a stamp captured from a different target.
				previous = contentStamp{}
			}
		}
		current := stampFromInfo(fsPath, info, generations.of(filepath.Dir(fsPath)))
		hash, changed, hashErr := resolveContent(fsPath, current, previous, now)
		if hashErr != nil {
			if !errors.Is(hashErr, fs.ErrNotExist) {
				deficits.add(canonical, "read failed: "+ioReason(hashErr))
			}
			return nil
		}
		seen[canonical] = struct{}{}
		fp := formatContentFingerprint(current, hash)
		if exists && !changed {
			// Metadata-only changes such as a touch need to update the stored
			// content stamp so a later refresh can take the warm path, but they
			// do not require reparsing the source.
			if prev.fingerprint != fp {
				fingerprintUpdates[canonical] = fp
			}
			return nil
		}
		stale = append(stale, candidate{fsPath: fsPath, modelPath: path, canonical: canonical, fingerprint: fp, hash: hash})
		return nil
	})
	if err != nil {
		return StructureStats{}, err
	}
	for canonical, fingerprint := range fingerprintUpdates {
		if f, ok := s.files[canonical]; ok {
			f.fingerprint = fingerprint
		}
	}
	removed := 0
	for canonical := range s.files {
		if _, ok := seen[canonical]; ok || deficits.covers(canonical) {
			continue
		}
		delete(s.files, canonical)
		removed++
	}
	moduleChanged := s.refreshModuleLocked(&generations, now)

	reparsed := 0
	if len(stale) > 0 {
		parsed := make([]*structFile, len(stale))
		readErrs := make([]error, len(stale))
		workers := max(min(runtime.NumCPU(), 16), 2)
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup
		for i, c := range stale {
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				data, err := structReadFile(c.fsPath)
				if err != nil {
					readErrs[i] = err
					return
				}
				fp := c.fingerprint
				if contentDigest(data) != c.hash {
					// The file changed between its hash and this read. Index
					// the bytes read, with no stamp, so the next refresh
					// observes it again instead of trusting a stamp of other
					// bytes.
					fp = ""
				}
				parsed[i] = parseStructSource(data, c.modelPath, c.canonical, fp)
				parsed[i].size = len(data)
			})
		}
		wg.Wait()
		for i, c := range stale {
			if err := readErrs[i]; err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					if _, ok := s.files[c.canonical]; ok {
						delete(s.files, c.canonical)
						removed++
					}
					continue
				}
				// The hash read succeeded and this one did not. What was
				// indexed before stays, as the last observation.
				deficits.add(c.canonical, "read failed: "+ioReason(err))
				continue
			}
			next := parsed[i]
			next.sourcePath = c.fsPath
			if !next.parsed {
				if prev, ok := s.files[c.canonical]; ok {
					if prev.parsed {
						next.lastGood = prev.symbols
					} else {
						next.lastGood = prev.lastGood
					}
				}
			}
			s.files[c.canonical] = next
			reparsed++
		}
		s.assignRefsLocked()
	}

	observed := deficits.list()
	if reparsed > 0 || removed > 0 || moduleChanged || !sameDeficits(observed, s.deficits) {
		s.generation++
	}
	s.deficits = observed
	s.secretFiles = secretFiles

	stats := StructureStats{Files: len(s.files), Reparsed: reparsed, RefreshTook: time.Since(started), Generation: s.generation, Deficits: observed}
	for _, f := range s.files {
		stats.Symbols += len(f.symbols)
		stats.Calls += len(f.calls)
		stats.SourceBytes += int64(f.size)
		if !f.parsed {
			stats.Broken++
		}
	}
	stats.RefreshTookS = stats.RefreshTook.Round(time.Millisecond).String()
	if reparsed > 0 || removed > 0 || len(observed) > 0 {
		logging.World("structure index refreshed: files=%d symbols=%d calls=%d reparsed=%d removed=%d deficits=%d generation=%d in %v",
			stats.Files, stats.Symbols, stats.Calls, stats.Reparsed, removed, len(observed), stats.Generation, stats.RefreshTook)
	}
	return stats, nil
}

// refreshModuleLocked reads the module path from go.mod when it changed, and
// reports whether the module path moved.
func (s *StructureIndex) refreshModuleLocked(generations *dirGenSnapshot, now int64) bool {
	before := s.module
	path := filepath.Join(s.root, "go.mod")
	info, err := os.Stat(path)
	if err != nil {
		s.module, s.moduleFP = "", ""
		return before != ""
	}
	current := stampFromInfo(path, info, generations.of(filepath.Dir(path)))
	var previous contentStamp
	previous, _ = parseContentFingerprint(s.moduleFP)
	hash, changed, err := resolveContent(path, current, previous, now)
	if err != nil {
		// Keep the last known module path when a transient read failure prevents
		// checking whether go.mod changed.
		return false
	}
	fp := formatContentFingerprint(current, hash)
	if !changed {
		s.moduleFP = fp
		return false
	}
	s.module = ""
	data, err := os.ReadFile(path)
	if err != nil {
		return before != ""
	}
	s.moduleFP = fp
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			s.module = strings.Trim(strings.TrimSpace(rest), `"`)
			break
		}
	}
	return s.module != before
}
