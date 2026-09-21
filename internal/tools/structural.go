package tools

import "strings"

// StructuralNoRows is the line a structural query prints when it found
// nothing. The working loop reads it to count a miss, so the tools and the
// loop share the one spelling.
const StructuralNoRows = "-- 0 rows"

// IsStructuralQuery reports the tools that answer from the world model's
// structure index rather than from raw files.
func IsStructuralQuery(name string) bool {
	switch name {
	case "find_symbol", "package_outline", "callers_of", "callees_of", "unreferenced_symbols",
		"get_element", "get_elements":
		return true
	}
	return false
}

// IsRawSearch reports the search tools the structure index stands in for. They
// are withheld from a working loop until its policy derives working_search_open.
// read_file is not one: it is a targeted source view, and the only way to read
// a file the index has no element model for.
func IsRawSearch(name string) bool {
	switch name {
	case "grep", "glob", "list_files", "search_code":
		return true
	}
	return false
}

// StructuralMissed reports whether a structural query's result was empty.
func StructuralMissed(content string) bool {
	return strings.Contains(content, StructuralNoRows)
}
