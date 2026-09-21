package build

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// modelLiteral matches a string that names a concrete model: a known family
// followed by a version. "gemini" alone is a provider; "gemini-3-pro" is a
// model, and a model is the user's to choose.
var modelLiteral = regexp.MustCompile(
	`^(?:[a-z0-9_-]+/)?(?:gemini|gpt|claude|grok|glm|qwen|kimi|moonshot|llama|muse|deepseek|mistral|codestral|o[134]|nomic|embeddinggemma|text-embedding)[-/.:a-zA-Z]*[0-9][-.:a-zA-Z0-9]*$`)

// modelLiteralAllowed names the production files that may spell a model, and
// why. Each is a table a human or a vendor defines, never a value the code
// falls back to when the configuration is silent.
var modelLiteralAllowed = map[string]string{
	"cmd/nerd/chat/config_wizard.go":      "the setup wizard's menu: the user picks one and it is written to config.json",
	"internal/usage/pricing.go":           "the price table, keyed by the model a receipt names",
	"internal/perception/client_types.go": "the OpenRouter catalogue the wizard offers",
}

// modelLiteralExact allows one literal in one file, for a string that has the
// shape of a model name and is not used as one.
var modelLiteralExact = map[string]string{
	"internal/perception/client_gemini.go|gemini-3":              "a family prefix matched against the configured model to detect a capability; never sent as a model",
	"internal/embedding/ollama.go|embeddinggemma:300m":           "tag alias: what is pulled when the user configured the bare name embeddinggemma; an unconfigured model is refused",
	"internal/config/user_config.go|gemini-3.1-flash-image":      "alias table: the API id for the friendly name nano-banana-2, which the user wrote in config.json",
	"internal/config/user_config.go|gemini-3.1-flash-lite-image": "alias table: the API id for the friendly name nano-banana-2-lite, which the user wrote in config.json",
}

// No model is hardcoded. Steve, 2026-09-21: "we need to make damn sure that we
// do not have hardcoded models anywhere in the codebase... if its not in the
// config.json... it just doesnt work". A default model is a decision nobody
// made: it spends the user's money on a model they did not choose, or stamps a
// record with a model that never ran (the prompt evolver labelled every
// verdict "gemini-3-pro" while judging on the configured Meta client).
//
// Production Go only: a test may name a model, because a test is not a
// fallback. String literals only, read from the AST, so a comment explaining a
// model name is not an offence.
func TestRepository_NoHardcodedModelNames(t *testing.T) {
	root := repoRoot(t)
	out, err := exec.Command("git", "-C", root, "ls-files", "-z", "--", "*.go").Output()
	if err != nil {
		t.Fatalf("git ls-files in %s: %v", root, err)
	}
	var offences []string
	for _, rel := range strings.Split(string(out), "\x00") {
		if rel == "" || strings.HasSuffix(rel, "_test.go") || strings.Contains("/"+rel, "/testdata/") {
			continue
		}
		if _, ok := modelLiteralAllowed[rel]; ok {
			continue
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if perr != nil {
			continue // the build gate reports a file that does not parse
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil || !modelLiteral.MatchString(value) {
				return true
			}
			if _, ok := modelLiteralExact[rel+"|"+value]; ok {
				return true
			}
			offences = append(offences, rel+":"+strconv.Itoa(fset.Position(lit.Pos()).Line)+"  "+lit.Value)
			return true
		})
	}
	if len(offences) > 0 {
		sort.Strings(offences)
		t.Errorf("%d hardcoded model name(s) in production code. A model comes from .nerd/config.json or the call fails; "+
			"a table of choices belongs in modelLiteralAllowed with its reason:\n%s",
			len(offences), strings.Join(offences, "\n"))
	}
}
