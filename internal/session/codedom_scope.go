package session

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// The run's CodeDOM scope.
//
// The world's ownership matrix (internal/world/world_predicates.go) gives
// code_element and its siblings to "the CodeDOM scope (session)": what is being
// looked at, not what is on disk. The only thing that ever opened such a scope
// is VirtualStore.handleOpenFile, behind a kernel-derived
// next_action(/open_file) -- and this path never reaches it, because the
// session executor does not consult next_action at all. So in a headless run
// the layer was empty by construction, not by accident.
//
// Measured on ladder run R1-16 (2026-09-19): 1,175 code_element queries in one
// run, not one row, against the same kernel where code_defines answered 950 of
// 1,216 and dependency_link 1,142 of 1,418. The world model was alive; only
// this layer was dark. With it dark, every rule in policy/codedom_edit.mg had
// nothing to fire on -- edit safety, breaking-change risk over
// element_visibility and element_parent, the API-handler warnings, the CodeDOM
// activation boosts: 197 lines of stratified policy deriving nothing.
//
// A turn does have something it is looking at: the file its last tool call
// touched, which recordWorkingResult already tracks as the working focus. So
// the turn scopes that file. The executor measures only which file it is; every
// decision taken over the resulting facts stays in the corpus.

// CodeElementSource parses one source file into its CodeDOM facts. Narrow
// interface so no import of internal/world is needed and no import cycle is
// possible -- the same reason fileContext is one.
type CodeElementSource interface {
	// FileFacts returns path's CodeDOM facts, keyed by the workspace-canonical
	// path, or nil when the file is not a source it can read.
	FileFacts(path string) ([]types.Fact, error)
}

// SetCodeElementSource attaches the parser the run's CodeDOM scope uses. With
// none attached the scope stays empty and nothing else changes.
func (e *Executor) SetCodeElementSource(src CodeElementSource) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.codeElements = src
	e.codedom = nil
}

// codedomScope is the set of files a run has parsed into the kernel, keyed by
// the canonical path its facts carry.
//
// The digest is the file's content as parsed. An edit re-parses, and the file's
// facts are REPLACED rather than added to: Mangle evaluation is monotone, so a
// stale code_element left beside a fresh one stays derivable forever, and every
// rule over it would see one element at two line ranges at once.
type codedomScope struct {
	mu    sync.Mutex
	src   CodeElementSource
	root  string
	files map[string]scopedFile
}

type scopedFile struct {
	digest string
	facts  []types.Fact
}

// scopeFocusFile puts the file the turn is looking at into the kernel's
// CodeDOM predicates. Every reason to do nothing -- no parser attached, no
// workspace, an entity that is not a file, a file that does not parse -- is a
// quiet return: the scope is an addition to what the turn can reason over,
// never a condition on it proceeding.
func (e *Executor) scopeFocusFile(entity string) {
	if entity == "" || entity == "." {
		return
	}
	root := e.workspaceForVerification()
	if root == "" || e.kernel == nil {
		return
	}
	e.mu.Lock()
	src := e.codeElements
	if src == nil {
		e.mu.Unlock()
		return
	}
	if e.codedom == nil || e.codedom.root != root {
		e.codedom = &codedomScope{src: src, root: root, files: make(map[string]scopedFile)}
	}
	scope := e.codedom
	e.mu.Unlock()
	scope.ensure(e.kernel, entity)
}

func (s *codedomScope) ensure(kernel types.Kernel, entity string) {
	abs := entity
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(s.root, filepath.FromSlash(entity))
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:8])
	canonical := types.CanonicalPath(s.root, abs)

	s.mu.Lock()
	defer s.mu.Unlock()
	if prior, ok := s.files[canonical]; ok && prior.digest == digest {
		return
	}
	facts, err := s.src.FileFacts(abs)
	if err != nil || len(facts) == 0 {
		return
	}
	if prior, ok := s.files[canonical]; ok && len(prior.facts) > 0 {
		if err := kernel.RetractExactFactsBatch(prior.facts); err != nil {
			// Leaving the old facts asserted beside the new ones is worse than
			// not refreshing: the file would be derivable at two revisions.
			logging.Get(logging.CategorySession).Warn(
				"codedom scope: %s keeps its previous elements, retraction failed: %v", canonical, err)
			return
		}
	}
	if err := kernel.LoadFacts(facts); err != nil {
		logging.Get(logging.CategorySession).Warn("codedom scope: %s not scoped: %v", canonical, err)
		delete(s.files, canonical)
		return
	}
	s.files[canonical] = scopedFile{digest: digest, facts: facts}
	logging.Get(logging.CategorySession).Debug("codedom scope: %s -> %d fact(s)", canonical, len(facts))
}
