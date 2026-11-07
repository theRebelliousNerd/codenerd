// Package specs pairs specifications with code at element granularity.
//
// A specification is a Markdown section: CodeDOM addresses it as
// "Docs/x.md:Install > Linux" (heading path, "#2" for a duplicate). Code is
// addressed by the structure index's element ref
// ("internal/core.RealKernel.Evaluate"). This package finds the edges between
// them, in prose, in doc comments and in explicit annotations, and projects
// them as kernel facts (facts.go) so rules can ask which section specifies an
// element, which specified code has moved since, and which citations name
// nothing any more.
//
// It reads text and a snapshot of the code model; it never walks the
// filesystem and never imports internal/world, which imports it. The world
// scan feeds it (internal/world/spec_facts.go) and refreshes it incrementally.
//
// # Where a link comes from
//
//   - Prose in a section (source /doc_citation, relation /cites): a workspace
//     path (`internal/core/foo.go`, optionally with `:12-20` lines), a path with
//     a symbol (`foo.go:Func`, `foo.go#Func`, or a backticked symbol next to the
//     path), or a backticked identifier that names exactly one element in the
//     workspace. Fenced code blocks are examples, not citations. The lexer is
//     docdecl's, shared with the citation auditor.
//   - A doc comment (source /code_comment, relation /specifies): a line
//     "spec: Docs/x.md#Install > Linux" (the heading path after "#", ":" or a
//     space) says which section specifies the element the comment documents.
//   - An annotation in Markdown (source /annotation, relation /implements):
//     "<!-- implements: internal/core/foo.go:Func, internal/core/bar.go#Type -->"
//     inside a section, for a link prose does not express.
//
// # What is refused
//
// A name that resolves to more than one element is a search, not an address:
// it yields no link and is reported (Result.Problems). A file-qualified
// citation whose symbol is gone, or whose file is gone, yields a link to the
// ref it spelled, which no element carries: the kernel derives
// dangling_spec_link from it. A bare identifier that resolves to nothing is
// not a stale citation (it may not be a citation at all) and yields nothing.
//
// # Machine-checkable specs
//
// A spec file is a Mangle file of ground facts under Docs/specs/ or
// .nerd/specs/ (requirements.go): spec_requirement(Id, Text),
// requires(Id, CodeRef, Property) with Property one of /exists, /exported,
// /has_test, /has_doc_comment, and requires_calls(Id, CodeRef, OtherRef). It
// holds data only: a rule, a Decl or another predicate is refused, so a spec
// file can state obligations but never change policy. The kernel derives
// requirement_violated from them against the measured code model.
package specs
