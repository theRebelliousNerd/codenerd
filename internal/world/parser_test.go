package world

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGoCodeParser_Parse tests Go source file parsing.
func TestGoCodeParser_Parse(t *testing.T) {
	// Create a temp Go file
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package test

type User struct {
	ID   int    ` + "`json:\"user_id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

func NewUser(id int, name string) *User {
	return &User{ID: id, Name: name}
}

func (u *User) GetName() string {
	return u.Name
}

func processAsync(ctx context.Context) {
	go func() {
		// do work
	}()
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoCodeParser(tmpDir)

	// Test SupportedExtensions
	exts := parser.SupportedExtensions()
	if len(exts) != 1 || exts[0] != ".go" {
		t.Errorf("Expected [.go], got %v", exts)
	}

	// Test Language
	if parser.Language() != "go" {
		t.Errorf("Expected 'go', got %s", parser.Language())
	}

	// Test Parse
	content, _ := os.ReadFile(goFile)
	elements, err := parser.Parse(goFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: User struct, NewUser func, GetName method, processAsync func
	if len(elements) < 4 {
		t.Errorf("Expected at least 4 elements, got %d", len(elements))
	}

	// Check for struct
	var foundStruct, foundFunc, foundMethod bool
	for _, elem := range elements {
		if elem.Type == ElementStruct && elem.Name == "User" {
			foundStruct = true
		}
		if elem.Type == ElementFunction && elem.Name == "NewUser" {
			foundFunc = true
		}
		if elem.Type == ElementMethod && elem.Name == "GetName" {
			foundMethod = true
			if elem.Parent == "" {
				t.Error("Method should have parent ref")
			}
		}
	}

	if !foundStruct {
		t.Error("Did not find User struct")
	}
	if !foundFunc {
		t.Error("Did not find NewUser function")
	}
	if !foundMethod {
		t.Error("Did not find GetName method")
	}

	// Test EmitLanguageFacts
	facts := parser.EmitLanguageFacts(elements)
	if len(facts) == 0 {
		t.Error("Expected language facts, got none")
	}

	// Should have go_struct fact
	var foundGoStruct bool
	for _, f := range facts {
		if f.Predicate == "go_struct" {
			foundGoStruct = true
		}
	}
	if !foundGoStruct {
		t.Error("Did not find go_struct fact")
	}
}

// TestMangleCodeParser_Parse tests Mangle source file parsing.
func TestMangleCodeParser_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	mgFile := filepath.Join(tmpDir, "test.mg")
	mgContent := `# Test Mangle file
Decl user(ID, Name).
Decl admin(ID).

user("alice", "Alice Smith").
user("bob", "Bob Jones").

admin(ID) :- user(ID, _), fn:starts_with(ID, "a").

?admin(X).
`
	if err := os.WriteFile(mgFile, []byte(mgContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewMangleCodeParser(tmpDir)

	// Test SupportedExtensions
	exts := parser.SupportedExtensions()
	if len(exts) != 3 {
		t.Errorf("Expected 3 extensions, got %d", len(exts))
	}

	// Test Language
	if parser.Language() != "mg" {
		t.Errorf("Expected 'mg', got %s", parser.Language())
	}

	// Test Parse
	content, _ := os.ReadFile(mgFile)
	elements, err := parser.Parse(mgFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: 2 decls, 2 facts, 1 rule, 1 query
	var declCount, factCount, ruleCount, queryCount int
	for _, elem := range elements {
		switch elem.Type {
		case ElementMangleDecl:
			declCount++
		case ElementMangleFact:
			factCount++
		case ElementMangleRule:
			ruleCount++
		case ElementMangleQuery:
			queryCount++
		}
	}

	if declCount != 2 {
		t.Errorf("Expected 2 decls, got %d", declCount)
	}
	if factCount != 2 {
		t.Errorf("Expected 2 facts, got %d", factCount)
	}
	if ruleCount != 1 {
		t.Errorf("Expected 1 rule, got %d", ruleCount)
	}
	if queryCount != 1 {
		t.Errorf("Expected 1 query, got %d", queryCount)
	}
}

// TestParserFactory_Registration tests parser registration.
func TestParserFactory_Registration(t *testing.T) {
	factory := NewParserFactory("/project")

	goParser := NewGoCodeParser("/project")
	factory.Register(goParser)

	// Should be able to get parser for .go files
	if !factory.HasParser("test.go") {
		t.Error("Factory should have parser for .go files")
	}
	if factory.HasParser("test.py") {
		t.Error("Factory should not have parser for .py files (yet)")
	}

	// GetParser should return the Go parser
	parser := factory.GetParser("test.go")
	if parser == nil {
		t.Error("GetParser returned nil for .go file")
	}
	if parser.Language() != "go" {
		t.Error("GetParser returned wrong parser")
	}
}

// TestParserFactory_Parse tests factory-based parsing.
func TestParserFactory_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	goContent := `package main

func main() {
	println("Hello")
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	factory := DefaultParserFactory(tmpDir)
	content, _ := os.ReadFile(goFile)

	elements, err := factory.Parse(goFile, content)
	if err != nil {
		t.Fatalf("Factory parse failed: %v", err)
	}

	if len(elements) != 1 {
		t.Errorf("Expected 1 element (main func), got %d", len(elements))
	}
}

// TestCodeElementParser_BackwardCompatibility tests legacy mode.
func TestCodeElementParser_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "test.go")
	goContent := `package test

type Config struct{}

func Init() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test legacy constructor (no factory)
	parser := NewCodeElementParser()
	elements, err := parser.ParseFile(goFile)
	if err != nil {
		t.Fatalf("Legacy parse failed: %v", err)
	}

	if len(elements) != 2 {
		t.Errorf("Expected 2 elements, got %d", len(elements))
	}

	// Factory should be nil in legacy mode
	if parser.Factory() != nil {
		t.Error("Legacy parser should have nil factory")
	}
}

// TestCodeElementParser_WithFactory tests polyglot mode.
func TestCodeElementParser_WithFactory(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "app.go")
	goContent := `package app

type App struct{}

func Run() {}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test factory-based constructor
	parser := NewCodeElementParserWithRoot(tmpDir)
	elements, err := parser.ParseFile(goFile)
	if err != nil {
		t.Fatalf("Factory-based parse failed: %v", err)
	}

	if len(elements) != 2 {
		t.Errorf("Expected 2 elements, got %d", len(elements))
	}

	// Factory should be present
	if parser.Factory() == nil {
		t.Error("Factory-based parser should have factory")
	}
}

// TestGoCodeParser_StructTags tests struct tag extraction.
func TestGoCodeParser_StructTags(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "model.go")
	goContent := `package model

type User struct {
	ID        int    ` + "`json:\"user_id\" db:\"id\"`" + `
	Name      string ` + "`json:\"name\"`" + `
	CreatedAt int64  ` + "`json:\"created_at\"`" + `
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoCodeParser(tmpDir)
	content, _ := os.ReadFile(goFile)
	elements, err := parser.Parse(goFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	// Should have go_tag facts for the struct fields
	var tagCount int
	for _, f := range facts {
		if f.Predicate == "go_tag" {
			tagCount++
		}
	}

	if tagCount < 3 {
		t.Errorf("Expected at least 3 go_tag facts, got %d", tagCount)
	}
}

// TestGoCodeParser_Goroutines tests goroutine detection.
func TestGoCodeParser_Goroutines(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "async.go")
	goContent := `package async

func ProcessBatch(items []Item) {
	go func() {
		for _, item := range items {
			process(item)
		}
	}()
}

func ProcessSingle(item Item) {
	go processItem(item)
}

func SyncProcess(item Item) {
	processItem(item)
}
`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewGoCodeParser(tmpDir)
	content, _ := os.ReadFile(goFile)
	elements, err := parser.Parse(goFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	// Should have go_goroutine facts for ProcessBatch and ProcessSingle, but not SyncProcess
	var goroutineCount int
	for _, f := range facts {
		if f.Predicate == "go_goroutine" {
			goroutineCount++
		}
	}

	if goroutineCount < 2 {
		t.Errorf("Expected at least 2 go_goroutine facts, got %d", goroutineCount)
	}
}

// TestMangleCodeParser_RuleDetection tests Mangle rule analysis.
func TestMangleCodeParser_RuleDetection(t *testing.T) {
	tmpDir := t.TempDir()
	mgFile := filepath.Join(tmpDir, "rules.mg")
	mgContent := `# Test rules
Decl ancestor(X, Y).
Decl parent(X, Y).

# Recursive rule
ancestor(X, Y) :- parent(X, Y).
ancestor(X, Z) :- parent(X, Y), ancestor(Y, Z).

# Rule with negation
orphan(X) :- person(X), not parent(_, X).

# Rule with aggregation
total_children(Parent, Count) :-
	parent(Parent, _) |>
	do fn:group_by(Parent),
	let Count = fn:count().
`
	if err := os.WriteFile(mgFile, []byte(mgContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewMangleCodeParser(tmpDir)
	content, _ := os.ReadFile(mgFile)
	elements, err := parser.Parse(mgFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	var recursiveCount, negationCount, aggregationCount int
	for _, f := range facts {
		switch f.Predicate {
		case "mg_recursive_rule":
			recursiveCount++
		case "mg_negation_rule":
			negationCount++
		case "mg_aggregation_rule":
			aggregationCount++
		}
	}

	if recursiveCount < 1 {
		t.Errorf("Expected at least 1 recursive rule, got %d", recursiveCount)
	}
	if negationCount < 1 {
		t.Errorf("Expected at least 1 negation rule, got %d", negationCount)
	}
	if aggregationCount < 1 {
		t.Errorf("Expected at least 1 aggregation rule, got %d", aggregationCount)
	}
}
