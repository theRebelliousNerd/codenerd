package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// A turn's test gate is measured more than once: the post-edit gate, the
// rechecks of the coverage, vet, removed-test and pinning rounds, and the
// closure that re-measures every gate the schedule ran. Each launched go test
// over the turn's packages and their importers again, whether or not the turn
// had written anything since. Ladder R1-24 (2026-10-29): the importer gate --
// 8 packages, about 2.5 minutes -- ran at 03:43:47 and again at 03:47:25 with
// only the critic between them, and again at 04:06:13 and 04:08:51; R1-23 did
// the same at 02:33 and 02:36. Five minutes of a 50-minute turn measured an
// identical tree.
//
// A measurement is now keyed on what it measured and on the content of every
// file in the turn's own packages (writtenFingerprint). A later request for the same measurement on the same
// key is answered by the one already made; any write changes the key. A run
// that did not finish (canceled, or out of budget) is not a measurement and
// is not kept.

// gateRun is one finished test-gate measurement.
type gateRun struct {
	v         TestVerification
	uncovered []UncoveredBlock
}

// writtenFingerprint identifies, as they stand on disk now, every file the
// turn wrote and every file beside one of them -- the turn's own packages --
// so a change a shell command made there, which no write tool recorded, is a
// new revision too. A missing file and an unreadable one are distinct from
// each other and from any content.
func writtenFingerprint(workspace string, written []string) string {
	files := map[string]bool{}
	dirs := map[string]bool{}
	for _, p := range written {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, filepath.FromSlash(p))
		}
		files[abs] = true
		dirs[filepath.Dir(abs)] = true
	}
	return filesFingerprint(files, dirs)
}

// dirsFingerprint identifies the regular files of each directory as they
// stand: the packages a measurement read beyond the turn's own.
func dirsFingerprint(dirList []string) string {
	dirs := map[string]bool{}
	for _, d := range dirList {
		dirs[d] = true
	}
	return filesFingerprint(map[string]bool{}, dirs)
}

// filesFingerprint hashes files plus every regular file of dirs.
func filesFingerprint(files, dirs map[string]bool) string {
	for dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				files[filepath.Join(dir, entry.Name())] = true
			}
		}
	}
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		data, err := os.ReadFile(p)
		switch {
		case os.IsNotExist(err):
			fmt.Fprintf(h, "%s\x00absent\x00", p)
		case err != nil:
			fmt.Fprintf(h, "%s\x00unreadable\x00", p)
		default:
			sum := sha256.Sum256(data)
			fmt.Fprintf(h, "%s\x00%x\x00", p, sum)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// gateKey is the memo key of the measurement kind ("own", "own+coverage",
// "importers") over the turn's writes as they stand. Besides the bytes of the
// turn's packages it names exactly which paths the turn wrote and what each
// held before: the gate's obligations follow membership (a written tagged
// test is run under its tags; coverage is projected onto the written files)
// and its baseline follows the preimages, and rewriting a file with its own
// bytes adds a path without changing a byte.
func gateKey(kind, workspace string, result *ExecutionResult) string {
	return kind + "|" + workspace + "|" + writtenFingerprint(workspace, result.WrittenPaths) +
		"|" + writtenMembership(result)
}

// writtenMembership digests the written path set and each path's preimage.
func writtenMembership(result *ExecutionResult) string {
	paths := append([]string(nil), result.WrittenPaths...)
	for i, p := range paths {
		paths[i] = filepath.ToSlash(filepath.Clean(p))
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		img, ok := result.PreWriteContents[p]
		switch {
		case !ok:
			fmt.Fprintf(h, "%s\x00nopreimage\x00", p)
		case img.Unknown != "":
			fmt.Fprintf(h, "%s\x00unknown\x00", p)
		case !img.Existed:
			fmt.Fprintf(h, "%s\x00new\x00", p)
		default:
			fmt.Fprintf(h, "%s\x00%x\x00", p, sha256.Sum256([]byte(img.Content)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// recalledGate is the measurement made under key, if there is one.
func recalledGate(result *ExecutionResult, key string) (gateRun, bool) {
	run, ok := result.gateRuns[key]
	return run, ok
}

// rememberGate keeps a finished measurement under key.
func rememberGate(result *ExecutionResult, key string, v TestVerification, uncovered []UncoveredBlock) {
	switch v.Verdict() {
	case VerifyCanceled, VerifyIndeterminate:
		return
	}
	if result.gateRuns == nil {
		result.gateRuns = map[string]gateRun{}
	}
	result.gateRuns[key] = gateRun{v: v, uncovered: uncovered}
}
