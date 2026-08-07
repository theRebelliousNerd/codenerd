package campaign

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// The build-topology contract spans two languages: Go decides which category strings
// survive normalization, and Mangle decides what each surviving category *means* for
// build ordering. Neither half can see the other, and when they drifted apart the
// symptom was a bogus "missing_category" validation error on every campaign plan --
// three categories (/research, /test, /ops) were accepted by Go and unknown to the
// kernel, so phase_precedence never derived and has_phase_category/1 was false.
//
// These tests are the seam. They read the real .mg corpus, not a copy.

const buildTopologyPath = "../core/defaults/build_topology.mg"

var (
	buildPhaseTypeRe = regexp.MustCompile(`^\s*build_phase_type\((/[a-z_]+),\s*(-?\d+)\)\.`)
	phaseSynonymRe   = regexp.MustCompile(`^\s*phase_synonym\((/[a-z_]+),\s*"([^"]+)"\)\.`)
	mgLineRe         = regexp.MustCompile(`(?m)^.*$`)
)

// kernelBuildLayers parses build_phase_type/2 out of the live corpus file.
func kernelBuildLayers(t *testing.T) (map[string]int, map[string]string) {
	t.Helper()

	data, err := os.ReadFile(filepath.FromSlash(buildTopologyPath))
	if err != nil {
		t.Fatalf("read %s: %v", buildTopologyPath, err)
	}

	layers := make(map[string]int)
	synonyms := make(map[string]string)
	for _, line := range mgLineRe.FindAllString(string(data), -1) {
		if m := buildPhaseTypeRe.FindStringSubmatch(line); m != nil {
			score, err := strconv.Atoi(m[2])
			if err != nil {
				t.Fatalf("build_phase_type score %q is not an integer: %v", m[2], err)
			}
			layers[m[1]] = score
			continue
		}
		if m := phaseSynonymRe.FindStringSubmatch(line); m != nil {
			synonyms[m[2]] = m[1]
		}
	}

	if len(layers) == 0 {
		t.Fatalf("parsed zero build_phase_type facts from %s -- regex or file layout changed", buildTopologyPath)
	}
	return layers, synonyms
}

func TestPhaseCategoryTablesMatchKernel(t *testing.T) {
	layers, _ := kernelBuildLayers(t)

	for category := range allowedPhaseCategories {
		if _, ok := layers[category]; !ok {
			t.Errorf("category %s is accepted by allowedPhaseCategories but has no build_phase_type fact in %s; "+
				"phases in that category derive no phase_precedence and the kernel reports missing_category",
				category, buildTopologyPath)
		}
	}

	for category := range layers {
		if _, ok := allowedPhaseCategories[category]; !ok {
			t.Errorf("build_phase_type(%s, _) exists in the kernel but normalizePhaseCategory can never emit it; "+
				"add it to allowedPhaseCategories or delete the fact", category)
		}
	}
}

// Two layers sharing a score are unorderable: architectural_violation and
// suspicious_gap both compare scores, so a tie silently disables enforcement between
// exactly those two layers.
func TestBuildLayerScoresAreDistinct(t *testing.T) {
	layers, _ := kernelBuildLayers(t)

	seen := make(map[int]string, len(layers))
	for category, score := range layers {
		if other, dup := seen[score]; dup {
			t.Errorf("build layers %s and %s share score %d; topology ordering between them is undefined",
				other, category, score)
		}
		seen[score] = category
	}
}

func TestPhaseSynonymTablesMatchKernel(t *testing.T) {
	layers, kernelSynonyms := kernelBuildLayers(t)

	for alias, category := range kernelSynonyms {
		got, ok := phaseCategorySynonyms[alias]
		if !ok {
			t.Errorf("phase_synonym(%s, %q) exists in the kernel but not in phaseCategorySynonyms; "+
				"the Go normalizer collapses %q to the fallback before the kernel ever sees it",
				category, alias, alias)
			continue
		}
		if got != category {
			t.Errorf("alias %q maps to %s in Go but %s in the kernel", alias, got, category)
		}
	}

	for alias, category := range phaseCategorySynonyms {
		if kernelSynonyms[alias] != category {
			t.Errorf("phaseCategorySynonyms[%q] = %s has no matching phase_synonym fact in %s",
				alias, category, buildTopologyPath)
		}
		if _, ok := layers[category]; !ok {
			t.Errorf("alias %q resolves to %s, which is not a canonical build layer", alias, category)
		}
		if _, collides := allowedPhaseCategories["/"+alias]; collides {
			t.Errorf("alias %q collides with canonical category /%s; the alias branch would shadow it", alias, alias)
		}
	}
}

func TestNormalizePhaseCategory(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"canonical passthrough", "/research", "/research"},
		{"canonical without slash", "test", "/test"},
		{"case and space insensitive", "  /Domain_Core  ", "/domain_core"},
		{"alias resolves to layer", "testing", "/test"},
		{"alias with slash", "/api", "/transport"},
		{"alias uppercase", "DEPLOY", "/ops"},
		{"empty falls back", "", "/service"},
		{"unknown falls back", "wibble", "/service"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizePhaseCategory(tc.input); got != tc.want {
				t.Errorf("normalizePhaseCategory(%q) = %s, want %s", tc.input, got, tc.want)
			}
		})
	}
}

// The scaffold plan substituted when decomposition fails must itself pass topology
// validation -- otherwise every degraded run also reports phantom category errors on
// top of the real failure, burying the signal that actually matters.
func TestScaffoldPlanCategoriesAreKernelKnown(t *testing.T) {
	layers, _ := kernelBuildLayers(t)

	for _, category := range []string{"/research", "/scaffold", "/test"} {
		normalized := normalizePhaseCategory(category)
		if normalized != category {
			t.Errorf("scaffold category %s normalized to %s", category, normalized)
		}
		if _, ok := layers[normalized]; !ok {
			t.Errorf("scaffold category %s has no build_phase_type fact", normalized)
		}
	}
}
