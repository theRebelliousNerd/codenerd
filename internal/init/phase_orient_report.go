package init

import (
	"fmt"
	"path/filepath"
	"strings"

	"codenerd/internal/atomicfile"
	"codenerd/internal/northstar"
)

func (i *Initializer) writeOrientationReport(nerdDir string, run orientationRun, result *InitResult) error {
	var b strings.Builder
	b.WriteString("# Orientation\n\nGenerated from repository measurements and Mangle judgments.\n\n")
	fmt.Fprintf(&b, "Measured HEAD: %s\n\n", run.Snapshot.Head)
	b.WriteString("## Timeline\n\n")
	if len(run.Brief.Eras) == 0 {
		b.WriteString(run.Brief.NoEraNote + "\n\n")
	}
	for _, era := range run.Brief.Eras {
		fmt.Fprintf(&b, "- era %d: months %d..%d, %s\n", era.Index, era.StartMonth, era.EndMonth, era.Kind)
	}
	for _, month := range run.Brief.Months {
		fmt.Fprintf(&b, "- %s: %d commits, %d files born, %d documents born\n", month.Label, month.Commits, month.FilesAdded, month.DocsAdded)
	}
	b.WriteString("\n## Origins and vision sources\n\n")
	for _, src := range run.Brief.Origins {
		fmt.Fprintf(&b, "- origin %s: %s\n", src.Path, src.Why)
	}
	for _, src := range run.Brief.Vision {
		fmt.Fprintf(&b, "- vision %s: %d%%, %s\n", src.Path, src.Weight, src.Why)
	}
	b.WriteString("\n## Evolution and supersession\n\n")
	for _, link := range run.Brief.Evolved {
		fmt.Fprintf(&b, "- %s evolved into %s\n", link.Old, link.New)
	}
	for _, link := range run.Brief.Superseded {
		fmt.Fprintf(&b, "- %s superseded by %s\n", link.Old, link.New)
	}
	b.WriteString("\n## Read budget omissions\n\n")
	if len(run.Omitted) == 0 {
		b.WriteString("None.\n")
	}
	for _, row := range run.Omitted {
		b.WriteString("- " + row.String() + "\n")
	}
	b.WriteString("\n## Document claims\n\n")
	if run.Class != nil {
		for _, line := range run.Class.Lines() {
			b.WriteString("- " + line + "\n")
		}
	}
	b.WriteString("\n## North star\n\n")
	if run.Draft == nil {
		b.WriteString("No vision drafted.\n")
	} else {
		fmt.Fprintf(&b, "Mission: %s\n\nProblem: %s\n\nVision: %s\n\n", run.Draft.Document.Mission, run.Draft.Document.Problem, run.Draft.Document.Vision)
		if run.Install.DerivedPath != "" {
			fmt.Fprintf(&b, "Draft recorded at %s; existing vision retained.\n", run.Install.DerivedPath)
		}
	}
	b.WriteString("\n## Measurement and enrichment notes\n\n")
	for _, note := range append(append([]string{}, run.Snapshot.Notes...), run.Notes...) {
		b.WriteString("- " + note + "\n")
	}
	path := filepath.Join(nerdDir, "orientation", "README.md")
	if err := atomicfile.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	result.FilesCreated = append(result.FilesCreated, path)
	return nil
}

// Preserve the strategic/* categories consumed by intelligence and query_strategic.
func strategicKnowledgeFromVision(vision *northstar.Vision, profile ProjectProfile) *StrategicKnowledge {
	knowledge := &StrategicKnowledge{ArchitectureStyle: profile.Architecture, DesignPrinciples: append([]string{}, profile.Patterns...)}
	if vision == nil {
		knowledge.Limitations = []string{"No north star could be derived from the orientation evidence."}
		return knowledge
	}
	knowledge.ProjectVision = vision.VisionStmt
	knowledge.CorePhilosophy = vision.Mission
	knowledge.SafetyConstraints = append([]string{}, vision.Constraints...)
	for _, capability := range vision.Capabilities {
		knowledge.CoreCapabilities = append(knowledge.CoreCapabilities, capability.Description)
	}
	for _, risk := range vision.Risks {
		knowledge.Limitations = append(knowledge.Limitations, risk.Description)
	}
	for _, requirement := range vision.Requirements {
		knowledge.FutureDirections = append(knowledge.FutureDirections, requirement.Description)
	}
	return knowledge
}
