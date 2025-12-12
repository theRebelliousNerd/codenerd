package autopoiesis

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
	"time"

	core "codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
)

var goSafetyPolicy string

func init() {
	if policy, err := core.GetDefaultContent("go_safety.mg"); err == nil {
		goSafetyPolicy = policy
	} else {
		logging.Get(logging.CategoryAutopoiesis).Warn("Failed to load embedded go_safety.mg: %v", err)
		goSafetyPolicy = ""
	}
}

// SafetyChecker validates generated tool code for safety using a Mangle policy.
type SafetyChecker struct {
	config      OuroborosConfig
	policy      string
	allowedPkgs []string
}

// SafetyReport contains the results of a safety check.
type SafetyReport struct {
	Safe           bool
	Violations     []SafetyViolation
	ImportsChecked int
	CallsChecked   int
	Score          float64 // 0.0 = unsafe, 1.0 = perfectly safe
}

// SafetyViolation describes a single safety issue.
type SafetyViolation struct {
	Type        ViolationType
	Location    string // file:line or logical identifier
	Description string
	Severity    ViolationSeverity
}

// ViolationType categorizes violations.
type ViolationType int

const (
	ViolationForbiddenImport ViolationType = iota
	ViolationDangerousCall
	ViolationUnsafePointer
	ViolationReflection
	ViolationCGO
	ViolationExec
	ViolationPanic
	ViolationGoroutineLeak
	ViolationParseError
	ViolationPolicy
	ViolationPanicMakerKill // NEW: Tool killed by PanicMaker adversarial testing
)

func (v ViolationType) String() string {
	switch v {
	case ViolationForbiddenImport:
		return "forbidden_import"
	case ViolationDangerousCall:
		return "dangerous_call"
	case ViolationUnsafePointer:
		return "unsafe_pointer"
	case ViolationReflection:
		return "reflection"
	case ViolationCGO:
		return "cgo"
	case ViolationExec:
		return "exec"
	case ViolationPanic:
		return "panic"
	case ViolationGoroutineLeak:
		return "goroutine_leak"
	case ViolationParseError:
		return "parse_error"
	case ViolationPolicy:
		return "policy_violation"
	case ViolationPanicMakerKill:
		return "panic_maker_kill"
	default:
		return "unknown"
	}
}

// ViolationSeverity indicates how serious a violation is.
type ViolationSeverity int

const (
	SeverityInfo ViolationSeverity = iota
	SeverityWarning
	SeverityCritical
	SeverityBlocking
)

// NewSafetyChecker creates a new safety checker backed by the Mangle policy.
func NewSafetyChecker(config OuroborosConfig) *SafetyChecker {
	logging.AutopoiesisDebug("Creating SafetyChecker: AllowFileSystem=%v, AllowNetworking=%v, AllowExec=%v",
		config.AllowFileSystem, config.AllowNetworking, config.AllowExec)

	checker := &SafetyChecker{
		config: config,
		policy: goSafetyPolicy,
	}
	checker.allowedPkgs = checker.buildAllowedPackages()

	logging.Autopoiesis("SafetyChecker initialized with %d allowed packages", len(checker.allowedPkgs))
	logging.AutopoiesisDebug("Allowed packages: %v", checker.allowedPkgs)

	return checker
}

// ExtractASTFacts parses Go source and emits structural facts for the safety policy.
func ExtractASTFacts(sourceCode string) ([]mangle.Fact, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", sourceCode, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	fileName := fset.File(file.Pos()).Name()
	emitter := &astFactEmitter{
		fset:     fset,
		fileName: fileName,
	}
	emitter.emitImports(file)
	ast.Walk(&astFactVisitor{emitter: emitter}, file)

	return emitter.facts, nil
}

// Check performs a safety check on the code using the Mangle policy.
func (sc *SafetyChecker) Check(code string) *SafetyReport {
	timer := logging.StartTimer(logging.CategoryAutopoiesis, "SafetyChecker.Check")
	defer timer.Stop()

	logging.Autopoiesis("Starting safety check on code (%d bytes)", len(code))

	report := &SafetyReport{
		Safe:       true,
		Violations: []SafetyViolation{},
		Score:      1.0,
	}

	// AST Analysis phase
	logging.AutopoiesisDebug("Extracting AST facts from code")
	astTimer := logging.StartTimer(logging.CategoryAutopoiesis, "ASTFactExtraction")
	facts, err := ExtractASTFacts(code)
	astTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("AST parsing failed: %v", err)
		return sc.fail(report, ViolationParseError, "", fmt.Sprintf("failed to parse code: %v", err))
	}
	logging.AutopoiesisDebug("Extracted %d AST facts", len(facts))

	index := buildFactIndex(facts)
	report.ImportsChecked = len(index.imports)
	report.CallsChecked = index.callCount
	logging.AutopoiesisDebug("Fact index built: imports=%d, calls=%d, panicFuncs=%d, goroutines=%d",
		len(index.imports), index.callCount, len(index.panicFuncs), len(index.goroutineLines))

	// Log detected imports
	if len(index.imports) > 0 {
		importList := make([]string, 0, len(index.imports))
		for imp := range index.imports {
			importList = append(importList, imp)
		}
		logging.AutopoiesisDebug("Detected imports: %v", importList)
	}

	// Seed allowlist facts from config.
	for _, pkg := range sc.allowedPkgs {
		facts = append(facts, mangle.Fact{
			Predicate: "allowed_package",
			Args:      []interface{}{pkg},
		})
	}

	// Policy validation phase
	logging.AutopoiesisDebug("Initializing safety policy engine")
	engine, err := sc.newEngine()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to init safety engine: %v", err)
		return sc.fail(report, ViolationPolicy, "", fmt.Sprintf("failed to init safety engine: %v", err))
	}

	if err := engine.AddFacts(facts); err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Failed to add facts to engine: %v", err)
		return sc.fail(report, ViolationPolicy, "", fmt.Sprintf("failed to add facts: %v", err))
	}
	logging.AutopoiesisDebug("Added %d facts to safety engine", len(facts))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	logging.AutopoiesisDebug("Querying safety policy for violations")
	queryTimer := logging.StartTimer(logging.CategoryAutopoiesis, "PolicyQuery")
	result, err := engine.Query(ctx, "?violation(V)")
	queryTimer.Stop()
	if err != nil {
		logging.Get(logging.CategoryAutopoiesis).Error("Safety policy query failed: %v", err)
		return sc.fail(report, ViolationPolicy, "", fmt.Sprintf("safety policy query failed: %v", err))
	}

	if len(result.Bindings) == 0 {
		logging.Autopoiesis("Safety check PASSED: no violations detected (imports=%d, calls=%d)",
			report.ImportsChecked, report.CallsChecked)
		return report
	}

	report.Safe = false
	report.Score = 0.0
	for _, binding := range result.Bindings {
		// Mangle bindings return generic values, usually matching declared types.
		// violation arg is string | int
		value := binding["V"]
		violation := describeViolation(value, index)
		report.Violations = append(report.Violations, violation)
		logging.Get(logging.CategoryAutopoiesis).Warn("Safety violation: type=%s, severity=%s, desc=%s",
			violation.Type.String(), severityString(violation.Severity), violation.Description)
	}

	logging.Autopoiesis("Safety check FAILED: %d violations detected", len(report.Violations))
	return report
}

func (sc *SafetyChecker) fail(report *SafetyReport, vType ViolationType, location, msg string) *SafetyReport {
	report.Safe = false
	report.Score = 0.0
	report.Violations = append(report.Violations, SafetyViolation{
		Type:        vType,
		Location:    location,
		Description: msg,
		Severity:    SeverityBlocking,
	})
	return report
}

func (sc *SafetyChecker) newEngine() (*mangle.Engine, error) {
	cfg := mangle.DefaultConfig()
	cfg.FactLimit = 20000
	cfg.AutoEval = true
	cfg.QueryTimeout = 5

	engine, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, err
	}

	if err := engine.LoadSchemaString(sc.policy); err != nil {
		return nil, err
	}

	return engine, nil
}

func (sc *SafetyChecker) buildAllowedPackages() []string {
	base := []string{
		"bytes",
		"bufio",
		"context",
		"encoding/base64",
		"encoding/hex",
		"encoding/json",
		"errors",
		"fmt",
		"io",
		"log",
		"math",
		"math/big",
		"regexp",
		"sort",
		"strconv",
		"strings",
		"sync",
		"sync/atomic",
		"time",
		"unicode",
		"unicode/utf8",
		"net/url",
	}

	if sc.config.AllowFileSystem {
		base = append(base, "os", "path/filepath", "io/ioutil", "path")
	}
	if sc.config.AllowNetworking {
		base = append(base, "net", "net/http", "net/url")
	}
	if sc.config.AllowExec {
		base = append(base, "os/exec")
	}

	seen := make(map[string]struct{}, len(base))
	for _, pkg := range base {
		seen[pkg] = struct{}{}
	}

	allowed := make([]string, 0, len(seen))
	for pkg := range seen {
		allowed = append(allowed, pkg)
	}
	sort.Strings(allowed)
	return allowed
}

type factIndex struct {
	imports        map[string]struct{}
	panicFuncs     map[string]struct{}
	goroutineLines map[string]struct{}
	callCount      int
}

func buildFactIndex(facts []mangle.Fact) factIndex {
	idx := factIndex{
		imports:        make(map[string]struct{}),
		panicFuncs:     make(map[string]struct{}),
		goroutineLines: make(map[string]struct{}),
	}

	for _, fact := range facts {
		switch fact.Predicate {
		case "ast_import":
			if len(fact.Args) > 1 {
				if pkg, ok := fact.Args[1].(string); ok {
					idx.imports[pkg] = struct{}{}
				}
			}
		case "ast_call":
			idx.callCount++
			if len(fact.Args) > 1 {
				callee, _ := fact.Args[1].(string)
				if callee == "panic" && len(fact.Args) > 0 {
					if fn, ok := fact.Args[0].(string); ok {
						idx.panicFuncs[fn] = struct{}{}
					}
				}
			}
		case "ast_goroutine_spawn":
			if len(fact.Args) > 1 {
				if line, ok := fact.Args[1].(string); ok {
					idx.goroutineLines[line] = struct{}{}
				}
			}
		}
	}
	return idx
}

func describeViolation(value interface{}, idx factIndex) SafetyViolation {
	switch v := value.(type) {
	case string:
		// Mangle may return atoms with "/" prefix - strip it for lookups
		lookupKey := v
		if strings.HasPrefix(v, "/") {
			lookupKey = v[1:]
		}

		if _, ok := idx.imports[lookupKey]; ok {
			return SafetyViolation{
				Type:        ViolationForbiddenImport,
				Description: fmt.Sprintf("import %q is not on the allowlist", lookupKey),
				Severity:    SeverityBlocking,
			}
		}
		if _, ok := idx.panicFuncs[lookupKey]; ok {
			return SafetyViolation{
				Type:        ViolationPanic,
				Location:    lookupKey,
				Description: "panic is not permitted in generated code; return an error instead",
				Severity:    SeverityBlocking,
			}
		}
		if _, ok := idx.goroutineLines[lookupKey]; ok {
			return SafetyViolation{
				Type:        ViolationGoroutineLeak,
				Location:    fmt.Sprintf("line:%s", lookupKey),
				Description: "goroutine spawn must accept a cancelable context",
				Severity:    SeverityBlocking,
			}
		}
		return SafetyViolation{
			Type:        ViolationPolicy,
			Description: fmt.Sprintf("policy violation: %v", v),
			Severity:    SeverityBlocking,
		}
	default:
		return SafetyViolation{
			Type:        ViolationPolicy,
			Description: fmt.Sprintf("policy violation: %v", v),
			Severity:    SeverityBlocking,
		}
	}
}

// astFactEmitter walks an AST and emits facts for the safety policy.
type astFactEmitter struct {
	fset       *token.FileSet
	fileName   string
	currentFcn string
	facts      []mangle.Fact
}

func (e *astFactEmitter) emitImports(file *ast.File) {
	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)
		e.facts = append(e.facts, mangle.Fact{
			Predicate: "ast_import",
			Args:      []interface{}{e.fileName, importPath},
		})
	}
}

func (e *astFactEmitter) emitCall(call *ast.CallExpr) {
	callee := e.exprToString(call.Fun)
	e.facts = append(e.facts, mangle.Fact{
		Predicate: "ast_call",
		Args:      []interface{}{e.currentFcn, callee},
	})
}

func (e *astFactEmitter) emitGoroutine(stmt *ast.GoStmt) {
	line := fmt.Sprintf("%d", e.fset.Position(stmt.Go).Line)
	target := e.exprToString(stmt.Call.Fun)
	e.facts = append(e.facts, mangle.Fact{
		Predicate: "ast_goroutine_spawn",
		Args:      []interface{}{target, line},
	})

	if e.usesContextCancellation(stmt.Call) {
		e.facts = append(e.facts, mangle.Fact{
			Predicate: "ast_uses_context_cancellation",
			Args:      []interface{}{line},
		})
	}
}

func (e *astFactEmitter) emitAssignment(assign *ast.AssignStmt) {
	for i, lhs := range assign.Lhs {
		if i >= len(assign.Rhs) {
			break
		}
		if ident, ok := lhs.(*ast.Ident); ok && ident.Name != "_" {
			if rhsIdent, ok := assign.Rhs[i].(*ast.Ident); ok && rhsIdent.Name == "nil" {
				e.facts = append(e.facts, mangle.Fact{
					Predicate: "ast_assignment",
					Args:      []interface{}{ident.Name, "nil"},
				})
			}
		}
	}
}

func (e *astFactEmitter) usesContextCancellation(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		switch a := arg.(type) {
		case *ast.Ident:
			if strings.Contains(strings.ToLower(a.Name), "ctx") {
				return true
			}
			if strings.Contains(strings.ToLower(a.Name), "cancel") {
				return true
			}
		case *ast.SelectorExpr:
			if ident, ok := a.X.(*ast.Ident); ok {
				name := strings.ToLower(ident.Name)
				if strings.Contains(name, "ctx") || strings.Contains(name, "cancel") {
					return true
				}
			}
		}
	}

	callee := strings.ToLower(e.exprToString(call.Fun))
	return strings.Contains(callee, "withcancel") ||
		strings.Contains(callee, "withtimeout") ||
		strings.Contains(callee, "withdeadline")
}

func (e *astFactEmitter) exprToString(expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, e.fset, expr)
	return buf.String()
}

type astFactVisitor struct {
	emitter *astFactEmitter
}

func (v *astFactVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		prev := v.emitter.currentFcn
		v.emitter.currentFcn = n.Name.Name
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.emitter.currentFcn = prev
		return nil
	case *ast.FuncLit:
		prev := v.emitter.currentFcn
		v.emitter.currentFcn = v.funcLiteralLabel(n)
		if n.Body != nil {
			ast.Walk(v, n.Body)
		}
		v.emitter.currentFcn = prev
		return nil
	case *ast.CallExpr:
		v.emitter.emitCall(n)
	case *ast.GoStmt:
		v.emitter.emitGoroutine(n)
	case *ast.AssignStmt:
		v.emitter.emitAssignment(n)
	}

	return v
}

func (v *astFactVisitor) funcLiteralLabel(lit *ast.FuncLit) string {
	pos := v.emitter.fset.Position(lit.Pos())
	// Use _ to differentiate system generated names
	return fmt.Sprintf("func_literal_%d", pos.Line)
}

// FormatViolationsForFeedback creates a human-readable description of violations
// suitable for feeding back to an LLM for code regeneration.
func FormatViolationsForFeedback(violations []SafetyViolation) string {
	if len(violations) == 0 {
		return "No violations detected."
	}

	var sb strings.Builder
	sb.WriteString("Safety violations detected:\n\n")
	for i, v := range violations {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, v.Type.String(), v.Description))
		if v.Location != "" {
			sb.WriteString(fmt.Sprintf(" (at %s)", v.Location))
		}
		sb.WriteString(fmt.Sprintf(" [Severity: %s]\n", severityString(v.Severity)))
	}
	return sb.String()
}

// severityString converts ViolationSeverity to a human-readable string.
func severityString(s ViolationSeverity) string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	case SeverityBlocking:
		return "blocking"
	default:
		return "unknown"
	}
}
