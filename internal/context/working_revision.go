package context

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"

	"codenerd/internal/tools"
	"codenerd/internal/world/codemodel"
)

// elementSeparator joins a file and an element key in an element entity. A key
// may hold "#" ("init#2") and ":" ("rule:pred/2@3fa2c1"); a workspace-relative
// path holds neither "::" nor a key.
const elementSeparator = "::"

const (
	workingRevisionUnavailable = "unavailable"
	workingRevisionAbsent      = "absent"
	workingRevisionNonFile     = "nonfile"
)

// ElementEntity is the working-set entity of one element of a file: its
// observations are dated by the element's own bytes, not the file's.
func ElementEntity(file, key string) string { return file + elementSeparator + key }

// EntityFile is the file an entity is in: the entity itself for a file, the
// file part of an element entity.
func EntityFile(entity string) string {
	file, _, _ := strings.Cut(entity, elementSeparator)
	return file
}

// sameWorkingRevision says whether a saved observation can be identified with
// the source as it exists now. "absent" and "unavailable" are lookup
// outcomes, not content identities: equal sentinel values cannot prove that
// historical bytes are current. "nonfile" is different: it explicitly
// records an observation without a source file and remains stable for that
// entity.
func sameWorkingRevision(saved, current string) bool {
	if saved != current {
		return false
	}
	switch saved {
	case workingRevisionAbsent, workingRevisionUnavailable:
		return false
	default:
		return true
	}
}

// Revision is content identity, not HEAD: uncommitted edits invalidate views.
//
// A file entity is the hash of the whole file. An element entity
// (ElementEntity) is the element's own revision -- its bytes, doc comment
// included (codemodel.Revision) -- so an edit elsewhere in the file leaves the
// observations of the element current; "absent" once the file no longer has
// it. With whole-file revisions every edit staled every observation of every
// function in the file: measured on one campaign (2026-09-22), one file read
// 34 times and 21 of 45 Go reads re-reads.
func (w *WorkingSet) Revision(entity string) string {
	return w.revisions(context.Background(), []string{entity})[entity]
}

// revisions observes the current revision of every entity in one observation
// epoch: each source file is resolved and read once and, when one of its
// elements is asked for, parsed once per language, however many of its
// elements and whether its whole-file digest are asked for. One ledger pass
// used to read and parse a file once per element entity in it.
//
// Each entity gets exactly what Revision gave it alone: the digest, the
// element's revision, "absent" for a key the parse did not retain (a partly
// parsed file keeps the elements it did parse -- a syntax error is not
// unavailability), "unavailable" for an unresolvable or unreadable file or an
// unsupported language, "nonfile" for the empty entity. The epoch is an
// interval, not an atomic snapshot of several files; nothing outlives the
// call, so a later round observes again. A canceled ctx leaves the remaining
// files unavailable rather than claiming a revision nobody read.
func (w *WorkingSet) revisions(ctx context.Context, entities []string) map[string]string {
	out := make(map[string]string, len(entities))
	type fileGroup struct {
		whole    []string // file entities
		elements []string // element entities
	}
	groups := map[string]*fileGroup{}
	var order []string
	for _, entity := range entities {
		if _, done := out[entity]; done {
			continue
		}
		if entity == "" {
			out[entity] = workingRevisionNonFile
			continue
		}
		out[entity] = workingRevisionUnavailable
		file, _, element := strings.Cut(entity, elementSeparator)
		path, err := tools.ResolveWorkspacePath(ctx, w.root, file)
		if err != nil {
			continue
		}
		g := groups[path]
		if g == nil {
			g = &fileGroup{}
			groups[path] = g
			order = append(order, path)
		}
		if element {
			g.elements = append(g.elements, entity)
		} else {
			g.whole = append(g.whole, entity)
		}
	}
	for _, path := range order {
		if ctx.Err() != nil {
			break
		}
		g := groups[path]
		if len(g.elements) == 0 {
			// The digest alone streams: a large file need not be held.
			if digest, ok := streamDigest(path); ok {
				for _, entity := range g.whole {
					out[entity] = digest
				}
			}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(g.whole) > 0 {
			sum := sha256.Sum256(data)
			digest := hex.EncodeToString(sum[:])
			for _, entity := range g.whole {
				out[entity] = digest
			}
		}
		// The language is the entity's spelling's, as Revision always took
		// it; one parse per language the group's spellings name.
		parsed := map[string]map[string]string{}
		for _, entity := range g.elements {
			file, key, _ := strings.Cut(entity, elementSeparator)
			lang := codemodel.LanguageOf(file)
			byKey, seen := parsed[lang]
			if !seen {
				if model, ok := codemodel.Parse(file, string(data)); ok {
					byKey = make(map[string]string, len(model.Elements))
					for i := range model.Elements {
						// The first element with a key wins, as the linear
						// search Revision ran did.
						if _, dup := byKey[model.Elements[i].Key]; !dup {
							byKey[model.Elements[i].Key] = model.Elements[i].Revision
						}
					}
				}
				parsed[lang] = byKey
			}
			if byKey == nil {
				continue // unsupported language: unavailable
			}
			if revision, ok := byKey[key]; ok {
				out[entity] = revision
			} else {
				out[entity] = workingRevisionAbsent
			}
		}
	}
	return out
}

// streamDigest is the sha256 of a file read as a stream.
func streamDigest(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", false
	}
	return hex.EncodeToString(h.Sum(nil)), true
}
