package specs

// Relation is how a spec section and a code element are related.
type Relation string

const (
	// RelCites: the section's prose mentions the element.
	RelCites Relation = "cites"
	// RelSpecifies: the element's doc comment names the section as its spec.
	RelSpecifies Relation = "specifies"
	// RelImplements: the section carries an explicit implements annotation.
	RelImplements Relation = "implements"
)

// Source is where a link was read from.
type Source string

const (
	SrcDocCitation Source = "doc_citation"
	SrcCodeComment Source = "code_comment"
	SrcAnnotation  Source = "annotation"
)

// Element is one code element as the pairing layer sees it: a row of the
// structure index, flattened.
type Element struct {
	// Ref is the element's workspace address ("internal/core.RealKernel.Evaluate",
	// "internal/core/defaults/policy/impact.mg:decl:impact_caller/2").
	Ref string
	// Key is the element's address within its file ("RealKernel.Evaluate").
	Key string
	// ID is the call graph's identity (pkg.Recv.Name), a second spelling a
	// citation may use.
	ID       string
	Name     string
	Receiver string
	// Kind is the codemodel kind: function, method, struct, interface, type,
	// const, var, header, decl, rule...
	Kind string
	File string
	// Dir is the package directory ("internal/core"); "." for the root.
	Dir      string
	Exported bool
	// HasDoc reports a doc comment on the element.
	HasDoc bool
	// Test reports an element in a test file.
	Test bool
	// Fingerprint is what the element says (codemodel.Fingerprint of its span,
	// leading comment included): whitespace-only edits and edits to a
	// neighbouring element leave it alone. A header's is the whole file's.
	Fingerprint string
}

// Section is one Markdown section, as a spec.
type Section struct {
	// Ref is "Docs/x.md:Install > Linux".
	Ref string
	// File is the document's workspace path.
	File string
	// Key is the heading path ("Install > Linux", "Install#2 > Linux").
	Key string
	// Heading is the section's own heading text.
	Heading string
	Level   int
	// Revision is the hash of the section's bytes.
	Revision string
	// Fingerprint is what the section says (codemodel.Fingerprint of its own
	// text after the heading, code blocks included, nested sections not):
	// reflowing a paragraph does not move it, rewording does, and an edit to a
	// sub-section is that sub-section's.
	Fingerprint string
}

// Link is one edge between a spec section and a code element.
type Link struct {
	// SpecRef is a section ref. For a code comment that names a section that
	// does not exist it is the ref as the comment spelled it.
	SpecRef string
	// CodeRef is an element ref. For a file-qualified citation whose element is
	// gone it is the ref as the citation spelled it ("path:Symbol").
	CodeRef  string
	Relation Relation
	Source   Source
}

// Problem is a citation that was read and refused.
type Problem struct {
	// Where is the ref of the section, or of the element, holding the citation.
	Where string
	// Text is the citation as written.
	Text string
	// Reason says why it made no link.
	Reason string
}

// Property names what a requirement may demand of an element: the
// closed set the kernel can check against the code model.
type Property string

const (
	PropExists        Property = "exists"
	PropExported      Property = "exported"
	PropHasTest       Property = "has_test"
	PropHasDocComment Property = "has_doc_comment"
	// PropCalls is checked through requires_calls, which carries the callee.
	PropCalls Property = "calls"
)

// Properties is the closed set a requires/3 fact may name.
var Properties = []Property{PropExists, PropExported, PropHasTest, PropHasDocComment}
