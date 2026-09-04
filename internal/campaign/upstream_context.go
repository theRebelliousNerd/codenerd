package campaign

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/logging"
)

var upstreamTotalBudgetBytes = 48 * 1024
var upstreamPerArtifactCapBytes = 12 * 1024

const verifyHollowFileThresholdBytes = 1024

type upstreamDocEntry struct {
	taskID      string
	description string
	path        string
	content     string
	totalLen    int
	phaseOrder  int
	taskOrder   int
	taskType    TaskType
	phaseID     string
}

type upstreamCandidate struct {
	taskID      string
	description string
	path        string
	phaseOrder  int
	taskOrder   int
	taskType    TaskType
	phaseID     string
}

func (o *Orchestrator) findUpstreamPhase(task *Task) *Phase {
	if o == nil || task == nil || o.campaign == nil {
		return nil
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	for i := range o.campaign.Phases {
		if o.campaign.Phases[i].ID == task.PhaseID {
			cp := o.campaign.Phases[i]
			return &cp
		}
	}
	if task.ID != "" {
		for i := range o.campaign.Phases {
			for _, t := range o.campaign.Phases[i].Tasks {
				if t.ID == task.ID {
					cp := o.campaign.Phases[i]
					return &cp
				}
			}
		}
	}
	return nil
}

func collectDepCandidates(cur *Phase, byID map[string]*Phase) []upstreamCandidate {
	var out []upstreamCandidate
	if cur == nil {
		return out
	}
	for _, dep := range cur.Dependencies {
		if dep.Type != DepHard && dep.Type != DepArtifact {
			continue
		}
		dp, ok := byID[dep.DependsOnPhaseID]
		if !ok {
			continue
		}
		for _, t := range dp.Tasks {
			if t.Status != TaskCompleted {
				continue
			}
			for _, a := range t.Artifacts {
				if a.Type != "/doc" || strings.TrimSpace(a.Path) == "" {
					continue
				}
				out = append(out, upstreamCandidate{
					taskID: t.ID, description: t.Description,
					path: a.Path, phaseOrder: dp.Order,
					taskOrder: t.Order, taskType: t.Type, phaseID: dp.ID,
				})
			}
		}
	}
	return out
}

func collectSamePhaseCandidates(cur *Phase, task *Task) []upstreamCandidate {
	var out []upstreamCandidate
	if cur == nil || task == nil {
		return out
	}
	selfIdx := -1
	for i := range cur.Tasks {
		if cur.Tasks[i].ID == task.ID {
			selfIdx = i
			break
		}
	}
	for i := range cur.Tasks {
		t := cur.Tasks[i]
		if t.ID == task.ID || t.Status != TaskCompleted {
			continue
		}
		earlier := true
		if selfIdx >= 0 {
			earlier = i < selfIdx
		} else if task.Order != 0 || t.Order != 0 {
			earlier = t.Order < task.Order
		}
		if !earlier {
			continue
		}
		for _, a := range t.Artifacts {
			if a.Type != "/doc" || strings.TrimSpace(a.Path) == "" {
				continue
			}
			out = append(out, upstreamCandidate{
				taskID: t.ID, description: t.Description,
				path: a.Path, phaseOrder: cur.Order,
				taskOrder: t.Order, taskType: t.Type, phaseID: cur.ID,
			})
		}
	}
	return out
}

func snapshotPhaseMap(o *Orchestrator) (map[string]*Phase, string) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	m := make(map[string]*Phase, len(o.campaign.Phases))
	for i := range o.campaign.Phases {
		m[o.campaign.Phases[i].ID] = &o.campaign.Phases[i]
	}
	return m, o.workspace
}

func readUpstreamEntries(cands []upstreamCandidate, workspace string) []upstreamDocEntry {
	if len(cands) == 0 {
		return nil
	}
	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].phaseOrder != cands[j].phaseOrder {
			return cands[i].phaseOrder > cands[j].phaseOrder
		}
		if cands[i].taskOrder != cands[j].taskOrder {
			return cands[i].taskOrder < cands[j].taskOrder
		}
		return cands[i].taskID < cands[j].taskID
	})
	seen := make(map[string]struct{}, len(cands))
	out := make([]upstreamDocEntry, 0, len(cands))
	for _, c := range cands {
		key := c.taskID + "\x00" + c.path
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		full := c.path
		if !filepath.IsAbs(full) && workspace != "" {
			full = filepath.Join(workspace, c.path)
		}
		data, err := os.ReadFile(full)
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, upstreamDocEntry{
			taskID: c.taskID, description: c.description,
			path: c.path, content: string(data), totalLen: len(data),
			phaseOrder: c.phaseOrder, taskOrder: c.taskOrder,
			taskType: c.taskType, phaseID: c.phaseID,
		})
	}
	return out
}

func (o *Orchestrator) collectUpstreamDocs(task *Task) []upstreamDocEntry {
	if o == nil || task == nil || o.campaign == nil {
		return nil
	}
	cur := o.findUpstreamPhase(task)
	if cur == nil {
		return nil
	}
	byID, workspace := snapshotPhaseMap(o)
	cands := collectDepCandidates(cur, byID)
	cands = append(cands, collectSamePhaseCandidates(cur, task)...)
	return readUpstreamEntries(cands, workspace)
}

func truncateUpstreamBody(content string, totalLen int, perCap int, path string) string {
	if totalLen <= perCap {
		return content
	}
	head := content
	if len(head) > perCap {
		head = head[:perCap]
	}
	return head + fmt.Sprintf("\n[truncated, %d bytes total; read the file at %s for the rest]", totalLen, path)
}

func (o *Orchestrator) upstreamArtifactContext(task *Task) string {
	if o == nil || task == nil {
		return ""
	}
	taskID := task.ID
	totalBudget := upstreamTotalBudgetBytes
	if totalBudget <= 0 {
		totalBudget = 48 * 1024
	}
	perCap := upstreamPerArtifactCapBytes
	if perCap <= 0 {
		perCap = 12 * 1024
	}
	entries := o.collectUpstreamDocs(task)
	if len(entries) == 0 {
		logging.Campaign("task %s: injected 0 upstream artifacts (0 bytes, budget %d)", taskID, totalBudget)
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Upstream findings (durable artifacts)\n")
	sb.WriteString("The following are durable outputs from upstream tasks. Use them as the evidence base for this task; do not claim there are no findings without addressing them.\n")
	included := 0
	contentBytes := 0
	for _, e := range entries {
		body := truncateUpstreamBody(e.content, e.totalLen, perCap, e.path)
		if contentBytes+len(body) > totalBudget && included > 0 {
			break
		}
		if len(body) > totalBudget {
			body = body[:totalBudget]
		}
		sb.WriteString("\n### ")
		sb.WriteString(e.taskID)
		if strings.TrimSpace(e.description) != "" {
			sb.WriteString(" — ")
			sb.WriteString(strings.TrimSpace(e.description))
		}
		sb.WriteString("\n_Artifact: ")
		sb.WriteString(e.path)
		sb.WriteString("_\n")
		sb.WriteString(body)
		sb.WriteString("\n")
		contentBytes += len(body)
		included++
		if contentBytes >= totalBudget {
			break
		}
	}
	if included == 0 {
		logging.Campaign("task %s: injected 0 upstream artifacts (0 bytes, budget %d)", taskID, totalBudget)
		return ""
	}
	section := sb.String()
	logging.Campaign("task %s: injected %d upstream artifacts (%d bytes, budget %d)", taskID, included, len(section), totalBudget)
	return section
}

func isHollowReportContent(s string) bool {
	l := strings.ToLower(s)
	markers := []string{
		"no findings",
		"nothing to verify",
		"no input content",
		"no content supplied",
		"no findings to rank",
	}
	for _, m := range markers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

func (o *Orchestrator) resolveVerifyReportTarget(task *Task, upstream []upstreamDocEntry) string {
	if o != nil && task != nil {
		if p := o.resolveFileTaskTargetPath(task); strings.TrimSpace(p) != "" {
			return strings.TrimSpace(p)
		}
		if task.Description != "" {
			if p := extractPathFromDescription(task.Description); strings.TrimSpace(p) != "" {
				return strings.TrimSpace(p)
			}
		}
	}
	for _, u := range upstream {
		if u.taskType == TaskTypeDocument {
			return u.path
		}
	}
	if len(upstream) > 0 {
		return upstream[0].path
	}
	return ""
}

func countFindingUpstreams(upstream []upstreamDocEntry) int {
	n := 0
	for _, u := range upstream {
		if isTrivialResult(u.content) || isHollowReportContent(u.content) {
			continue
		}
		n++
	}
	return n
}

func (o *Orchestrator) checkVerifyHollowReport(task *Task) error {
	if o == nil || task == nil || o.campaign == nil {
		return nil
	}
	upstream := o.collectUpstreamDocs(task)
	if len(upstream) == 0 {
		return nil
	}
	findings := countFindingUpstreams(upstream)
	if findings == 0 {
		return nil
	}
	target := o.resolveVerifyReportTarget(task, upstream)
	if strings.TrimSpace(target) == "" {
		return nil
	}
	full := target
	if !filepath.IsAbs(full) && o.workspace != "" {
		full = filepath.Join(o.workspace, target)
	}
	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return nil
	}
	if info.Size() < verifyHollowFileThresholdBytes {
		return fmt.Errorf("verify %s failed: report target %s is hollow (%d bytes) while %d upstream artifacts contain findings; regenerate the report from the upstream evidence instead of completing", task.ID, target, info.Size(), findings)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil
	}
	if isHollowReportContent(string(data)) {
		return fmt.Errorf("verify %s failed: report target %s declares no findings while %d upstream artifacts contain findings; regenerate the report from the upstream evidence instead of completing", task.ID, target, findings)
	}
	return nil
}
