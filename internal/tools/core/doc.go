// Package core provides modular core filesystem tools for the JIT Clean Loop.
//
// These tools wrap VirtualStore's file operations and make them available
// to any agent based on intent-driven JIT selection.
//
// Tools:
//   - read_file: Read file contents
//   - write_file: Write content to a file
//   - edit_file: Edit file with replacements
//   - list_files: List directory contents
//   - glob: Find files matching a pattern
//   - grep: Search file contents with regex
//   - delete_file: Delete a file (requires permission)
package core
