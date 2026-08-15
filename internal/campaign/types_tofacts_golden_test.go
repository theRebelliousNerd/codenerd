package campaign

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

// ToFacts is the transduction boundary between the Go plan and the Mangle
// executive. Every rule that schedules, blocks or verifies a campaign matches
// on the shape of these facts, and Mangle matches positionally: an argument
// inserted, removed or reordered does not fail loudly, it just stops matching.
// The rule keeps evaluating, derives nothing, and the campaign quietly loses a
// capability — a soft dependency that no longer orders anything, a write target
// that no longer gates anything.
//
// These tests pin the emitted shape three ways:
//
//  1. the exact predicate/arity/type set, against a golden file;
//  2. every emitted predicate against its Decl in internal/core/defaults, so
//     Go and the kernel schema cannot drift apart;
//  3. that the fixture below still exercises every emitting branch, so the
//     golden cannot silently shrink.
//
// Regenerate with: go test ./internal/campaign/ -run ToFacts -update-golden

var updateGolden = flag.Bool("update-golden", false, "rewrite the ToFacts golden file from the current fixture")

const toFactsGoldenPath = "testdata/tofacts_predicates.golden"

// goldenToFactsCampaign is a campaign that exercises EVERY conditional emit in
// Campaign.ToFacts, Phase.ToFacts, Task.ToFacts and ContextProfile.ToFacts.
// Optional branches (sub-campaign, inference, attempts, retry window, error,
// compression, artifacts, soft deps, resources) are all populated on purpose.
func goldenToFactsCampaign() *Campaign {
	created := time.Unix(1_700_000_000, 0).UTC()
	return &Campaign{
		ID:              "/campaign_golden",
		Type:            CampaignTypeFeature,
		Title:           "Golden fixture",
		Goal:            "Pin the ToFacts contract",
		SourceMaterial:  []string{"docs/spec.md"},
		Status:          StatusActive,
		CreatedAt:       created,
		Confidence:      0.85,
		CompletedPhases: 1,
		TotalPhases:     2,
		CompletedTasks:  1,
		TotalTasks:      3,
		SourceDocs: []SourceDocument{{
			CampaignID: "/campaign_golden",
			Path:       "docs/spec.md",
			Type:       "/spec",
			ParsedAt:   created,
		}},
		ContextProfiles: []ContextProfile{{
			ID:              "/profile_golden",
			RequiredSchemas: []string{"file_topology"},
			RequiredTools:   []string{"exec_cmd"},
			FocusPatterns:   []string{"internal/**"},
		}},
		Phases: []Phase{{
			ID:             "/phase_golden_0",
			CampaignID:     "/campaign_golden",
			Name:           "Golden phase",
			Order:          0,
			Category:       "/implementation",
			Status:         PhaseCompleted,
			ContextProfile: "/profile_golden",
			Objectives: []PhaseObjective{{
				Type:               ObjectiveCreate,
				Description:        "do the thing",
				VerificationMethod: VerifyTestsPass,
			}},
			Dependencies: []PhaseDependency{{
				DependsOnPhaseID: "/phase_golden_seed",
				Type:             DepHard,
			}},
			EstimatedTasks:      3,
			EstimatedComplexity: "/high",
			CompressedSummary:   "phase summary",
			OriginalAtomCount:   7,
			CompressedAt:        created,
			Tasks: []Task{{
				ID:              "/task_golden_0",
				PhaseID:         "/phase_golden_0",
				Description:     "write a file",
				Status:          TaskFailed,
				Type:            TaskTypeFileCreate,
				Priority:        PriorityHigh,
				Order:           1,
				DependsOn:       []string{"/task_golden_seed"},
				SoftDeps:        []string{"/task_golden_soft"},
				Resources:       []string{"build_lock"},
				SubCampaignID:   "/campaign_golden_child",
				WriteSet:        []string{"internal/x/y.go"},
				InferredFrom:    "/requirement_1",
				InferenceConf:   0.8,
				InferenceReason: "requirement mentions y.go",
				LastError:       "boom",
				NextRetryAt:     created.Add(time.Minute),
				Artifacts: []TaskArtifact{{
					Type: "/file",
					Path: "internal/x/y.go",
					Hash: "deadbeef",
				}},
				Attempts: []TaskAttempt{{
					Number:    1,
					Outcome:   "/failed",
					Timestamp: created,
				}},
			}},
		}},
	}
}

type factShape struct {
	predicate string
	arity     int
	kinds     []string
}

func (f factShape) line() string {
	return fmt.Sprintf("%s/%d %s", f.predicate, f.arity, strings.Join(f.kinds, ","))
}

func shapeOf(facts []core.Fact) []factShape {
	seen := make(map[string]factShape)
	for _, fact := range facts {
		kinds := make([]string, 0, len(fact.Args))
		for _, arg := range fact.Args {
			kinds = append(kinds, argKind(arg))
		}
		shape := factShape{predicate: fact.Predicate, arity: len(fact.Args), kinds: kinds}
		key := shape.line()
		seen[key] = shape
	}
	out := make([]factShape, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].line() < out[j].line() })
	return out
}

// argKind reduces a fact argument to the category Mangle cares about.
//
// The kernel decides atom-vs-string with its own heuristic
// (core.shouldBeName: leading slash, few segments, no file extension). This is
// a coarser approximation on purpose — the point is not to predict the kernel's
// choice but to notice when an argument's Go type changes underneath a slot,
// e.g. a confidence emitted as float64 into a slot declared /number.
func argKind(arg any) string {
	switch v := arg.(type) {
	case string:
		if strings.HasPrefix(v, "/") {
			return "atom"
		}
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	default:
		return fmt.Sprintf("unknown(%T)", arg)
	}
}

func TestToFacts_EmittedPredicateArity_ShouldMatchGolden(t *testing.T) {
	shapes := shapeOf(goldenToFactsCampaign().ToFacts())

	lines := make([]string, 0, len(shapes))
	for _, s := range shapes {
		lines = append(lines, s.line())
	}
	got := strings.Join(lines, "\n") + "\n"

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(toFactsGoldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(toFactsGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden updated: %s", toFactsGoldenPath)
		return
	}

	want, err := os.ReadFile(toFactsGoldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with -update-golden)", toFactsGoldenPath, err)
	}

	if got != string(want) {
		t.Errorf("ToFacts emitted a different predicate/arity/type set.\n\n"+
			"Mangle matches positionally, so a changed arity or argument type stops rules from firing "+
			"instead of failing. If this change is intended, update every rule in "+
			"internal/core/defaults that matches the affected predicate and its Decl, then regenerate "+
			"with -update-golden.\n\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

var declPattern = regexp.MustCompile(`^Decl\s+([a-z_][a-zA-Z0-9_]*)\s*\(([^)]*)\)`)

// TestToFacts_EveryEmittedPredicate_ShouldHaveMatchingDecl walks the Mangle
// corpus and checks the declared arity against what Go actually emits.
//
// This found three predicates that Go had been asserting with no Decl at all —
// task_soft_dependency, requires_resource and task_sub_campaign — which meant no
// rule could reference them and the executive could not see soft dependencies,
// resource semaphores or sub-campaign links.
func TestToFacts_EveryEmittedPredicate_ShouldHaveMatchingDecl(t *testing.T) {
	decls := loadMangleDecls(t)
	shapes := shapeOf(goldenToFactsCampaign().ToFacts())

	for _, shape := range shapes {
		declared, ok := decls[shape.predicate]
		if !ok {
			t.Errorf("ToFacts emits %s/%d but no Decl exists under internal/core/defaults. "+
				"Facts with no Decl cannot be referenced by any rule, so the kernel is blind to them.",
				shape.predicate, shape.arity)
			continue
		}
		if declared != shape.arity {
			t.Errorf("arity drift: ToFacts emits %s/%d, Decl says %s/%d. "+
				"Mangle matches positionally; every rule over %s is now dead.",
				shape.predicate, shape.arity, shape.predicate, declared, shape.predicate)
		}
	}
}

func loadMangleDecls(t *testing.T) map[string]int {
	t.Helper()
	root := filepath.Join(repoRoot(t), "internal", "core", "defaults")

	decls := make(map[string]int)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".mg") {
			return nil
		}
		f, oerr := os.Open(path)
		if oerr != nil {
			return oerr
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			m := declPattern.FindStringSubmatch(strings.TrimSpace(scanner.Text()))
			if m == nil {
				continue
			}
			args := strings.TrimSpace(m[2])
			arity := 0
			if args != "" {
				arity = len(strings.Split(args, ","))
			}
			decls[m[1]] = arity
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatalf("scanning mangle corpus: %v", err)
	}
	if len(decls) < 100 {
		t.Fatalf("found only %d Decls; the corpus scan is broken, not the schema", len(decls))
	}
	return decls
}

// TestToFacts_GoldenFixture_ShouldExerciseEveryEmitBranch keeps the golden
// honest. Without it, deleting an emit and deleting its golden line together
// would look like a clean, passing change.
func TestToFacts_GoldenFixture_ShouldExerciseEveryEmitBranch(t *testing.T) {
	// Every predicate Task/Phase/Campaign/ContextProfile ToFacts can emit.
	required := []string{
		"campaign", "campaign_metadata", "campaign_goal", "campaign_progress",
		"context_profile", "source_document",
		"campaign_phase", "phase_category", "phase_objective", "phase_dependency",
		"phase_estimate", "context_compression",
		"campaign_task", "task_priority", "task_order", "task_dependency",
		"task_soft_dependency", "requires_resource", "task_sub_campaign",
		"task_artifact", "task_inference", "task_attempt", "task_retry_at",
		"task_error", "task_write_target",
	}

	emitted := make(map[string]bool)
	for _, f := range goldenToFactsCampaign().ToFacts() {
		emitted[f.Predicate] = true
	}

	for _, pred := range required {
		if !emitted[pred] {
			t.Errorf("fixture no longer emits %s; the golden would silently stop covering it", pred)
		}
	}

	// The reverse direction: an emit the fixture covers but this list forgot.
	known := make(map[string]bool, len(required))
	for _, pred := range required {
		known[pred] = true
	}
	for pred := range emitted {
		if !known[pred] {
			t.Errorf("ToFacts emits %s, which this coverage list does not name. "+
				"Add it here and to the golden so the new fact has a pinned shape.", pred)
		}
	}
}
