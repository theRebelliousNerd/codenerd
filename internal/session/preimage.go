package session

import (
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
