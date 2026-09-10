// Package core provides modular core filesystem tools for the JIT Clean Loop.
//
// These tools wrap VirtualStore's file operations and make them available
// to any agent based on intent-driven JIT selection.
//
// Tools:
//   - read_file: read the region around an edit, an outline of the rest, and a
//     precondition handle that lets a later edit be refused if the file moved
//   - write_file: Write content to a file
//   - edit_file: Edit file with replacements, optionally under a read precondition
//   - list_files: List directory contents
//   - glob: Find files matching a pattern
//   - grep: Search file contents with regex
//   - search_code: search shaped to symbols and dependency edges, not lines
//   - search_expand: read the raw lines behind a search_code handle
//   - delete_file: Delete a file (requires permission)
package core
