package projectdoc

import (
	"testing"

	"codenerd/internal/types"
)

func TestNorthstarFacts_EmitsModuleNorthstarAndRequirements(t *testing.T) {
	doc := &Document{
		Path: "internal/auth/nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			Northstar: &Northstar{
				Purpose: "Build a logic-first agent",
				Requirements: []NorthstarRequirement{
					{ID: "REQ-1", Statement: "The kernel must derive next_action", Severity: "blocker"},
					{ID: "REQ-2", Statement: "Every action must be permitted", Severity: "major"},
				},
			},
		},
	}
	facts := doc.Facts()
	byPred := map[string][]types.Fact{}
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}

	nsFacts := byPred[PredModuleNorthstar]
	if len(nsFacts) != 1 {
		t.Fatalf("module_northstar facts = %d, want 1; got %+v", len(nsFacts), nsFacts)
	}
	if got := nsFacts[0].Args[0]; got != "internal/auth" {
		t.Errorf("module_northstar ModulePath = %q, want %q", got, "internal/auth")
	}
	if got := nsFacts[0].Args[1]; got != "Build a logic-first agent" {
		t.Errorf("module_northstar Purpose = %q, want %q", got, "Build a logic-first agent")
	}
	// severity and module path atoms must be convertible
	if _, err := nsFacts[0].ToAtom(); err != nil {
		t.Errorf("module_northstar fact does not convert to atom: %v", err)
	}

	reqFacts := byPred[PredModuleRequirement]
	if len(reqFacts) != 2 {
		t.Fatalf("module_requirement facts = %d, want 2; got %+v", len(reqFacts), reqFacts)
	}
	// Check first requirement
	if got := reqFacts[0].Args[0]; got != "internal/auth" {
		t.Errorf("module_requirement[0] ModulePath = %q, want %q", got, "internal/auth")
	}
	if got := reqFacts[0].Args[1]; got != "REQ-1" {
		t.Errorf("module_requirement[0] ID = %q, want %q", got, "REQ-1")
	}
	if got := reqFacts[0].Args[2]; got != "The kernel must derive next_action" {
		t.Errorf("module_requirement[0] Statement = %q, want %q", got, "The kernel must derive next_action")
	}
	if atom, ok := reqFacts[0].Args[3].(types.MangleAtom); !ok {
		t.Fatalf("module_requirement[0] Severity is %T, want types.MangleAtom", reqFacts[0].Args[3])
	} else if string(atom) != "/blocker" {
		t.Errorf("module_requirement[0] Severity = %q, want %q", atom, "/blocker")
	}
	if _, err := reqFacts[0].ToAtom(); err != nil {
		t.Errorf("module_requirement[0] fact does not convert to atom: %v", err)
	}
	// Check second requirement severity /major
	if atom, ok := reqFacts[1].Args[3].(types.MangleAtom); !ok {
		t.Fatalf("module_requirement[1] Severity is %T, want types.MangleAtom", reqFacts[1].Args[3])
	} else if string(atom) != "/major" {
		t.Errorf("module_requirement[1] Severity = %q, want %q", atom, "/major")
	}
}

func TestNorthstarFacts_RootModulePathUsesDot(t *testing.T) {
	doc := &Document{
		Path: "nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			Northstar: &Northstar{
				Purpose: "Root module purpose",
				Requirements: []NorthstarRequirement{
					{ID: "REQ-1", Statement: "Root requirement", Severity: "blocker"},
				},
			},
		},
	}
	facts := doc.Facts()
	byPred := map[string][]types.Fact{}
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}
	nsFacts := byPred[PredModuleNorthstar]
	if len(nsFacts) != 1 {
		t.Fatalf("module_northstar facts = %d, want 1", len(nsFacts))
	}
	if got := nsFacts[0].Args[0]; got != "." {
		t.Errorf("root ModulePath = %q, want %q", got, ".")
	}
	reqFacts := byPred[PredModuleRequirement]
	if len(reqFacts) != 1 {
		t.Fatalf("module_requirement facts = %d, want 1", len(reqFacts))
	}
	if got := reqFacts[0].Args[0]; got != "." {
		t.Errorf("root requirement ModulePath = %q, want %q", got, ".")
	}
}

func TestNorthstarFacts_SubdirectoryUsesRelativePOSIX(t *testing.T) {
	doc := &Document{
		Path: "internal/projectdoc/nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			Northstar: &Northstar{
				Purpose: "Projectdoc purpose",
				Requirements: []NorthstarRequirement{
					{ID: "REQ-42", Statement: "Something required", Severity: "minor"},
				},
			},
		},
	}
	facts := doc.Facts()
	byPred := map[string][]types.Fact{}
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}
	nsFacts := byPred[PredModuleNorthstar]
	if len(nsFacts) != 1 {
		t.Fatalf("module_northstar facts = %d, want 1", len(nsFacts))
	}
	if got := nsFacts[0].Args[0]; got != "internal/projectdoc" {
		t.Errorf("subdirectory ModulePath = %q, want %q", got, "internal/projectdoc")
	}
	reqFacts := byPred[PredModuleRequirement]
	if len(reqFacts) != 1 {
		t.Fatalf("module_requirement facts = %d, want 1", len(reqFacts))
	}
	if got := reqFacts[0].Args[0]; got != "internal/projectdoc" {
		t.Errorf("subdirectory requirement ModulePath = %q, want %q", got, "internal/projectdoc")
	}
}

func TestNorthstarFacts_NilNorthstarEmitsNeither(t *testing.T) {
	doc := &Document{
		Path: "nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			// Northstar is nil
		},
	}
	facts := doc.Facts()
	for _, f := range facts {
		if f.Predicate == PredModuleNorthstar || f.Predicate == PredModuleRequirement {
			t.Errorf("nil Northstar should emit no %s fact, got %+v", f.Predicate, f)
		}
	}
}

func TestNorthstarFacts_EmptyPurposeEmitsNoNorthstarButRequirements(t *testing.T) {
	cases := []string{"", "   ", "\t\n "}
	for _, purpose := range cases {
		doc := &Document{
			Path: "internal/foo/nerd.md",
			Spec: Spec{
				Schema: SchemaVersion,
				Northstar: &Northstar{
					Purpose: purpose,
					Requirements: []NorthstarRequirement{
						{ID: "REQ-1", Statement: "Requirement still emitted", Severity: "blocker"},
						{ID: "REQ-2", Statement: "Second requirement", Severity: "major"},
					},
				},
			},
		}
		facts := doc.Facts()
		byPred := map[string][]types.Fact{}
		for _, f := range facts {
			byPred[f.Predicate] = append(byPred[f.Predicate], f)
		}
		if len(byPred[PredModuleNorthstar]) != 0 {
			t.Errorf("purpose %q should emit no module_northstar fact, got %d", purpose, len(byPred[PredModuleNorthstar]))
		}
		if len(byPred[PredModuleRequirement]) != 2 {
			t.Errorf("purpose %q should still emit 2 module_requirement facts, got %d", purpose, len(byPred[PredModuleRequirement]))
		}
		// Ensure requirements still have correct module path
		for _, rf := range byPred[PredModuleRequirement] {
			if got := rf.Args[0]; got != "internal/foo" {
				t.Errorf("empty purpose requirement ModulePath = %q, want %q", got, "internal/foo")
			}
		}
	}
}

func TestNorthstarFacts_EmptySeverityEmitsUnspecified(t *testing.T) {
	doc := &Document{
		Path: "nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			Northstar: &Northstar{
				Purpose: "Purpose with unspecified severity",
				Requirements: []NorthstarRequirement{
					{ID: "REQ-1", Statement: "No severity given", Severity: ""},
					{ID: "REQ-2", Statement: "Also no severity", Severity: "   "},
					{ID: "REQ-3", Statement: "Has severity", Severity: "minor"},
				},
			},
		},
	}
	facts := doc.Facts()
	byPred := map[string][]types.Fact{}
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}
	reqFacts := byPred[PredModuleRequirement]
	if len(reqFacts) != 3 {
		t.Fatalf("module_requirement facts = %d, want 3", len(reqFacts))
	}
	// First two should be /unspecified
	for i := 0; i < 2; i++ {
		atom, ok := reqFacts[i].Args[3].(types.MangleAtom)
		if !ok {
			t.Fatalf("requirement[%d] Severity is %T, want types.MangleAtom", i, reqFacts[i].Args[3])
		}
		if string(atom) != "/unspecified" {
			t.Errorf("requirement[%d] Severity = %q, want %q (empty severity should be /unspecified)", i, atom, "/unspecified")
		}
		if _, err := reqFacts[i].ToAtom(); err != nil {
			t.Errorf("requirement[%d] fact does not convert to atom: %v", i, err)
		}
	}
	// Third should be /minor as atom, not string
	atom, ok := reqFacts[2].Args[3].(types.MangleAtom)
	if !ok {
		t.Fatalf("requirement[2] Severity is %T, want types.MangleAtom", reqFacts[2].Args[3])
	}
	if string(atom) != "/minor" {
		t.Errorf("requirement[2] Severity = %q, want %q", atom, "/minor")
	}
	// Also verify severity is atom, not quoted string — atoms are disjoint from strings in Mangle
	for _, rf := range reqFacts {
		if _, isAtom := rf.Args[3].(types.MangleAtom); !isAtom {
			t.Errorf("Severity should be types.MangleAtom, got %T (%v)", rf.Args[3], rf.Args[3])
		}
		if str, isStr := rf.Args[3].(string); isStr {
			t.Errorf("Severity should not be plain string %q, should be MangleAtom", str)
		}
	}
}

func TestNorthstarFacts_PurposeTrimmed(t *testing.T) {
	doc := &Document{
		Path: "pkg/mymod/nerd.md",
		Spec: Spec{
			Schema: SchemaVersion,
			Northstar: &Northstar{
				Purpose: "   trimmed purpose   ",
				Requirements: []NorthstarRequirement{
					{ID: "REQ-1", Statement: "S", Severity: "blocker"},
				},
			},
		},
	}
	facts := doc.Facts()
	byPred := map[string][]types.Fact{}
	for _, f := range facts {
		byPred[f.Predicate] = append(byPred[f.Predicate], f)
	}
	nsFacts := byPred[PredModuleNorthstar]
	if len(nsFacts) != 1 {
		t.Fatalf("module_northstar facts = %d, want 1", len(nsFacts))
	}
	if got := nsFacts[0].Args[1]; got != "trimmed purpose" {
		t.Errorf("purpose should be trimmed, got %q, want %q", got, "trimmed purpose")
	}
}
