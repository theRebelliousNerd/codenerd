package campaign

import (
	"fmt"
	"strings"
)

// PlanManifest is the bounded plan the advisory board reviews: every task's
// identity, phase, obligation, dependency edges and write set, up to the
// configured bound (campaign.advisory_manifest_max_tasks). Tasks past the
// bound are named in Unavailable, so a review of a partial plan reports that
// it was partial instead of claiming the whole graph.
type PlanManifest struct {
	Tasks       []ManifestTask  `json:"tasks"`
	Phases      []ManifestPhase `json:"phases"`
	Unavailable []string        `json:"unavailable,omitzero"`
	Acceptance  string          `json:"acceptance,omitzero"`
}

// ManifestPhase is a phase and its declared objectives.
type ManifestPhase struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Objectives []string `json:"objectives,omitzero"`
}

// ManifestTask is one task as the advisors see it.
type ManifestTask struct {
	ID          string   `json:"id"`
	PhaseID     string   `json:"phase_id"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on,omitzero"`
	WriteSet    []string `json:"write_set,omitzero"`
	Artifacts   []string `json:"artifacts,omitzero"`
}

// AdvisoryCoverage is what of the plan the panel was given to examine.
// NotAssigned is empty while every advisor is assigned the whole manifest;
// per-advisor obligation assignment is not yet a kernel decision.
type AdvisoryCoverage struct {
	Examined    []string `json:"examined"`
	Unavailable []string `json:"unavailable,omitzero"`
	NotAssigned []string `json:"not_assigned,omitzero"`
}

// buildPlanManifest renders the live campaign into a manifest of at most
// maxTasks tasks, in phase and task order.
func buildPlanManifest(c *Campaign, maxTasks int) PlanManifest {
	var m PlanManifest
	if c == nil {
		return m
	}
	if c.Acceptance != nil && len(c.Acceptance.Command) > 0 {
		m.Acceptance = strings.Join(c.Acceptance.Command, " ")
	}
	for _, phase := range c.Phases {
		mp := ManifestPhase{ID: phase.ID, Name: phase.Name}
		for _, obj := range phase.Objectives {
			if d := strings.TrimSpace(obj.Description); d != "" {
				mp.Objectives = append(mp.Objectives, fmt.Sprintf("%s %s (verify: %s)", obj.Type, d, obj.VerificationMethod))
			}
		}
		m.Phases = append(m.Phases, mp)
		for _, task := range phase.Tasks {
			if len(m.Tasks) >= maxTasks {
				m.Unavailable = append(m.Unavailable, task.ID)
				continue
			}
			mt := ManifestTask{
				ID:          task.ID,
				PhaseID:     phase.ID,
				Type:        string(task.Type),
				Description: task.Description,
				DependsOn:   append([]string(nil), task.DependsOn...),
				WriteSet:    append([]string(nil), task.WriteSet...),
			}
			for _, a := range task.Artifacts {
				mt.Artifacts = append(mt.Artifacts, a.Path)
			}
			m.Tasks = append(m.Tasks, mt)
		}
	}
	return m
}

// Coverage reports the manifest's examined and unavailable task IDs.
func (m PlanManifest) Coverage() AdvisoryCoverage {
	cov := AdvisoryCoverage{
		Examined:    make([]string, 0, len(m.Tasks)),
		Unavailable: append([]string(nil), m.Unavailable...),
	}
	for _, t := range m.Tasks {
		cov.Examined = append(cov.Examined, t.ID)
	}
	return cov
}

// render writes the manifest into the advisory context.
func (m PlanManifest) render(sb *strings.Builder) {
	if len(m.Tasks) == 0 && len(m.Unavailable) == 0 {
		return
	}
	sb.WriteString("### Plan Manifest\n")
	for _, p := range m.Phases {
		if len(p.Objectives) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("Phase %s (%s) objectives:\n", p.ID, p.Name))
		for _, o := range p.Objectives {
			sb.WriteString(fmt.Sprintf("  - %s\n", o))
		}
	}
	if m.Acceptance != "" {
		sb.WriteString(fmt.Sprintf("Acceptance command: `%s`\n", m.Acceptance))
	}
	for _, t := range m.Tasks {
		sb.WriteString(fmt.Sprintf("- %s [%s, phase %s]: %s\n", t.ID, t.Type, t.PhaseID, t.Description))
		if len(t.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("    depends_on: %s\n", strings.Join(t.DependsOn, ", ")))
		}
		if len(t.WriteSet) > 0 {
			sb.WriteString(fmt.Sprintf("    writes: %s\n", strings.Join(t.WriteSet, ", ")))
		}
		if len(t.Artifacts) > 0 {
			sb.WriteString(fmt.Sprintf("    artifacts: %s\n", strings.Join(t.Artifacts, ", ")))
		}
	}
	if len(m.Unavailable) > 0 {
		sb.WriteString(fmt.Sprintf("NOT SHOWN: %d task(s) beyond the manifest bound were not given to this review; your assessment does not cover them.\n",
			len(m.Unavailable)))
	}
	sb.WriteString("\n")
}
