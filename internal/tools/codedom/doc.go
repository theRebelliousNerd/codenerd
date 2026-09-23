// Package codedom provides modular Code DOM tools for the JIT Clean Loop.
//
// These tools make the element model (internal/world/codemodel) the view of
// code: declarations, not lines, are what is read, searched and edited.
//
// Tools:
//   - get_elements: Every element of a file with its span, kind, ref and rev
//   - get_element: An element's source with its doc comment, by ref
//   - find_symbol: Declarations by name, several names, pattern, kind and path
//   - package_outline: Every declaration in a directory or file, with line spans
//   - callers_of: Every call site of a function or method
//   - callees_of: Every call made inside a function or method
//   - unreferenced_symbols: Declarations nothing else in the workspace names
//   - importers_of: Every file importing a package
//   - find_text: Text in string literals, comments or identifiers, answered as element refs
//   - predicate_outline: Where a Mangle predicate is declared, derived and read
//   - edit_element: Replace text inside one element, anchored uniquely within it
//   - replace_element: Replace one whole element, doc comment included
//   - insert_element: Insert declarations before or after an element, the header or the end
//   - delete_element: Delete an element nothing else uses, or repoint its uses in the same call
//   - create_file: Create a Go or Mangle file validated as a unit, imports derived
//   - repoint: Rewrite every use of a package-level name to another, in one transaction
//   - edit_lines: Replace specific lines in a file
//   - insert_lines: Insert lines at a position
//   - delete_lines: Delete a range of lines
//   - apply_edits: Transactionally apply 2-16 edits across distinct existing files
//   - run_impacted_tests: Run tests affected by recent edits
//   - get_impacted_tests: Query impacted tests without running
package codedom
