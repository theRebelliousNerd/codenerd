package turnjudge

import (
	"context"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"codenerd/internal/typesafe"
)

// Step technologies: before a planned step edits its file, which of the
// technologies the file imports will the step's change write? The import says
// the file uses a technology; the step's own change text ("STEP <file> ::
// <change>", internal/session/work_steps.go) is the only evidence, before the
// edit, of what the change will touch. One Noul per candidate technology; the
// kernel decides (policy/shard_experts.mg expert_step_declines): a reading at
// or below the no threshold keeps that technology's specialists out of the
// step's pre-edit consultation. The switch is typesafe.step_technologies.

// OpStepTechnologies is the usage tracker's operation for this use.
const OpStepTechnologies = "jev_step_technologies"

// StepTechnologyRequest builds the state and one question per technology,
// named tech_<i> in sorted technology order. imports maps each technology to
// the import paths the file reaches it through: the technology's name alone
// ("mangle") does not tell Jev that ast.Constant or analysis.Analyze are its
// code, the package names do.
func StepTechnologyRequest(file, change string, imports map[string][]string) (map[string]any, map[string]typesafe.Question, []string) {
	techs := make([]string, 0, len(imports))
	for tech := range imports {
		techs = append(techs, tech)
	}
	sort.Strings(techs)
	libraries := make([]map[string]any, 0, len(techs))
	for _, tech := range techs {
		libraries = append(libraries, map[string]any{"technology": tech, "import_paths": imports[tech], "package_names": packageNames(imports[tech])})
	}
	state := map[string]any{
		"file":               file,
		"file_extension":     path.Ext(file),
		"planned_change":     strings.TrimSpace(change),
		"imported_libraries": libraries,
	}
	qs := make(map[string]typesafe.Question, len(techs))
	for i, tech := range techs {
		pkgs := strings.Join(packageNames(imports[tech]), ", ")
		qs[fmt.Sprintf("tech_%d", i)] = typesafe.Noul{
			Instructions: fmt.Sprintf("`file` imports the %s library (packages %s; see `imported_libraries`). Does carrying out `planned_change` on `file` require writing or changing code that uses those packages (their types, functions or values), or writing %s programs, rules or data?", tech, pkgs, tech),
			Criteria: &typesafe.NoulCriteria{
				True:  fmt.Sprintf("The change names, calls, builds or edits something from the %s packages (%s), or writes %s itself, so someone who knows %s should be consulted before the edit.", tech, pkgs, tech, tech),
				False: fmt.Sprintf("The change is to the file's own logic and touches nothing from the %s packages (%s); every existing use of them stays as it is.", tech, pkgs),
			},
		}
	}
	return state, qs, techs
}

// packageNames are the last path elements of import paths, the names code
// refers to them by.
func packageNames(imports []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, imp := range imports {
		name := path.Base(strings.TrimSpace(imp))
		if name != "" && name != "." && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// JudgeStepTechnologies asks the questions and returns each technology's
// probability as a percent (0-100). A failed call is an error, never a "no".
func JudgeStepTechnologies(ctx context.Context, a Asker, file, change string, imports map[string][]string) (map[string]int64, error) {
	if a == nil {
		return nil, fmt.Errorf("turnjudge: no Jev client")
	}
	if strings.TrimSpace(change) == "" || len(imports) == 0 {
		return nil, fmt.Errorf("turnjudge: a step technology reading needs a change and a technology")
	}
	state, qs, techs := StepTechnologyRequest(file, change, imports)
	resp, err := a.Ask(typesafe.WithOperation(ctx, OpStepTechnologies), state, qs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(techs))
	for i, tech := range techs {
		p, err := resp.Noul(fmt.Sprintf("tech_%d", i))
		if err != nil {
			return nil, err
		}
		out[tech] = int64(math.Round(p * 100))
	}
	return out, nil
}

// StepTechnologyReader is a StepReader (internal/experts) over Jev.
type StepTechnologyReader struct {
	Asker Asker
}

// ReadStep implements experts.StepReader.
func (r StepTechnologyReader) ReadStep(ctx context.Context, file, change string, imports map[string][]string) (map[string]int64, error) {
	return JudgeStepTechnologies(ctx, r.Asker, file, change, imports)
}
