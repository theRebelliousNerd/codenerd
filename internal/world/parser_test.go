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

// =============================================================================
// Python Parser Tests
// =============================================================================

// TestPythonCodeParser_Parse tests Python source file parsing.
func TestPythonCodeParser_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "models.py")
	pyContent := `from pydantic import BaseModel

class User(BaseModel):
    """A user model."""
    id: int
    name: str

    def validate_name(self) -> bool:
        return len(self.name) > 0

@login_required
def get_user(user_id: int) -> User:
    """Fetch a user by ID."""
    return fetch_user(user_id)

async def async_fetch(url: str) -> dict:
    """Async fetch function."""
    return await http_get(url)

def _private_helper():
    pass
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewPythonCodeParser(tmpDir)

	// Test SupportedExtensions
	exts := parser.SupportedExtensions()
	if len(exts) != 2 || exts[0] != ".py" {
		t.Errorf("Expected [.py, .pyw], got %v", exts)
	}

	// Test Language
	if parser.Language() != "py" {
		t.Errorf("Expected 'py', got %s", parser.Language())
	}

	// Test Parse
	content, _ := os.ReadFile(pyFile)
	elements, err := parser.Parse(pyFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: User class, validate_name method, get_user func, async_fetch func, _private_helper
	if len(elements) < 4 {
		t.Errorf("Expected at least 4 elements, got %d", len(elements))
	}

	// Check for class and methods
	var foundClass, foundMethod, foundAsync, foundPrivate bool
	for _, elem := range elements {
		switch elem.Name {
		case "User":
			foundClass = true
			if elem.Type != ElementStruct {
				t.Error("User should be ElementStruct")
			}
		case "validate_name":
			foundMethod = true
			if elem.Type != ElementMethod {
				t.Error("validate_name should be ElementMethod")
			}
			if elem.Parent == "" {
				t.Error("Method should have parent ref")
			}
		case "async_fetch":
			foundAsync = true
		case "_private_helper":
			foundPrivate = true
			if elem.Visibility != VisibilityPrivate {
				t.Error("_private_helper should be private")
			}
		}
	}

	if !foundClass {
		t.Error("Did not find User class")
	}
	if !foundMethod {
		t.Error("Did not find validate_name method")
	}
	if !foundAsync {
		t.Error("Did not find async_fetch function")
	}
	if !foundPrivate {
		t.Error("Did not find _private_helper function")
	}
}

// TestPythonCodeParser_EmitLanguageFacts tests Python fact emission.
func TestPythonCodeParser_EmitLanguageFacts(t *testing.T) {
	tmpDir := t.TempDir()
	pyFile := filepath.Join(tmpDir, "api.py")
	pyContent := `from pydantic import BaseModel

class UserRequest(BaseModel):
    name: str

@dataclass
class Config:
    debug: bool

@login_required
@rate_limit(100)
async def handle_request(req: UserRequest) -> dict:
    return {"ok": True}

def typed_function(x: int) -> str:
    return str(x)
`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewPythonCodeParser(tmpDir)
	content, _ := os.ReadFile(pyFile)
	elements, err := parser.Parse(pyFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	var hasPydanticBase, hasDecorator, hasAsync, hasTyped bool
	for _, f := range facts {
		switch f.Predicate {
		case "has_pydantic_base":
			hasPydanticBase = true
		case "py_decorator":
			hasDecorator = true
		case "py_async_def":
			hasAsync = true
		case "py_typed_function":
			hasTyped = true
		}
	}

	if !hasPydanticBase {
		t.Error("Expected has_pydantic_base fact")
	}
	if !hasDecorator {
		t.Error("Expected py_decorator facts")
	}
	if !hasAsync {
		t.Error("Expected py_async_def fact")
	}
	if !hasTyped {
		t.Error("Expected py_typed_function fact")
	}
}

// =============================================================================
// TypeScript Parser Tests
// =============================================================================

// TestTypeScriptCodeParser_Parse tests TypeScript source file parsing.
func TestTypeScriptCodeParser_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "components.tsx")
	tsContent := `export interface IUser {
  id: number;
  name: string;
  email?: string;
}

export type UserID = string | number;

export class UserService {
  private cache: Map<string, IUser>;

  async fetchUser(id: number): Promise<IUser> {
    return await api.get('/users/' + id);
  }

  getFromCache(id: string): IUser | undefined {
    return this.cache.get(id);
  }
}

export function validateUser(user: IUser): boolean {
  return user.name.length > 0;
}

export const UserCard = (props: { user: IUser }) => {
  const [loading, setLoading] = useState(false);
  return <div className="card">{props.user.name}</div>;
};
`
	if err := os.WriteFile(tsFile, []byte(tsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewTypeScriptCodeParser(tmpDir)

	// Test SupportedExtensions
	exts := parser.SupportedExtensions()
	expectedExts := []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"}
	if len(exts) != len(expectedExts) {
		t.Errorf("Expected %d extensions, got %d", len(expectedExts), len(exts))
	}

	// Test Language
	if parser.Language() != "ts" {
		t.Errorf("Expected 'ts', got %s", parser.Language())
	}

	// Test Parse
	content, _ := os.ReadFile(tsFile)
	elements, err := parser.Parse(tsFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: IUser interface, UserID type, UserService class, methods, functions, UserCard
	if len(elements) < 5 {
		t.Errorf("Expected at least 5 elements, got %d", len(elements))
	}

	var foundInterface, foundType, foundClass, foundMethod, foundFunc bool
	for _, elem := range elements {
		switch elem.Name {
		case "IUser":
			foundInterface = true
			if elem.Type != ElementInterface {
				t.Error("IUser should be ElementInterface")
			}
		case "UserID":
			foundType = true
			if elem.Type != ElementType_ {
				t.Error("UserID should be ElementType_")
			}
		case "UserService":
			foundClass = true
			if elem.Type != ElementStruct {
				t.Error("UserService should be ElementStruct")
			}
		case "fetchUser":
			foundMethod = true
			if elem.Type != ElementMethod {
				t.Error("fetchUser should be ElementMethod")
			}
		case "validateUser":
			foundFunc = true
			if elem.Type != ElementFunction {
				t.Error("validateUser should be ElementFunction")
			}
		}
	}

	if !foundInterface {
		t.Error("Did not find IUser interface")
	}
	if !foundType {
		t.Error("Did not find UserID type")
	}
	if !foundClass {
		t.Error("Did not find UserService class")
	}
	if !foundMethod {
		t.Error("Did not find fetchUser method")
	}
	if !foundFunc {
		t.Error("Did not find validateUser function")
	}
}

// TestTypeScriptCodeParser_EmitLanguageFacts tests TypeScript fact emission.
func TestTypeScriptCodeParser_EmitLanguageFacts(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "app.tsx")
	tsContent := `export interface IConfig {
  apiUrl: string;
  timeout: number;
}

export const App = () => {
  const [state, setState] = useState({});
  useEffect(() => console.log("mounted"), []);
  return <div><h1>App</h1></div>;
};

async function fetchData(): Promise<void> {
  await api.get('/data');
}
`
	if err := os.WriteFile(tsFile, []byte(tsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewTypeScriptCodeParser(tmpDir)
	content, _ := os.ReadFile(tsFile)
	elements, err := parser.Parse(tsFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	var hasInterface, hasInterfaceProp, hasComponent, hasHook, hasAsync bool
	for _, f := range facts {
		switch f.Predicate {
		case "ts_interface":
			hasInterface = true
		case "ts_interface_prop":
			hasInterfaceProp = true
		case "ts_component":
			hasComponent = true
		case "ts_hook":
			hasHook = true
		case "ts_async_function":
			hasAsync = true
		}
	}

	if !hasInterface {
		t.Error("Expected ts_interface fact")
	}
	if !hasInterfaceProp {
		t.Error("Expected ts_interface_prop facts")
	}
	if !hasComponent {
		t.Error("Expected ts_component fact for App")
	}
	if !hasHook {
		t.Error("Expected ts_hook facts for useState/useEffect")
	}
	if !hasAsync {
		t.Error("Expected ts_async_function fact")
	}
}

// =============================================================================
// Rust Parser Tests
// =============================================================================

// TestRustCodeParser_Parse tests Rust source file parsing.
func TestRustCodeParser_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	rsFile := filepath.Join(tmpDir, "lib.rs")
	rsContent := `use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct User {
    pub id: u64,
    #[serde(rename = "user_name")]
    pub name: String,
}

pub enum Status {
    Active,
    Inactive,
}

pub trait Repository {
    fn find(&self, id: u64) -> Option<User>;
    fn save(&mut self, user: User) -> Result<(), Error>;
}

impl User {
    pub fn new(id: u64, name: String) -> Self {
        Self { id, name }
    }

    pub async fn fetch_profile(&self) -> Result<Profile, Error> {
        api::get_profile(self.id).await
    }
}

pub fn validate_user(user: &User) -> bool {
    !user.name.is_empty()
}

fn private_helper() {
    // internal use
}

const MAX_USERS: usize = 1000;
`
	if err := os.WriteFile(rsFile, []byte(rsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewRustCodeParser(tmpDir)

	// Test SupportedExtensions
	exts := parser.SupportedExtensions()
	if len(exts) != 1 || exts[0] != ".rs" {
		t.Errorf("Expected [.rs], got %v", exts)
	}

	// Test Language
	if parser.Language() != "rs" {
		t.Errorf("Expected 'rs', got %s", parser.Language())
	}

	// Test Parse
	content, _ := os.ReadFile(rsFile)
	elements, err := parser.Parse(rsFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should find: User struct, Status enum, Repository trait, impl methods, functions, const
	if len(elements) < 6 {
		t.Errorf("Expected at least 6 elements, got %d", len(elements))
	}

	var foundStruct, foundEnum, foundTrait, foundMethod, foundFunc, foundConst, foundPrivate bool
	for _, elem := range elements {
		switch elem.Name {
		case "User":
			foundStruct = true
			if elem.Type != ElementStruct {
				t.Error("User should be ElementStruct")
			}
			if elem.Visibility != VisibilityPublic {
				t.Error("User should be public")
			}
		case "Status":
			foundEnum = true
			if elem.Type != ElementType_ {
				t.Error("Status should be ElementType_")
			}
		case "Repository":
			foundTrait = true
			if elem.Type != ElementInterface {
				t.Error("Repository should be ElementInterface")
			}
		case "new":
			foundMethod = true
			if elem.Type != ElementMethod {
				t.Error("new should be ElementMethod")
			}
		case "validate_user":
			foundFunc = true
			if elem.Type != ElementFunction {
				t.Error("validate_user should be ElementFunction")
			}
		case "MAX_USERS":
			foundConst = true
			if elem.Type != ElementConst {
				t.Error("MAX_USERS should be ElementConst")
			}
		case "private_helper":
			foundPrivate = true
			if elem.Visibility != VisibilityPrivate {
				t.Error("private_helper should be private")
			}
		}
	}

	if !foundStruct {
		t.Error("Did not find User struct")
	}
	if !foundEnum {
		t.Error("Did not find Status enum")
	}
	if !foundTrait {
		t.Error("Did not find Repository trait")
	}
	if !foundMethod {
		t.Error("Did not find impl methods")
	}
	if !foundFunc {
		t.Error("Did not find validate_user function")
	}
	if !foundConst {
		t.Error("Did not find MAX_USERS const")
	}
	if !foundPrivate {
		t.Error("Did not find private_helper function")
	}
}

// TestRustCodeParser_EmitLanguageFacts tests Rust fact emission.
func TestRustCodeParser_EmitLanguageFacts(t *testing.T) {
	tmpDir := t.TempDir()
	rsFile := filepath.Join(tmpDir, "api.rs")
	rsContent := `#[derive(Debug, Serialize)]
pub struct Response {
    data: Vec<u8>,
}

pub async fn fetch_data() -> Result<Response, Error> {
    let data = api::get().await?;
    Ok(Response { data })
}

fn dangerous_operation() {
    unsafe {
        ptr::write(addr, value);
    }
}

fn risky_unwrap() {
    let value = some_option.unwrap();
}
`
	if err := os.WriteFile(rsFile, []byte(rsContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	parser := NewRustCodeParser(tmpDir)
	content, _ := os.ReadFile(rsFile)
	elements, err := parser.Parse(rsFile, content)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	facts := parser.EmitLanguageFacts(elements)

	var hasStruct, hasDerive, hasAsync, hasUnsafe, hasUnwrap, hasResult bool
	for _, f := range facts {
		switch f.Predicate {
		case "rs_struct":
			hasStruct = true
		case "rs_derive":
			hasDerive = true
		case "rs_async_fn":
			hasAsync = true
		case "rs_unsafe_block":
			hasUnsafe = true
		case "rs_uses_unwrap":
			hasUnwrap = true
		case "rs_returns_result":
			hasResult = true
		}
	}

	if !hasStruct {
		t.Error("Expected rs_struct fact")
	}
	if !hasDerive {
		t.Error("Expected rs_derive facts")
	}
	if !hasAsync {
		t.Error("Expected rs_async_fn fact")
	}
	if !hasUnsafe {
		t.Error("Expected rs_unsafe_block fact")
	}
	if !hasUnwrap {
		t.Error("Expected rs_uses_unwrap fact")
	}
	if !hasResult {
		t.Error("Expected rs_returns_result fact")
	}
}

// =============================================================================
// Parser Factory Polyglot Tests
// =============================================================================

// TestParserFactory_PolyglotParsing tests factory-based polyglot parsing.
func TestParserFactory_PolyglotParsing(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files in different languages
	files := map[string]string{
		"main.go": `package main
func main() { println("Go") }`,
		"app.py": `def main():
    print("Python")`,
		"lib.ts": `export function main(): void { console.log("TypeScript"); }`,
		"mod.rs": `pub fn main() { println!("Rust"); }`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", name, err)
		}
	}

	factory := DefaultParserFactory(tmpDir)

	// Test that factory has parsers for all languages
	for filename := range files {
		if !factory.HasParser(filename) {
			t.Errorf("Factory should have parser for %s", filename)
		}
	}

	// Test parsing each file
	for filename := range files {
		path := filepath.Join(tmpDir, filename)
		content, _ := os.ReadFile(path)
		elements, err := factory.Parse(path, content)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", filename, err)
		}
		if len(elements) == 0 {
			t.Errorf("Expected elements from %s, got none", filename)
		}
	}
}

// TestParserFactory_EmitAllLanguageFacts tests fact emission across languages.
func TestParserFactory_EmitAllLanguageFacts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files that will emit facts
	goFile := filepath.Join(tmpDir, "model.go")
	goContent := `package model
type User struct {
    ID int ` + "`json:\"user_id\"`" + `
}`
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatal(err)
	}

	pyFile := filepath.Join(tmpDir, "model.py")
	pyContent := `from pydantic import BaseModel
class User(BaseModel):
    user_id: int`
	if err := os.WriteFile(pyFile, []byte(pyContent), 0644); err != nil {
		t.Fatal(err)
	}

	tsFile := filepath.Join(tmpDir, "model.ts")
	tsContent := `export interface IUser {
  user_id: number;
}`
	if err := os.WriteFile(tsFile, []byte(tsContent), 0644); err != nil {
		t.Fatal(err)
	}

	rsFile := filepath.Join(tmpDir, "model.rs")
	rsContent := `#[derive(Serialize)]
pub struct User {
    #[serde(rename = "user_id")]
    pub id: u64,
}`
	if err := os.WriteFile(rsFile, []byte(rsContent), 0644); err != nil {
		t.Fatal(err)
	}

	factory := DefaultParserFactory(tmpDir)

	// Parse and emit facts for each language
	predicateCounts := make(map[string]int)

	testFiles := []string{goFile, pyFile, tsFile, rsFile}
	for _, path := range testFiles {
		content, _ := os.ReadFile(path)
		elements, err := factory.Parse(path, content)
		if err != nil {
			t.Errorf("Failed to parse %s: %v", path, err)
			continue
		}

		parser := factory.GetParser(path)
		if parser == nil {
			continue
		}

		facts := parser.EmitLanguageFacts(elements)
		for _, f := range facts {
			predicateCounts[f.Predicate]++
		}
	}

	// Verify we got facts from each language
	expectedPredicates := []string{
		"go_struct", "go_tag",
		"py_class", "has_pydantic_base",
		"ts_interface", "ts_interface_prop",
		"rs_struct", "rs_derive",
	}

	for _, pred := range expectedPredicates {
		if predicateCounts[pred] == 0 {
			t.Errorf("Expected predicate %s, but none found", pred)
		}
	}
}
