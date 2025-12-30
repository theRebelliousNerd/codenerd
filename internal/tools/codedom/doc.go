// Package codedom provides modular Code DOM tools for the JIT Clean Loop.
//
// These tools enable semantic code navigation and editing, working with
// code elements (functions, classes, methods) rather than raw text.
//
// Tools:
//   - open_file: Open a file and load its code elements
//   - get_elements: Query code elements in the current scope
//   - get_element: Get a specific element by reference
//   - edit_element: Replace an element's body
//   - insert_lines: Insert lines at a position
//   - delete_lines: Delete a range of lines
//   - refresh_scope: Re-parse after changes
//   - close_scope: Close the current scope
package codedom
