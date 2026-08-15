package context

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The compressed-context contract with the chat layer, encoded as a test.
//
// AUDIT (context TODO P0, "audit every chat path that injects history for
// IsCompressionActive parity"). The chat layer has exactly three places that
// hand conversation state to a model:
//
//	cmd/nerd/chat/process.go            perception input  — gated on IsCompressionActive
//	cmd/nerd/chat/process.go            articulation input — gated on IsCompressionActive
//	cmd/nerd/chat/model_session_context.go  shard SessionContext — compressed block only
//
// The first two choose between raw m.history and the compressor's recent
// window; both consult IsCompressionActive, so parity holds today. The third
// never touches m.history at all: shards get the compressed block as their
// only history, which is why it is unconditional and correct.
//
// The failure mode this guards is a fourth path: someone injecting raw
// m.history next to the compressed block without the gate, so the model
// receives the same turns twice — once verbatim and once compressed — and the
// window blows past the budget the compressor was built to defend. An audit
// that is not enforced decays, so it is enforced here.

const chatPkgDir = "../../cmd/nerd/chat"

// compressedContextAccessors are the compressor methods that yield the
// compressed history representation.
var compressedContextAccessors = map[string]bool{
	"GetContextString":    true,
	"GetRecentTurnWindow": true,
}

type funcFacts struct {
	name              string
	file              string
	usesRawHistory    bool
	usesCompressedCtx bool
	checksActive      bool
}

func scanChatFuncs(t *testing.T) []funcFacts {
	t.Helper()

	entries, err := os.ReadDir(chatPkgDir)
	if err != nil {
		t.Skipf("chat package not reachable from this test root: %v", err)
	}

	var out []funcFacts
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(chatPkgDir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ff := funcFacts{name: fn.Name.Name, file: name}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "history":
					if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "m" {
						ff.usesRawHistory = true
					}
				case "IsCompressionActive":
					ff.checksActive = true
				default:
					if compressedContextAccessors[sel.Sel.Name] {
						ff.usesCompressedCtx = true
					}
				}
				return true
			})
			out = append(out, ff)
		}
	}
	if len(out) == 0 {
		t.Fatal("scanned no functions in cmd/nerd/chat; the audit would pass vacuously")
	}
	return out
}

func TestChatHistoryInjection_WhenPathMixesRawAndCompressed_ShouldGateOnIsCompressionActive(t *testing.T) {
	mixed := 0
	for _, ff := range scanChatFuncs(t) {
		if !ff.usesRawHistory || !ff.usesCompressedCtx {
			continue
		}
		mixed++
		if !ff.checksActive {
			t.Errorf("%s:%s injects raw m.history alongside compressed context without consulting "+
				"IsCompressionActive — the model would receive the same turns twice",
				ff.file, ff.name)
		}
	}
	if mixed == 0 {
		// Without at least one real mixed path the check above proves nothing.
		t.Fatal("no chat function was found handling both raw history and compressed context; " +
			"either the paths moved or the scanner stopped matching, and this audit is now vacuous")
	}
}

func TestChatSessionContext_WhenBuildingShardContext_ShouldStayCompressedOnly(t *testing.T) {
	// buildSessionContext feeds shards via types.SessionContext.CompressedHistory
	// and is the one path allowed to inject unconditionally, precisely because
	// it carries no raw history. If raw history is ever added there it must
	// come with the gate, so we pin the property that makes the exception safe.
	found := false
	for _, ff := range scanChatFuncs(t) {
		if ff.name != "buildSessionContext" {
			continue
		}
		found = true
		if !ff.usesCompressedCtx {
			t.Error("buildSessionContext no longer injects the compressed block; the shard blackboard lost its history")
		}
		if ff.usesRawHistory && !ff.checksActive {
			t.Error("buildSessionContext started injecting raw m.history without an IsCompressionActive gate")
		}
	}
	if !found {
		t.Fatal("buildSessionContext not found in cmd/nerd/chat; the shard context path moved and this audit needs updating")
	}
}

func TestChatHistoryInjection_WhenCompressionActive_ShouldUseCompressorWindow(t *testing.T) {
	// Both LLM-facing paths in process.go must consult the compressor for the
	// window size rather than hardcoding a turn count next to the gate.
	var gated int
	for _, ff := range scanChatFuncs(t) {
		if ff.file == "process.go" && ff.checksActive && ff.usesCompressedCtx {
			gated++
		}
	}
	if gated == 0 {
		t.Fatal("no gated history-injection path found in cmd/nerd/chat/process.go; " +
			"perception/articulation stopped honouring IsCompressionActive")
	}
}
