package world

import (
	"codenerd/internal/core"
)

// CodeParser defines the contract for language-specific code element parsers.
// This interface enables polyglot CodeDOM by allowing different languages
// to be parsed using language-appropriate techniques (go/ast, Tree-sitter, etc.)
// while emitting a unified CodeElement representation.
//
// The Stratified Bridge Pattern:
//   - Parse() returns language-agnostic CodeElements (common structure)
//   - EmitLanguageFacts() returns language-specific Mangle facts (Stratum 0)
//   - Bridge rules in Mangle normalize these to semantic archetypes (Stratum 1)
type CodeParser interface {
	// Parse extracts CodeElements from source content.
	// The path is used for generating stable Ref URIs and error messages.
	// Content is the raw file bytes (allows parsing in-memory content).
	//
	// Returns a slice of CodeElements with:
	//   - Ref: Repo-anchored URI (e.g., "go:internal/auth/user.go:User.Login")
	//   - Type: Semantic type (/function, /method, /struct, etc.)
	//   - StartLine/EndLine: 1-indexed, inclusive
	//   - Signature: Declaration line
	//   - Body: Full source text (optional)
	//   - Parent: Ref of containing element (for methods)
	//   - Visibility: /public or /private
	Parse(path string, content []byte) ([]CodeElement, error)

	// SupportedExtensions returns the file extensions this parser handles.
	// Extensions should include the leading dot (e.g., ".go", ".py", ".ts").
	// The first extension is considered the primary/canonical extension.
	SupportedExtensions() []string

	// EmitLanguageFacts generates language-specific Mangle facts (Stratum 0 EDB).
	// These facts capture language-specific semantics that bridge rules
	// can normalize into semantic archetypes.
	//
	// Examples:
	//   - Go: go_struct(Ref), go_tag(Ref, TagContent), go_goroutine(Ref)
	//   - Python: py_class(Ref), py_decorator(Ref, Name), py_async_def(Ref)
	//   - TypeScript: ts_interface(Ref), ts_component(Ref, TagName)
	//   - Rust: rs_struct(Ref), rs_impl(Ref, Trait), rs_unsafe_block(Ref)
	//
	// These facts enable cross-language reasoning via bridge rules like:
	//   is_data_contract(Ref) :- go_struct(Ref).
	//   is_data_contract(Ref) :- py_class(Ref), py_decorator(Ref, "dataclass").
	EmitLanguageFacts(elements []CodeElement) []core.Fact

	// Language returns the language identifier used in Ref URIs.
	// This should be a short lowercase identifier (e.g., "go", "py", "ts", "rs").
	Language() string
}

// LanguageMetadata provides additional language-specific information
// that can be stored in CodeElement.Metadata for advanced analysis.
type LanguageMetadata struct {
	// Decorators lists decorators/annotations on the element (Python, Java, Kotlin)
	Decorators []string `json:"decorators,omitempty"`

	// StructTags contains Go struct field tags
	StructTags map[string]string `json:"struct_tags,omitempty"`

	// IsAsync indicates the element is async (Python async def, Rust async fn)
	IsAsync bool `json:"is_async,omitempty"`

	// IsGeneric indicates the element uses generics/type parameters
	IsGeneric bool `json:"is_generic,omitempty"`

	// Implements lists interfaces/traits this element implements
	Implements []string `json:"implements,omitempty"`

	// Extends lists parent classes/structs (for inheritance)
	Extends []string `json:"extends,omitempty"`

	// ReceiverType is the receiver type for methods (Go)
	ReceiverType string `json:"receiver_type,omitempty"`

	// IsPointerReceiver indicates pointer vs value receiver (Go)
	IsPointerReceiver bool `json:"is_pointer_receiver,omitempty"`

	// Unsafe indicates the element contains unsafe code (Rust, Go CGo)
	Unsafe bool `json:"unsafe,omitempty"`

	// WireName is the serialization name (JSON tag, Pydantic alias, etc.)
	WireName string `json:"wire_name,omitempty"`
}

// ParseResult wraps the output of a parse operation with metadata.
type ParseResult struct {
	// Elements are the extracted code elements
	Elements []CodeElement

	// LanguageFacts are the Stratum 0 facts for this file
	LanguageFacts []core.Fact

	// Patterns are detected code patterns (generated, CGo, build tags)
	Patterns CodePatterns

	// Errors contains non-fatal parse warnings
	Errors []ParseError
}

// ParseError represents a non-fatal parsing issue.
type ParseError struct {
	Line    int
	Column  int
	Message string
}
