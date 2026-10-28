package campaign

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	internaltypes "codenerd/internal/types"
)

// Recovery admission: which durable generation of a campaign may resume.
//
// Parsing a snapshot as JSON is not a verdict. saveCampaign journals a write
// request, replaces the snapshot, then journals the commit, and a process can
// die between any two of those steps; another writer can also replace the
// file. Every resume door therefore inspects the snapshot against the journal
// (InspectCampaignRecovery, read-only), asks the kernel whether that class of
// evidence may continue (campaign_rules.mg Section 14), preserves damaged
// journal bytes before repairing them, and only then hydrates the kernel and
// publishes. Verify, replay and resume all read the same classifier.

// RecoveryClass is what inspection observed. It is asserted as
// campaign_recovery_class; the kernel derives admission from it.
type RecoveryClass string

const (
	// RecoveryCommittedMatch: the snapshot hashes to the last committed write
	// and no later write is pending.
	RecoveryCommittedMatch RecoveryClass = "/committed_match"
	// RecoveryPreparedSnapshot: the last write request has no commit, and the
	// snapshot (or the temp file it was renamed from) is exactly what that
	// request wrote. The publication was interrupted after the bytes landed.
	RecoveryPreparedSnapshot RecoveryClass = "/prepared_snapshot_present"
	// RecoveryPreviousGeneration: the last write request has no commit and
	// never published; the snapshot (or the copy moved aside by a replacement
	// fallback) is still the previous committed generation.
	RecoveryPreviousGeneration RecoveryClass = "/previous_generation_present"
	// RecoveryConflict: the snapshot matches nothing the journal recorded, is
	// missing, belongs to another campaign, or the journal has no valid record.
	RecoveryConflict RecoveryClass = "/conflict_or_corruption"
	// RecoveryStorageUnreadable: the snapshot or journal could not be read.
	RecoveryStorageUnreadable RecoveryClass = "/storage_unreadable"
	// RecoveryLegacyUnjournaled: a snapshot with no journal at all.
	RecoveryLegacyUnjournaled RecoveryClass = "/legacy_unjournaled"
)

// RecoveryProvenance is how an admitted generation earned trust, as the
// kernel derived it (campaign_recovery_admitted).
type RecoveryProvenance string

const (
	ProvenanceCommitted             RecoveryProvenance = "/committed"
	ProvenanceReconciledPublication RecoveryProvenance = "/reconciled_publication"
	ProvenancePreviousGeneration    RecoveryProvenance = "/previous_generation"
	ProvenanceLegacyImport          RecoveryProvenance = "/legacy_import"
	ProvenanceOperatorAdopted       RecoveryProvenance = "/operator_adopted"
)

// JournalEventRecoveryAdmitted records an admission: the class observed, the
// provenance derived, and the generation's checksum. It closes any write
// request the admission found dangling.
const JournalEventRecoveryAdmitted = "recovery_admitted"

// RecoveryReport is the evidence an admission decision was made on.
type RecoveryReport struct {
	CampaignID   string `json:"campaign_id"`
	SnapshotPath string `json:"snapshot_path"`
	JournalPath  string `json:"journal_path"`

	Class  RecoveryClass `json:"class"`
	Detail string        `json:"detail"`

	// The journal's watermarks: the last write it asked for and the last one
	// it confirmed.
	JournalEvents         int              `json:"journal_events"`
	LastRequestedSeq      uint64           `json:"last_requested_seq,omitzero"`
	LastRequestedChecksum string           `json:"last_requested_checksum,omitzero"`
	LastCommittedSeq      uint64           `json:"last_committed_seq,omitzero"`
	LastCommittedChecksum string           `json:"last_committed_checksum,omitzero"`
	JournalDamaged        bool             `json:"journal_damaged,omitzero"`
	Problems              []JournalProblem `json:"problems,omitzero"`

	// CandidatePath is the file holding the generation: the snapshot itself,
	// or a temp/moved-aside file an interrupted replacement left behind.
	CandidatePath    string `json:"candidate_path,omitzero"`
	SnapshotChecksum string `json:"snapshot_checksum,omitzero"`

	// Set by admission.
	Admitted         bool               `json:"admitted"`
	Provenance       RecoveryProvenance `json:"provenance,omitzero"`
	PreservedJournal string             `json:"preserved_journal,omitzero"`
}

// AdmissionPolicy is the operator's say in recovery admission. It is asserted
// to the kernel; it does not decide by itself.
type AdmissionPolicy struct {
	// AdoptSnapshot admits a snapshot the journal does not vouch for
	// (/conflict_or_corruption) as a new provenance boundary. It never admits
	// unreadable storage.
	AdoptSnapshot bool
}

// RecoveryRefusedError is returned when the kernel refuses the observed class.
type RecoveryRefusedError struct {
	Report *RecoveryReport
}

func (e *RecoveryRefusedError) Error() string {
	if e == nil || e.Report == nil {
		return "campaign recovery refused"
	}
	return fmt.Sprintf("campaign %s recovery refused: %s: %s", e.Report.CampaignID, e.Report.Class, e.Report.Detail)
}

// InspectCampaignRecovery classifies a campaign's durable state. Read-only.
func InspectCampaignRecovery(workspace, campaignID string) *RecoveryReport {
	return inspectCampaignRecovery(filepath.Join(workspace, ".nerd", "campaigns"), campaignID)
}

type recoveryCandidate struct {
	path string
	data []byte
}

func inspectCampaignRecovery(campaignsDir, campaignID string) *RecoveryReport {
	r := &RecoveryReport{
		CampaignID:   campaignID,
		SnapshotPath: filepath.Join(campaignsDir, campaignID+".json"),
		JournalPath:  filepath.Join(campaignsDir, campaignID+".journal.jsonl"),
	}
	set := func(class RecoveryClass, format string, args ...any) *RecoveryReport {
		r.Class = class
		r.Detail = fmt.Sprintf(format, args...)
		return r
	}

	scan, jerr := scanJournal(r.JournalPath, campaignID)
	journalPresent := true
	if jerr != nil {
		if !os.IsNotExist(jerr) {
			return set(RecoveryStorageUnreadable, "journal unreadable: %v", jerr)
		}
		journalPresent = false
	}
	r.JournalEvents = len(scan.events)
	r.JournalDamaged = scan.damaged
	r.Problems = scan.problems
	for _, ev := range scan.events {
		switch ev.EventType {
		case JournalEventSnapshotRequested:
			r.LastRequestedSeq, r.LastRequestedChecksum = ev.Seq, ev.SnapshotChecksum
		case JournalEventSnapshotCommitted:
			r.LastCommittedSeq, r.LastCommittedChecksum = ev.Seq, ev.SnapshotChecksum
		}
	}

	snapshot, serr := os.ReadFile(r.SnapshotPath)
	if serr != nil && !os.IsNotExist(serr) {
		return set(RecoveryStorageUnreadable, "snapshot unreadable: %v", serr)
	}

	if !journalPresent || (len(scan.events) == 0 && !scan.damaged) {
		// Without journal evidence a stray replacement candidate proves
		// nothing; only the snapshot itself can be imported.
		if serr != nil {
			return set(RecoveryConflict, "no snapshot and no journal record; nothing to resume")
		}
		r.CandidatePath, r.SnapshotChecksum = r.SnapshotPath, checksumBytes(snapshot)
		if err := snapshotBelongsTo(snapshot, campaignID); err != nil {
			return set(RecoveryConflict, "snapshot without a journal is not this campaign: %v", err)
		}
		return set(RecoveryLegacyUnjournaled, "snapshot has no journal: importing it starts a new provenance boundary")
	}
	if len(scan.events) == 0 {
		return set(RecoveryConflict, "the journal has no valid record (%s)", describeProblems(scan.problems))
	}

	uncommitted := r.LastRequestedSeq > r.LastCommittedSeq
	match := func(sum string) RecoveryClass {
		switch {
		case !uncommitted && r.LastCommittedSeq > 0 && sum == r.LastCommittedChecksum:
			return RecoveryCommittedMatch
		case uncommitted && sum == r.LastRequestedChecksum:
			return RecoveryPreparedSnapshot
		case uncommitted && r.LastCommittedSeq > 0 && sum == r.LastCommittedChecksum:
			return RecoveryPreviousGeneration
		}
		return ""
	}

	var candidates []recoveryCandidate
	if serr == nil {
		candidates = []recoveryCandidate{{path: r.SnapshotPath, data: snapshot}}
	} else {
		found, err := replacementCandidates(campaignsDir, campaignID)
		if err != nil {
			return set(RecoveryStorageUnreadable, "snapshot missing and replacement candidates unreadable: %v", err)
		}
		if len(found) == 0 {
			return set(RecoveryConflict, "the journal records writes but no snapshot or replacement candidate exists")
		}
		candidates = found
	}

	// A prepared generation (the newer bytes) wins over the previous one when
	// an interrupted replacement left both behind.
	var best *recoveryCandidate
	var bestClass RecoveryClass
	for i := range candidates {
		class := match(checksumBytes(candidates[i].data))
		if class == "" || snapshotBelongsTo(candidates[i].data, campaignID) != nil {
			continue
		}
		if best == nil || (class == RecoveryPreparedSnapshot && bestClass != RecoveryPreparedSnapshot) {
			best, bestClass = &candidates[i], class
		}
	}
	if best == nil {
		r.CandidatePath, r.SnapshotChecksum = candidates[0].path, checksumBytes(candidates[0].data)
		return set(RecoveryConflict, "%s hashes to %s; the journal last requested %s (seq %d) and last committed %s (seq %d)",
			filepath.Base(candidates[0].path), shortChecksum(r.SnapshotChecksum),
			shortChecksum(r.LastRequestedChecksum), r.LastRequestedSeq,
			shortChecksum(r.LastCommittedChecksum), r.LastCommittedSeq)
	}
	r.CandidatePath, r.SnapshotChecksum = best.path, checksumBytes(best.data)
	switch bestClass {
	case RecoveryCommittedMatch:
		set(bestClass, "snapshot matches the last committed write (seq %d)", r.LastCommittedSeq)
	case RecoveryPreparedSnapshot:
		set(bestClass, "write seq %d landed in %s but its commit was never recorded", r.LastRequestedSeq, filepath.Base(best.path))
	case RecoveryPreviousGeneration:
		set(bestClass, "write seq %d never published; %s is the generation committed at seq %d",
			r.LastRequestedSeq, filepath.Base(best.path), r.LastCommittedSeq)
	}
	if r.JournalDamaged {
		r.Detail += fmt.Sprintf("; journal tail damaged after seq %d (%s)", scan.lastSeq(), describeProblems(scan.problems))
	}
	return r
}

// replacementCandidates lists what an interrupted snapshot replacement can
// leave when the named snapshot is gone: the synced temp file
// (<id>.json.tmp-*) and the previous snapshot moved aside by
// renameAtomicReplace's fallback (<id>.json.prev-*). Newest name first.
func replacementCandidates(campaignsDir, campaignID string) ([]recoveryCandidate, error) {
	base := filepath.Base(campaignID + ".json")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && (strings.HasPrefix(n, base+".tmp-") || strings.HasPrefix(n, base+".prev-")) {
			names = append(names, n)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	out := make([]recoveryCandidate, 0, len(names))
	for _, n := range names {
		p := filepath.Join(campaignsDir, n)
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, recoveryCandidate{path: p, data: data})
	}
	return out, nil
}

func snapshotBelongsTo(data []byte, campaignID string) error {
	var c Campaign
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("not a campaign snapshot: %w", err)
	}
	if sanitizeCampaignID(c.ID) != sanitizeCampaignID(campaignID) {
		return fmt.Errorf("snapshot is campaign %q", c.ID)
	}
	return nil
}

func describeProblems(problems []JournalProblem) string {
	if len(problems) == 0 {
		return "no defect recorded"
	}
	kinds := make([]string, 0, len(problems))
	for _, p := range problems {
		kinds = append(kinds, p.Kind)
	}
	return strings.Join(kinds, ", ")
}

// ResumeCampaign is the one door through which a persisted campaign resumes.
// It takes the cross-process ownership lock, classifies the durable state,
// lets the kernel decide admission, preserves and repairs a damaged journal,
// restores an interrupted replacement candidate, hydrates the kernel from the
// admitted bytes, records the admission in the journal and publishes a fresh
// committed generation. Nothing is written and no fact is hydrated before the
// kernel admits. A refusal returns *RecoveryRefusedError with the report.
//
// It does not change the campaign's status: Run publishes active when work
// actually starts, and PrepareResume re-arms a failed campaign.
func (o *Orchestrator) ResumeCampaign(campaignID string, policy AdmissionPolicy) (*RecoveryReport, error) {
	timer := logging.StartTimer(logging.CategoryCampaign, "ResumeCampaign")
	defer timer.Stop()

	o.mu.Lock()
	defer o.mu.Unlock()
	if o.isRunning {
		return nil, fmt.Errorf("cannot resume %s: a campaign is running on this orchestrator", campaignID)
	}
	if err := o.acquireOwnershipLocked(campaignID); err != nil {
		return nil, err
	}

	report := inspectCampaignRecovery(filepath.Join(o.nerdDir, "campaigns"), campaignID)
	logging.Campaign("Recovery inspection for %s: %s (%s)", campaignID, report.Class, report.Detail)
	provenance, err := o.deriveRecoveryAdmissionLocked(report, policy)
	if err != nil {
		return report, err
	}
	report.Admitted, report.Provenance = true, provenance

	// Evidence first: a damaged tail is copied aside before it is cut.
	_, preserved, err := o.recoverJournalLocked(campaignID)
	if err != nil {
		return report, fmt.Errorf("recovery admission of %s: %w", campaignID, err)
	}
	report.PreservedJournal = preserved

	if report.CandidatePath != report.SnapshotPath {
		if err := os.Rename(report.CandidatePath, report.SnapshotPath); err != nil {
			return report, fmt.Errorf("recovery admission of %s: restore %s: %w", campaignID, report.CandidatePath, err)
		}
		if err := syncDirIfSupported(filepath.Dir(report.SnapshotPath)); err != nil {
			return report, fmt.Errorf("recovery admission of %s: sync restored snapshot: %w", campaignID, err)
		}
	}
	data, err := os.ReadFile(report.SnapshotPath)
	if err != nil {
		return report, fmt.Errorf("recovery admission of %s: %w", campaignID, err)
	}
	if checksumBytes(data) != report.SnapshotChecksum {
		return report, fmt.Errorf("recovery admission of %s: the snapshot changed after it was inspected; resume again", campaignID)
	}
	var c Campaign
	if err := json.Unmarshal(data, &c); err != nil {
		return report, fmt.Errorf("recovery admission of %s: %w", campaignID, err)
	}

	if err := o.kernel.LoadFacts(c.ToFacts()); err != nil {
		return report, fmt.Errorf("recovery admission of %s: the kernel rejected the admitted generation: %w", campaignID, err)
	}
	o.campaign = &c
	o.publishPolicyParams()
	if o.contextPager != nil && c.ContextBudget > 0 {
		o.contextPager.SetBudget(c.ContextBudget)
	}

	if err := o.appendJournalEventLocked(JournalEventRecoveryAdmitted, map[string]any{
		"class":             report.Class,
		"provenance":        report.Provenance,
		"candidate":         report.CandidatePath,
		"preserved_journal": report.PreservedJournal,
	}, report.SnapshotChecksum); err != nil {
		return report, fmt.Errorf("recovery admission of %s: record admission: %w", campaignID, err)
	}
	if err := o.saveCampaign(); err != nil {
		return report, fmt.Errorf("recovery admission of %s: publish admitted generation: %w", campaignID, err)
	}
	logging.Campaign("Campaign %s admitted (%s): %s, %d phases, %d tasks",
		c.ID, report.Provenance, c.Title, len(c.Phases), c.TotalTasks)
	return report, nil
}

// recoveryFactPredicates are the per-campaign inputs of Section 14.
var recoveryFactPredicates = []string{"campaign_recovery_class", "campaign_recovery_adopt"}

// deriveRecoveryAdmissionLocked asserts the observed class and the operator's
// policy and reads the kernel's verdict. Neither verdict deriving means the
// rules are not loaded, which refuses: there is no Go mirror of this decision.
func (o *Orchestrator) deriveRecoveryAdmissionLocked(r *RecoveryReport, policy AdmissionPolicy) (RecoveryProvenance, error) {
	if o.kernel == nil {
		return "", errors.New("recovery admission needs the kernel; none is configured")
	}
	id := r.CampaignID
	for _, pred := range recoveryFactPredicates {
		if err := o.retractCampaignScopedFacts(pred, id); err != nil {
			return "", fmt.Errorf("recovery admission: clear %s: %w", pred, err)
		}
	}
	facts := []core.Fact{{Predicate: "campaign_recovery_class", Args: []any{id, string(r.Class)}}}
	if policy.AdoptSnapshot {
		facts = append(facts, core.Fact{Predicate: "campaign_recovery_adopt", Args: []any{id}})
	}
	if err := o.kernel.LoadFacts(facts); err != nil {
		return "", fmt.Errorf("recovery admission: assert observation: %w", err)
	}

	admitted, err := o.kernel.Query("campaign_recovery_admitted")
	if err != nil {
		return "", fmt.Errorf("recovery admission: query verdict: %w", err)
	}
	if f, ok := factFor(admitted, id); ok && len(f.Args) > 1 {
		return RecoveryProvenance(internaltypes.ExtractString(f.Args[1])), nil
	}
	refused, err := o.kernel.Query("campaign_recovery_refused")
	if err != nil {
		return "", fmt.Errorf("recovery admission: query verdict: %w", err)
	}
	if _, ok := factFor(refused, id); ok {
		return "", &RecoveryRefusedError{Report: r}
	}
	return "", fmt.Errorf("recovery admission rules (campaign_rules.mg Section 14) are not loaded in the kernel; refusing to resume %s", id)
}

// factFor returns the first fact whose first argument is campaignID.
func factFor(facts []core.Fact, campaignID string) (core.Fact, bool) {
	for _, f := range facts {
		if len(f.Args) > 0 && internaltypes.ExtractString(f.Args[0]) == campaignID {
			return f, true
		}
	}
	return core.Fact{}, false
}

// RenderRecoveryReport formats an admission report for a terminal.
func RenderRecoveryReport(r *RecoveryReport) string {
	if r == nil {
		return "no recovery report\n"
	}
	var sb strings.Builder
	verdict := "REFUSED"
	if r.Admitted {
		verdict = "ADMITTED " + string(r.Provenance)
	}
	fmt.Fprintf(&sb, "Campaign recovery: %s  [%s]\n", r.CampaignID, verdict)
	fmt.Fprintf(&sb, "  class          : %s\n", r.Class)
	fmt.Fprintf(&sb, "  evidence       : %s\n", r.Detail)
	fmt.Fprintf(&sb, "  last requested : seq %d %s\n", r.LastRequestedSeq, shortChecksum(r.LastRequestedChecksum))
	fmt.Fprintf(&sb, "  last committed : seq %d %s\n", r.LastCommittedSeq, shortChecksum(r.LastCommittedChecksum))
	if r.CandidatePath != "" {
		fmt.Fprintf(&sb, "  loaded from    : %s %s\n", r.CandidatePath, shortChecksum(r.SnapshotChecksum))
	} else {
		sb.WriteString("  loaded from    : unknown\n")
	}
	if r.PreservedJournal != "" {
		fmt.Fprintf(&sb, "  damaged journal: preserved at %s\n", r.PreservedJournal)
	}
	for _, p := range r.Problems {
		fmt.Fprintf(&sb, "    [%s] line %d %s\n", p.Kind, p.Line, p.Detail)
	}
	if !r.Admitted && r.Class == RecoveryConflict {
		sb.WriteString("  Inspect the snapshot; `nerd campaign resume --adopt-snapshot` admits it as a new provenance boundary.\n")
	}
	return sb.String()
}
