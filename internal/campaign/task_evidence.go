package campaign

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/observation"
	toolscore "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// A task's upstream evidence: what the work before it produced, handed to it
// in its brief. Which artifacts, and in what form, is the kernel's decision
// (policy/campaign_evidence.mg, task_evidence/3); this file measures, asks and
// renders. It replaced upstream_context.go (2026-09-23), where Go chose every
// /doc artifact of the transitive upstream phases by phase order and cut each
// at 12 KiB and all of them at 48 KiB.

// The forms task_evidence hands an artifact in.
const (
	evidenceInline = "/inline"
	evidenceDigest = "/digest"
	evidenceHandle = "/handle"
)

// evidenceTask is the part of a task the evidence measurement reads, copied
// out of the live campaign under its lock.
type evidenceTask struct {
	id, phaseID, description, shardInput string
	order, phaseOrder                    int
	status                               TaskStatus
	artifacts                            []TaskArtifact
	writeSet                             []string
}

// measuredArtifact is a declared artifact found on disk as a regular file.
type measuredArtifact struct {
	producer string
	path     string // as task_artifact carries it (normalizePath)
	artType  string
	ext      string
	bytes    int64
	full     string
	info     os.FileInfo
}

// evidenceRow is one task_evidence answer, with what rendering it needs.
type evidenceRow struct {
	path      string
	mode      string
	producers []evidenceTask
}

// snapshotEvidenceTasks copies every task of the live campaign, with its
// phase's order, and the workspace.
func (o *Orchestrator) snapshotEvidenceTasks() ([]evidenceTask, string) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if o.campaign == nil {
		return nil, o.workspace
	}
	var out []evidenceTask
	for i := range o.campaign.Phases {
		ph := &o.campaign.Phases[i]
		for j := range ph.Tasks {
			t := &ph.Tasks[j]
			out = append(out, evidenceTask{
				id: t.ID, phaseID: t.PhaseID, description: t.Description, shardInput: t.ShardInput,
				order: t.Order, phaseOrder: ph.Order, status: t.Status,
				artifacts: append([]TaskArtifact(nil), t.Artifacts...),
				writeSet:  append([]string(nil), t.WriteSet...),
			})
		}
	}
	return out, o.workspace
}

// resolveWorkspacePath makes a declared path absolute against the workspace.
func resolveWorkspacePath(workspace, p string) string {
	host := filepath.FromSlash(strings.TrimSpace(p))
	if filepath.IsAbs(host) || workspace == "" {
		return filepath.Clean(host)
	}
	return filepath.Join(workspace, host)
}

// measureArtifacts stats every declared artifact of every task: the ones that
// are regular files are what the policy can hand over.
func measureArtifacts(tasks []evidenceTask, workspace string) []measuredArtifact {
	var out []measuredArtifact
	seen := map[string]bool{}
	for _, t := range tasks {
		for _, a := range t.artifacts {
			if strings.TrimSpace(a.Path) == "" {
				continue
			}
			path := normalizePath(a.Path)
			key := t.id + "\x00" + path
			if seen[key] {
				continue
			}
			seen[key] = true
			full := resolveWorkspacePath(workspace, a.Path)
			info, err := os.Stat(full)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			out = append(out, measuredArtifact{
				producer: t.id, path: path, artType: a.Type,
				ext:   strings.ToLower(filepath.Ext(path)),
				bytes: info.Size(), full: full, info: info,
			})
		}
	}
	return out
}

// briefNames reports whether a brief names a path: the workspace-relative path
// itself, or its file name standing as a word of its own. Case does not matter.
func briefNames(brief, path string) bool {
	lower := strings.ToLower(filepath.ToSlash(brief))
	p := strings.ToLower(filepath.ToSlash(path))
	if p == "" {
		return false
	}
	if strings.Contains(lower, p) {
		return true
	}
	base := filepath.Base(filepath.FromSlash(p))
	base = strings.ToLower(filepath.ToSlash(base))
	if base == "" || base == "." || !strings.Contains(base, ".") {
		return false
	}
	isNameByte := func(b byte) bool {
		return b == '_' || b == '-' || b == '.' || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
	}
	for from := 0; from < len(lower); {
		i := strings.Index(lower[from:], base)
		if i < 0 {
			return false
		}
		start := from + i
		end := start + len(base)
		before := start == 0 || !isNameByte(lower[start-1])
		after := end == len(lower) || !(isNameByte(lower[end]) && lower[end] != '.')
		if before && after {
			return true
		}
		from = start + 1
	}
	return false
}

// outputFiles stats the asked task's declared outputs -- its artifacts and its
// write set -- so a measured artifact that is the same file is known to be the
// task's own output, whatever spelling the plan used for it.
func outputFiles(t evidenceTask, workspace string) []os.FileInfo {
	var out []os.FileInfo
	add := func(p string) {
		if strings.TrimSpace(p) == "" || containsGlobMeta(p) {
			return
		}
		if info, err := os.Stat(resolveWorkspacePath(workspace, p)); err == nil && info.Mode().IsRegular() {
			out = append(out, info)
		}
	}
	for _, a := range t.artifacts {
		add(a.Path)
	}
	for _, w := range t.writeSet {
		add(w)
	}
	return out
}

// evidenceFacts is what Go measured for the asked task.
func evidenceFacts(asked evidenceTask, arts []measuredArtifact, workspace string) (onDisk, named, own []core.Fact) {
	brief := asked.description + "\n" + asked.shardInput
	outputs := outputFiles(asked, workspace)
	namedSeen := map[string]bool{}
	ownSeen := map[string]bool{}
	for _, a := range arts {
		onDisk = append(onDisk, core.Fact{
			Predicate: "task_artifact_on_disk",
			Args:      []any{a.producer, a.path, types.MangleAtom(a.artType), a.ext, a.bytes},
		})
		if a.producer != asked.id && !namedSeen[a.path] && briefNames(brief, a.path) {
			namedSeen[a.path] = true
			named = append(named, core.Fact{Predicate: "task_brief_names", Args: []any{asked.id, a.path}})
		}
		if ownSeen[a.path] {
			continue
		}
		mine := a.producer == asked.id
		for _, info := range outputs {
			if mine || os.SameFile(info, a.info) {
				mine = true
				break
			}
		}
		if mine {
			ownSeen[a.path] = true
			own = append(own, core.Fact{Predicate: "task_output_path", Args: []any{asked.id, a.path}})
		}
	}
	return onDisk, named, own
}

// assertEvidenceMeasurements replaces what the kernel holds of the campaign's
// artifacts on disk, and of what the asked task's brief names and writes.
func (o *Orchestrator) assertEvidenceMeasurements(askedID string, onDisk, named, own []core.Fact) error {
	if _, ok := o.kernel.(types.KernelTransactor); ok {
		tx := types.NewKernelTx(o.kernel)
		tx.Retract("task_artifact_on_disk")
		tx.RetractFact(core.Fact{Predicate: "task_brief_names", Args: []any{askedID}})
		tx.RetractFact(core.Fact{Predicate: "task_output_path", Args: []any{askedID}})
		tx.LoadFacts(onDisk)
		tx.LoadFacts(named)
		tx.LoadFacts(own)
		return tx.Commit()
	}
	o.mu.RLock()
	var ids []string
	if o.campaign != nil {
		for i := range o.campaign.Phases {
			for j := range o.campaign.Phases[i].Tasks {
				ids = append(ids, o.campaign.Phases[i].Tasks[j].ID)
			}
		}
	}
	o.mu.RUnlock()
	for _, id := range ids {
		if err := o.kernel.RetractFact(core.Fact{Predicate: "task_artifact_on_disk", Args: []any{id}}); err != nil {
			return err
		}
	}
	for _, pred := range []string{"task_brief_names", "task_output_path"} {
		if err := o.kernel.RetractFact(core.Fact{Predicate: pred, Args: []any{askedID}}); err != nil {
			return err
		}
	}
	return o.kernel.LoadFacts(append(append(append([]core.Fact(nil), onDisk...), named...), own...))
}

// measureTaskEvidence asserts what the evidence policy decides from, for the
// asked task. It holds evidenceMu so concurrent tasks never ask between another
// task's retraction and assertion. The returned tasks and artifacts are the
// snapshot the measurement was taken from.
func (o *Orchestrator) measureTaskEvidence(task *Task) ([]evidenceTask, []measuredArtifact, bool, error) {
	tasks, workspace := o.snapshotEvidenceTasks()
	var asked *evidenceTask
	for i := range tasks {
		if tasks[i].id == task.ID {
			asked = &tasks[i]
			break
		}
	}
	if asked == nil {
		// Not a task of this campaign (or no campaign): nothing came before it.
		return nil, nil, false, nil
	}
	if o.kernel == nil {
		return nil, nil, false, fmt.Errorf("no kernel to derive the upstream evidence of %s", task.ID)
	}
	if err := o.ensureTaskRows(task); err != nil {
		return nil, nil, false, err
	}
	arts := measureArtifacts(tasks, workspace)
	onDisk, named, own := evidenceFacts(*asked, arts, workspace)
	if err := o.assertEvidenceMeasurements(asked.id, onDisk, named, own); err != nil {
		return nil, nil, false, fmt.Errorf("assert the evidence measurements of %s: %w", task.ID, err)
	}
	return tasks, arts, true, nil
}

// askTaskEvidence returns the kernel's task_evidence rows for the task.
func (o *Orchestrator) askTaskEvidence(taskID string) (map[string]string, error) {
	facts, err := o.kernel.Query("task_evidence")
	if err != nil {
		return nil, fmt.Errorf("query task_evidence: %w", err)
	}
	modes := map[string]string{}
	for _, f := range facts {
		if factArg(f, 0) != taskID {
			continue
		}
		path, mode := factArg(f, 1), factArg(f, 2)
		if path == "" || mode == "" {
			return nil, fmt.Errorf("the kernel derived a malformed task_evidence row %v", f.Args)
		}
		if prev, ok := modes[path]; ok && prev != mode {
			return nil, fmt.Errorf("the kernel derived %s and %s for %s of %s; the policy must decide one", prev, mode, path, taskID)
		}
		modes[path] = mode
	}
	return modes, nil
}

// taskEvidenceSection is the brief's upstream evidence for the task: the
// artifacts the kernel selected, inline whole, digested with a recall handle,
// or as a handle alone. Empty when nothing came before the task.
func (o *Orchestrator) taskEvidenceSection(task *Task) (string, error) {
	if o == nil || task == nil {
		return "", nil
	}
	o.evidenceMu.Lock()
	defer o.evidenceMu.Unlock()
	tasks, arts, ok, err := o.measureTaskEvidence(task)
	if err != nil || !ok {
		return "", err
	}
	modes, err := o.askTaskEvidence(task.ID)
	if err != nil {
		return "", err
	}
	rows := evidenceRows(modes, tasks, arts)
	section, counts := renderEvidence(rows, arts)
	logging.Campaign("task %s: upstream evidence %d inline, %d digest, %d handle of %d artifacts on disk (%d bytes)",
		task.ID, counts[evidenceInline], counts[evidenceDigest], counts[evidenceHandle], len(arts), len(section))
	return section, nil
}

// evidenceRows joins the kernel's answer with the completed tasks that
// produced each path, in the order the section presents them: inline, then
// digests, then handles; within each, the newest phase first.
func evidenceRows(modes map[string]string, tasks []evidenceTask, arts []measuredArtifact) []evidenceRow {
	byID := make(map[string]evidenceTask, len(tasks))
	for _, t := range tasks {
		byID[t.id] = t
	}
	var rows []evidenceRow
	for path, mode := range modes {
		row := evidenceRow{path: path, mode: mode}
		seen := map[string]bool{}
		for _, a := range arts {
			if a.path != path || seen[a.producer] {
				continue
			}
			if t, ok := byID[a.producer]; ok && t.status == TaskCompleted {
				seen[a.producer] = true
				row.producers = append(row.producers, t)
			}
		}
		sort.SliceStable(row.producers, func(i, j int) bool {
			a, b := row.producers[i], row.producers[j]
			if a.phaseOrder != b.phaseOrder {
				return a.phaseOrder > b.phaseOrder
			}
			if a.order != b.order {
				return a.order < b.order
			}
			return a.id < b.id
		})
		rows = append(rows, row)
	}
	rank := map[string]int{evidenceInline: 0, evidenceDigest: 1, evidenceHandle: 2}
	lead := func(r evidenceRow) evidenceTask {
		if len(r.producers) > 0 {
			return r.producers[0]
		}
		return evidenceTask{}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if rank[a.mode] != rank[b.mode] {
			return rank[a.mode] < rank[b.mode]
		}
		la, lb := lead(a), lead(b)
		if la.phaseOrder != lb.phaseOrder {
			return la.phaseOrder > lb.phaseOrder
		}
		if la.order != lb.order {
			return la.order < lb.order
		}
		return a.path < b.path
	})
	return rows
}

// renderEvidence writes the section. A file that vanished between the
// measurement and now is named as missing, never silently dropped.
func renderEvidence(rows []evidenceRow, arts []measuredArtifact) (string, map[string]int) {
	counts := map[string]int{}
	if len(rows) == 0 {
		return "", counts
	}
	fullOf := map[string]string{}
	for _, a := range arts {
		if _, ok := fullOf[a.path]; !ok {
			fullOf[a.path] = a.full
		}
	}
	var sb strings.Builder
	sb.WriteString("## Upstream findings (durable artifacts)\n")
	sb.WriteString("The following are durable outputs from upstream tasks. Use them as the evidence base for this task; do not claim there are no findings without addressing them.\n")
	handles := 0
	for _, r := range rows {
		data, err := os.ReadFile(fullOf[r.path])
		if err != nil {
			fmt.Fprintf(&sb, "\n### %s\n_Artifact: %s (missing on disk: %v)_\n", producerIDs(r), r.path, err)
			continue
		}
		counts[r.mode]++
		switch r.mode {
		case evidenceInline:
			fmt.Fprintf(&sb, "\n### %s\n_Artifact: %s_\n%s\n", producerHeading(r), r.path, string(data))
		case evidenceDigest:
			fmt.Fprintf(&sb, "\n### %s\n_Artifact: %s (digest)_\n%s", producerHeading(r), r.path, evidenceProjection(r, data).Text(toolscore.SubagentExpandToolName))
		case evidenceHandle:
			if handles == 0 {
				sb.WriteString("\n### Further upstream\n")
			}
			handles++
			proj := evidenceProjection(r, data)
			if proj.Handle == "" {
				// Too small to retain: the projection carries it whole.
				fmt.Fprintf(&sb, "- %s %s (%d bytes):\n%s", producerIDs(r), r.path, len(data), proj.Text(toolscore.SubagentExpandToolName))
				continue
			}
			fmt.Fprintf(&sb, "- %s %s (%d bytes): %s handle=%s\n", producerIDs(r), r.path, len(data), toolscore.SubagentExpandToolName, proj.Handle)
		}
	}
	return sb.String(), counts
}

// evidenceProjection projects an artifact through the subagent-return codec,
// which retains the whole text behind an obs:sa: handle that recall_context and
// subagent_expand redeem. A handle is minted by content, so the same artifact
// is the same handle in every brief; a retained copy that has already expired
// is minted again, so the handle printed is live when it is printed.
func evidenceProjection(r evidenceRow, data []byte) observation.ReturnResult {
	// The heading above the projection names the producer's task; the codec
	// is not told it again.
	ret := observation.Return{Output: string(data), Changed: []string{r.path}}
	if len(r.producers) > 0 {
		ret.Agent = strings.TrimPrefix(r.producers[0].id, "/")
	}
	codec := observation.SharedSubagents()
	proj := codec.EncodeReturn(ret, observation.ReturnLimits{})
	if proj.Handle != "" {
		if _, err := codec.HydrateReturn(proj.Handle, observation.ReturnWindow{Limit: 1}); errors.Is(err, observation.ErrNotFound) {
			proj = codec.EncodeReturn(ret, observation.ReturnLimits{})
		}
	}
	return proj
}

// factArg is a derived row's argument i as a string, or "" when the row is
// shorter than that.
func factArg(f core.Fact, i int) string {
	if i < len(f.Args) {
		return types.ExtractString(f.Args[i])
	}
	return ""
}

func producerIDs(r evidenceRow) string {
	if len(r.producers) == 0 {
		return "(no completed producer)"
	}
	ids := make([]string, 0, len(r.producers))
	for _, p := range r.producers {
		ids = append(ids, p.id)
	}
	return strings.Join(ids, ", ")
}

func producerHeading(r evidenceRow) string {
	h := producerIDs(r)
	if len(r.producers) > 0 {
		if d := strings.TrimSpace(r.producers[0].description); d != "" {
			h += " — " + d
		}
	}
	return h
}

// checkVerifyHollowReport fails a /verify task whose report the kernel judges
// hollow (verify_report_hollow): a deliverable of a task it depends on is under
// campaign.verify_report_min_bytes while that task was owed upstream evidence.
func (o *Orchestrator) checkVerifyHollowReport(task *Task) error {
	if o == nil || task == nil {
		return nil
	}
	o.evidenceMu.Lock()
	defer o.evidenceMu.Unlock()
	if _, _, ok, err := o.measureTaskEvidence(task); err != nil || !ok {
		return err
	}
	facts, err := o.kernel.Query("verify_report_hollow")
	if err != nil {
		return fmt.Errorf("query verify_report_hollow: %w", err)
	}
	var hollow []string
	for _, f := range facts {
		if factArg(f, 0) != task.ID {
			continue
		}
		hollow = append(hollow, fmt.Sprintf("%s is %s bytes while its producer was owed %s upstream artifacts",
			factArg(f, 1), factArg(f, 2), factArg(f, 3)))
	}
	if len(hollow) == 0 {
		return nil
	}
	sort.Strings(hollow)
	return fmt.Errorf("verify %s failed: the report it checks is hollow: %s; regenerate the report from the upstream evidence instead of completing",
		task.ID, strings.Join(hollow, "; "))
}
