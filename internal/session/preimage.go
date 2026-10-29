package session

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
)

// PreImage is what a written path held before the turn's first write to it.
//
// A file that did not exist and a file that existed empty are different
// preimages: when both were recorded as "", undoing a forcing round deleted a
// pre-existing zero-byte file instead of restoring it (external audit F7,
// 2026-09-19), and the test-baseline overlay treated it as deleted. A file
// that could not be read has no preimage at all: Unknown says why, and every
// consumer treats it as unknown -- never as absent.
type PreImage struct {
	// Existed is true when the path held a file before the turn.
	Existed bool
	// Content is that file's bytes; empty when it did not exist.
	Content string
	// Unknown is set when the path could not be read for a reason other than
	// not existing; Existed and Content then mean nothing.
	Unknown string
	// LastWriteHash is the hex sha256 of what the turn itself last wrote to
	// this path, recorded by the write hook after every successful write
	// mutation (and after gofmt, which rewrites outside the tools). The
	// restore compares the file on disk against it: a match means only the
	// turn wrote here since, a mismatch means another agent did and the
	// restore leaves the file as found instead of destroying that work
	// (dogfood run 5, AG7 finding 7). Empty with LastWriteAbsent false is
	// unrecorded -- a result built without the hook -- and restores as
	// before, so results the hook never saw keep their old behaviour.
	LastWriteHash string
	// LastWriteAbsent reports that the turn's last write to this path
	// removed it (delete_file): the disk matches when the path is absent.
	LastWriteAbsent bool
}

// noteLastWrittenBytes records bytes the turn, or a restore the turn's own
// harness just performed, left at path. A restore that does not update this
// leaves the hash on the pre-restore bytes, so the next restore reads its
// own undo as another agent's edit and refuses to put the file back
// (AG7 finding 7, the second restore). Absent is a removal. An entry the
// write hook never created is not invented here.
func (r *ExecutionResult) noteLastWrittenBytes(path string, content []byte, absent bool) {
	if r == nil || r.PreWriteContents == nil {
		return
	}
	before, ok := r.PreWriteContents[path]
	if !ok {
		return
	}
	if absent {
		before.LastWriteHash = ""
		before.LastWriteAbsent = true
	} else {
		sum := sha256.Sum256(content)
		before.LastWriteHash = hex.EncodeToString(sum[:])
		before.LastWriteAbsent = false
	}
	r.PreWriteContents[path] = before
}

// lastWriteMatches reports whether the bytes on disk now are what the turn
// itself last wrote: absent for a removal, the recorded hash otherwise.
// Unrecorded always matches: with nothing to compare against, the restore
// cannot tell another writer's bytes from the turn's and keeps its old
// behaviour. readErr other than not-exist also matches: an unreadable file
// is not evidence of another writer, and the restore attempt reports its
// own failure.
func (p PreImage) lastWriteMatches(current []byte, readErr error) bool {
	if p.LastWriteHash == "" && !p.LastWriteAbsent {
		return true
	}
	if readErr != nil {
		if !errors.Is(readErr, fs.ErrNotExist) {
			return true
		}
		return p.LastWriteAbsent
	}
	if p.LastWriteAbsent {
		return false
	}
	sum := sha256.Sum256(current)
	recorded, err := hex.DecodeString(p.LastWriteHash)
	// An undecodable record matches nothing: the restore cannot prove the
	// bytes are the turn's, so it leaves the file as found. (The hook only
	// ever writes hex; this is a corrupt record, not a missing one.)
	return err == nil && bytes.Equal(sum[:], recorded)
}

// Known reports whether the preimage was recorded: the file was read, or it
// did not exist.
func (p PreImage) Known() bool { return p.Unknown == "" }

// readPreImage reads what path holds now, as the preimage of a write about to
// happen.
func readPreImage(path string) PreImage {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		return PreImage{Existed: true, Content: string(data)}
	case errors.Is(err, fs.ErrNotExist):
		return PreImage{}
	default:
		return PreImage{Unknown: err.Error()}
	}
}
