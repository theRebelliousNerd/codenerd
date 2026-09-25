package context

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/tools"
)

// Working-context archives are born one per working scope: every executor
// that runs a tool loop -- the session's, and a fresh clone for every
// delegated task and subagent, which a campaign mints by the thousand -- opens
// .nerd/context/<sha256(scope)>.db. Nothing ever removed one. Measured
// 2026-09-19 on the dogfood workspace: 170 archives, 86 MB, and 320 by
// 2026-09-21 (elite ladder, program of work item 15). Unattended hardening's
// fourth requirement is that nothing grows without bound.
//
// The retention decision is working_retention.mg's, over facts measured here:
// each archive's owner record (written before its database, by the process
// that minted the scope), whether that process still runs, and whether its
// executor retired the scope. A scope's name is random and lives only in its
// executor's memory, so an archive whose owner is gone, or retired it, can
// never be redeemed; nothing about its age enters the decision.

//go:embed working_retention.mg
var workingRetentionPolicy string

// ownerSuffix names an archive's owner record, beside its database.
const ownerSuffix = ".owner"

// workingOwner is the owner record of one archive.
type workingOwner struct {
	PID     int    `json:"pid"`
	Host    string `json:"host,omitempty"`
	Retired bool   `json:"retired,omitempty"`
}

// Owner states a survey reports.
const (
	ArchiveOwnerLive       = "live"       // minted by a running process on this host
	ArchiveOwnerForeign    = "foreign"    // minted on another host
	ArchiveOwnerRetired    = "retired"    // its executor finished with the scope
	ArchiveOwnerGone       = "gone"       // its process no longer runs
	ArchiveOwnerUnrecorded = "unrecorded" // minted before owners were recorded
)

// WorkingArchive is one archive as a survey found it.
type WorkingArchive struct {
	Name     string // the scope digest
	Bytes    int64  // database, journal and owner record together
	Owner    string // one of the ArchiveOwner* states
	Prunable bool   // working_archive_prunable derived
}

// WorkingArchiveReport is a survey, and when it pruned, what it removed.
type WorkingArchiveReport struct {
	Dir           string
	Archives      []WorkingArchive
	Total         int
	Redeemable    int
	Prunable      int
	Bytes         int64
	PrunableBytes int64
	Removed       int
	RemovedBytes  int64
	// Failures are archives the policy released and the file system kept
	// (on Windows, a file another process holds open). They stay on disk and
	// in Total; the next prune tries again.
	Failures []string
}

// String is the one-line summary `nerd status` and `nerd memory` print.
func (r WorkingArchiveReport) String() string {
	s := fmt.Sprintf("%d archive(s), %s: %d redeemable, %d prunable (%s)",
		r.Total, humanBytes(r.Bytes), r.Redeemable, r.Prunable, humanBytes(r.PrunableBytes))
	if r.Removed > 0 {
		s += fmt.Sprintf("; removed %d (%s)", r.Removed, humanBytes(r.RemovedBytes))
	}
	if len(r.Failures) > 0 {
		s += fmt.Sprintf("; %d could not be removed", len(r.Failures))
	}
	return s
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// workingContextDir is <workspace>/.nerd/context, resolved and contained.
func workingContextDir(workspace string) (string, error) {
	return tools.ResolveWorkspacePath(context.Background(), workspace, filepath.Join(".nerd", "context"))
}

// thisHost names the host owner records are compared against. A host whose
// name cannot be read records "", and "" matches only itself.
func thisHost() string {
	h, _ := os.Hostname()
	return h
}

// recordWorkingOwner writes the owner record for digest before its database
// exists, so a survey never meets a database of this process without its
// owner. The write is a rename, so a survey never reads half a record.
func recordWorkingOwner(dir, digest string, o workingOwner) error {
	data, err := json.Marshal(o)
	if err != nil {
		return err
	}
	// The writer's pid is in the name, so a survey can tell a record being
	// written (its writer runs) from one a crash left half-way (it does not).
	tmp, err := os.CreateTemp(dir, fmt.Sprintf("%s%s.%d.tmp*", digest, ownerSuffix, os.Getpid()))
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, filepath.Join(dir, digest+ownerSuffix)); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

func readWorkingOwner(dir, digest string) (workingOwner, bool) {
	data, err := os.ReadFile(filepath.Join(dir, digest+ownerSuffix))
	if err != nil {
		return workingOwner{}, false
	}
	var o workingOwner
	if json.Unmarshal(data, &o) != nil {
		return workingOwner{}, false
	}
	return o, true
}

// RetireWorkingScope records that scope's executor is done with it -- a task
// clone or a subagent that finished its one task -- and prunes its archive:
// with the scope's name gone from memory, nothing can redeem a handle into it.
func RetireWorkingScope(workspace, scope string) (WorkingArchiveReport, error) {
	dir, err := workingContextDir(workspace)
	if err != nil {
		return WorkingArchiveReport{}, err
	}
	digest := workingDigest(scope)
	o, ok := readWorkingOwner(dir, digest)
	if !ok {
		// The scope never opened an archive (no tool loop ran), or it is
		// already gone.
		return WorkingArchiveReport{Dir: dir}, nil
	}
	o.Retired = true
	if err := recordWorkingOwner(dir, digest, o); err != nil {
		return WorkingArchiveReport{Dir: dir}, fmt.Errorf("retire working scope: %w", err)
	}
	return surveyArchives(dir, map[string]bool{digest: true}, true)
}

// SurveyWorkingArchives reports every archive under workspace and the
// policy's verdict on each, deleting nothing.
func SurveyWorkingArchives(workspace string) (WorkingArchiveReport, error) {
	dir, err := workingContextDir(workspace)
	if err != nil {
		return WorkingArchiveReport{}, err
	}
	return surveyArchives(dir, nil, false)
}

// PruneWorkingArchives deletes every archive the policy derives prunable and
// reports what it kept and what it removed.
func PruneWorkingArchives(workspace string) (WorkingArchiveReport, error) {
	dir, err := workingContextDir(workspace)
	if err != nil {
		return WorkingArchiveReport{}, err
	}
	return surveyArchives(dir, nil, true)
}

// archiveFiles groups the directory's files by the scope digest they belong
// to. A name that does not start with a 64-hex digest is not an archive's and
// is never touched.
func archiveFiles(dir string) (map[string][]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	groups := make(map[string][]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		digest, rest, _ := strings.Cut(name, ".")
		if pid, ok := ownerTempWriter(rest); ok && processAlive(pid) {
			// An owner record being written: until its rename lands, the
			// scope it names has no archive to judge. One whose writer is
			// gone was left by a crash and belongs to its archive's files.
			continue
		}
		if len(digest) != 64 {
			continue
		}
		if _, err := hex.DecodeString(digest); err != nil {
			continue
		}
		groups[digest] = append(groups[digest], name)
	}
	return groups, nil
}

// ownerTempWriter reads the writer's pid from the part of a file name after
// the digest, when it is an owner record's temporary ("owner.<pid>.tmp<n>").
func ownerTempWriter(rest string) (int, bool) {
	after, ok := strings.CutPrefix(rest, strings.TrimPrefix(ownerSuffix, ".")+".")
	if !ok {
		return 0, false
	}
	pidText, tail, ok := strings.Cut(after, ".")
	if !ok || !strings.HasPrefix(tail, "tmp") {
		return 0, false
	}
	pid, err := strconv.Atoi(pidText)
	if err != nil {
		return 0, false
	}
	return pid, true
}

// surveyArchives measures the archives (all, or only those named), asks the
// retention policy which are prunable, and when prune is set deletes them.
func surveyArchives(dir string, only map[string]bool, prune bool) (WorkingArchiveReport, error) {
	report := WorkingArchiveReport{Dir: dir}
	groups, err := archiveFiles(dir)
	if err != nil {
		return report, err
	}
	host := thisHost()
	var facts []mangle.Fact
	archives := make(map[string]*WorkingArchive, len(groups))
	for digest, files := range groups {
		if only != nil && !only[digest] {
			continue
		}
		a := &WorkingArchive{Name: digest}
		for _, f := range files {
			if info, err := os.Stat(filepath.Join(dir, f)); err == nil {
				a.Bytes += info.Size()
			}
		}
		facts = append(facts, mangle.Fact{Predicate: "working_archive", Args: []any{digest}})
		o, recorded := readWorkingOwner(dir, digest)
		switch {
		case !recorded:
			a.Owner = ArchiveOwnerUnrecorded
		case o.Retired:
			a.Owner = ArchiveOwnerRetired
			facts = append(facts, mangle.Fact{Predicate: "working_archive_retired", Args: []any{digest}})
		case o.Host != host:
			a.Owner = ArchiveOwnerForeign
			facts = append(facts, mangle.Fact{Predicate: "working_archive_owner_foreign", Args: []any{digest}})
		case processAlive(o.PID):
			a.Owner = ArchiveOwnerLive
			facts = append(facts, mangle.Fact{Predicate: "working_archive_owner_live", Args: []any{digest}})
		default:
			a.Owner = ArchiveOwnerGone
		}
		archives[digest] = a
	}
	if len(archives) == 0 {
		return report, nil
	}
	prunable, err := derivePrunable(facts)
	if err != nil {
		return report, fmt.Errorf("working retention policy: %w", err)
	}
	digests := make([]string, 0, len(archives))
	for d := range archives {
		digests = append(digests, d)
	}
	sort.Strings(digests)
	for _, d := range digests {
		a := archives[d]
		a.Prunable = prunable[d]
		report.Total++
		report.Bytes += a.Bytes
		if !a.Prunable {
			report.Redeemable++
			report.Archives = append(report.Archives, *a)
			continue
		}
		report.Prunable++
		report.PrunableBytes += a.Bytes
		if prune {
			if err := removeArchive(dir, groups[d]); err != nil {
				report.Failures = append(report.Failures, fmt.Sprintf("%s: %v", d, err))
				report.Archives = append(report.Archives, *a)
				continue
			}
			report.Removed++
			report.RemovedBytes += a.Bytes
			continue
		}
		report.Archives = append(report.Archives, *a)
	}
	if report.Removed > 0 || len(report.Failures) > 0 {
		logging.Context("working context: %s", report)
	}
	for _, f := range report.Failures {
		logging.Get(logging.CategoryContext).Warn("working context archive the retention policy released was not removed: %s", f)
	}
	return report, nil
}

// derivePrunable evaluates working_retention.mg over the survey's facts in a
// private engine and returns the archives it derives prunable.
func derivePrunable(facts []mangle.Fact) (map[string]bool, error) {
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = true
	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, err
	}
	defer engine.Close()
	if err := engine.LoadSchemaString(workingRetentionPolicy); err != nil {
		return nil, err
	}
	if err := engine.AddFacts(facts); err != nil {
		return nil, err
	}
	if err := engine.Evaluate(); err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, f := range engine.QueryFacts("working_archive_prunable") {
		if len(f.Args) == 1 {
			out[fmt.Sprint(f.Args[0])] = true
		}
	}
	return out, nil
}

// removeArchive deletes one archive's files, its owner record last: a
// database that cannot be removed keeps the record that explains it.
func removeArchive(dir string, files []string) error {
	var owner string
	for _, f := range files {
		if strings.HasSuffix(f, ownerSuffix) {
			owner = f
			continue
		}
		if err := os.Remove(filepath.Join(dir, f)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if owner != "" {
		if err := os.Remove(filepath.Join(dir, owner)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
