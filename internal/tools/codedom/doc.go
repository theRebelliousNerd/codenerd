// Package codedom provides modular Code DOM tools for the JIT Clean Loop.
//
// These tools enable semantic code navigation and editing, working with
// code elements (functions, classes, methods) rather than raw text.
//
// Tools:
//   - get_elements: Query code elements in a file
//   - get_element: Get a specific element by reference
//   - edit_lines: Replace specific lines in a file
//   - insert_lines: Insert lines at a position
//   - delete_lines: Delete a range of lines
//   - apply_edits: Transactionally apply 2-16 edits across distinct existing files
//   - run_impacted_tests: Run tests affected by recent edits
//   - get_impacted_tests: Query impacted tests without running
package codedom
