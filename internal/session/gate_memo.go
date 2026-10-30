package session

import (
	"codenerd/internal/evidence"
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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
// "importers", "full") over the inputs the measurement reads. Besides the
// bytes of the turn's packages (the whole workspace during a fix, whose
// checks also depend on imports, neighbouring tests, build files and config
// outside WrittenPaths) it names exactly which paths the turn wrote and what
// each held before: the gate's obligations follow membership (a written
// tagged test is run under its tags; coverage is projected onto the written
// files) and its baseline follows the preimages, and rewriting a file with
// its own bytes adds a path without changing a byte. An unavailable snapshot
// disables memoization; it never supplies a stale pass.
func gateKey(ctx context.Context, kind, workspace string, result *ExecutionResult) string {
	fingerprint := writtenFingerprint(workspace, result.WrittenPaths)
	if result.Intent.Verb == "/fix" {
		var err error
		fingerprint, err = evidence.Snapshot(ctx, workspace)
		if err != nil {
			logging.Get(logging.CategorySession).Warn("test gate memo disabled: snapshot inputs: %v", err)
			return ""
		}
	}
	return kind + "|" + workspace + "|" + fingerprint + "|" + writtenMembership(result)
}

// writtenMembership digests the written path set and each path's preimage.
func writtenMembership(result *ExecutionResult) string {
	paths := append([]string(nil), result.WrittenPaths...)
	for i, p := range paths {
		paths[i] = filepath.ToSlash(filepath.Clean(p))
	}
	sort.Strings(paths)
	paths = slices.Compact(paths) // a path written twice is one member
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
	if key == "" {
		return gateRun{}, false
	}
	run, ok := result.gateRuns[key]
	return run, ok
}

// rememberGate keeps a finished measurement under key.
func rememberGate(result *ExecutionResult, key string, v TestVerification, uncovered []UncoveredBlock) {
	if key == "" {
		return
	}
	switch v.Verdict() {
	case VerifyCanceled, VerifyIndeterminate:
		return
	}
	if result.gateRuns == nil {
		result.gateRuns = map[string]gateRun{}
	}
	result.gateRuns[key] = gateRun{v: v, uncovered: uncovered}
}

// fixVerificationScope reads the executive's scope for the host-owned phase.
// Neither a model argument nor a tool name can advance a call to final.
func (e *Executor) fixVerificationScope(phase types.MangleAtom) (string, error) {
	if e.kernel == nil {
		return "", errors.New("no kernel to derive fix verification scope")
	}
	rows, err := e.kernel.Query("fix_verification_scope")
	if err != nil {
		return "", fmt.Errorf("query fix verification scope: %w", err)
	}
	scope := ""
	for _, row := range rows {
		// The kernel query is schema-bound to this predicate's two slots.
		if types.ExtractString(row.Args[0]) == string(phase) {
			if scope != "" {
				return "", errors.New("ambiguous fix verification scope")
			}
			scope = types.ExtractString(row.Args[1])
		}
	}
	if scope == "" {
		return "", fmt.Errorf("no fix verification scope for %s", phase)
	}
	return scope, nil
}

// finalFixTests runs only at closure, after the narrow gates on the current
// tree. Its key covers the whole workspace, not only the turn's written files.
// All outcomes are retained: a timeout is not permission to spend the same
// unchanged whole-suite run again, nor is it a passing measurement.
func finalFixTests(ctx context.Context, workspace string, result *ExecutionResult) (TestVerification, error) {
	for _, check := range []BuildVerification{result.BuildCheck, result.VetCheck, result.PinCheck} {
		if verdict := check.Verdict(); verdict != VerifySkipped && verdict != VerifyPassed {
			return TestVerification{Outcome: VerifyIndeterminate, Reason: "post-edit gates have not passed; final suite withheld"}, nil
		}
	}
	before, err := evidence.Snapshot(ctx, workspace)
	if err != nil {
		return TestVerification{}, fmt.Errorf("snapshot final test inputs: %w", err)
	}
	// Revalidate narrow obligations even when bytes have not changed: a new
	// written tagged test or baseline can make a previously green gate red.
	own, uncovered := gateTests(ctx, workspace, result, true)
	own.Repair = inheritRepair(own.Verdict(), result.TestCheck.Repair)
	result.TestCheck = own
	result.UncoveredBlocks = narrowToChangedLines(workspace, result, uncovered)
	if own.Verdict() != VerifyPassed {
		if failuresAllPredate(own) {
			return TestVerification{Outcome: VerifyIndeterminate, Reason: "narrow tests are not green; final suite withheld"}, nil
		}
		return own, nil
	}
	if verdict := result.ImporterCheck.Verdict(); verdict != VerifySkipped && verdict != VerifyPassed {
		return result.ImporterCheck, nil
	}
	key := "full|" + workspace + "|" + before
	if run, ok := recalledGate(result, key); ok {
		return run.v, nil
	}
	v := verifyTests(ctx, workspace, []string{"./..."}, "-count=1")
	after, err := evidence.Snapshot(ctx, workspace)
	if err != nil {
		v.Outcome = VerifyIndeterminate
		v.OK = false
		v.Reason = "final test inputs could not be rechecked"
		if result.gateRuns == nil {
			result.gateRuns = map[string]gateRun{}
		}
		result.gateRuns[key] = gateRun{v: v}
		return TestVerification{}, fmt.Errorf("snapshot final test outputs: %w", err)
	}
	if before != after {
		v.Outcome = VerifyIndeterminate
		v.OK = false
		v.Reason = "workspace changed during final test run"
	}
	if result.gateRuns == nil {
		result.gateRuns = map[string]gateRun{}
	}
	result.gateRuns[key] = gateRun{v: v}
	return v, nil
}

// constrainFixVerification applies the working phase's executive scope before
// dispatch. It never advances a model call into the host-owned final phase.
func (e *Executor) constrainFixVerification(call *types.ToolCall, result *ExecutionResult) error {
	if result == nil || result.Intent.Verb != "/fix" {
		return nil
	}
	switch call.Name {
	case "run_tests", "run_build", "run_lint":
	default:
		return nil
	}
	scope, err := e.fixVerificationScope(types.MangleAtom("/working"))
	if err != nil {
		return err
	}
	if scope != "/narrow" {
		return fmt.Errorf("unexpected working fix verification scope %s", scope)
	}
	var packages []string
	switch values := call.Input["packages"].(type) {
	case nil:
	case []string:
		packages = values
	case []any:
		for _, v := range values {
			s, ok := v.(string)
			if !ok {
				return errors.New("packages must contain strings")
			}
			packages = append(packages, s)
		}
	default:
		return errors.New("packages must be an array")
	}
	root := e.workspaceForVerification()
	dir := root
	if requested, ok := call.Input["working_dir"].(string); ok && strings.TrimSpace(requested) != "" {
		dir = requested
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(root, dir)
		}
	}
	if filepath.Clean(dir) != filepath.Clean(root) {
		return nil
	} // a subtree is already a narrower run; the tool checks containment
	explicit := false
	for _, pkg := range packages {
		pkg = strings.TrimSpace(pkg)
		if pkg == "" || pkg == "null" || pkg == "undefined" {
			continue
		}
		explicit = true
		if filepath.ToSlash(filepath.Clean(pkg)) == "..." || pkg == "all" {
			return errors.New("narrow-first fix policy: repository-wide checks belong to the final gate; run covering tests or edited packages while editing")
		}
	}
	if !explicit && call.Name == "run_build" {
		packages = packagesForPaths(result.WrittenPaths)
		if len(packages) == 0 {
			return errors.New("narrow-first fix policy: specify the package to diagnose before any Go edits")
		}
		input := make(map[string]any, len(call.Input)+1)
		for k, v := range call.Input {
			input[k] = v
		}
		input["packages"] = packages
		call.Input = input
	}
	return nil
}

// buildPackagesForTurn keeps fix/repair compilation on the edited packages.
// The final go test ./... compiles the whole repository; other intents retain
// their existing workspace build contract.
func buildPackagesForTurn(result *ExecutionResult) []string {
	if result != nil && result.Intent.Verb == "/fix" {
		return packagesForPaths(result.WrittenPaths)
	}
	return nil
}
