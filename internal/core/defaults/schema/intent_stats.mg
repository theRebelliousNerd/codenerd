# Intent Definitions - Codebase Statistics (/stats)
# SECTION 1: CODEBASE STATISTICS - DIRECT RESPONSE, NO SHARD
# These queries about codebase metrics should be answered directly using
# shell commands or the knowledge database - NOT by spawning shards.

# --- File Type Breakdown ---
intent_definition("What is the breakdown of file types?", /stats, "file_types").
intent_category("What is the breakdown of file types?", /query).

intent_definition("what is the breakdown of file types", /stats, "file_types").
intent_category("what is the breakdown of file types", /query).

intent_definition("How many files of each type are there?", /stats, "file_types").
intent_category("How many files of each type are there?", /query).

intent_definition("Show me the file type distribution.", /stats, "file_types").
intent_category("Show me the file type distribution.", /query).

intent_definition("What file types are in this codebase?", /stats, "file_types").
intent_category("What file types are in this codebase?", /query).

intent_definition("Count files by extension.", /stats, "file_types").
intent_category("Count files by extension.", /query).

intent_definition("What extensions are used in this project?", /stats, "file_types").
intent_category("What extensions are used in this project?", /query).

intent_definition("File type summary.", /stats, "file_types").
intent_category("File type summary.", /query).

intent_definition("What kinds of files are here?", /stats, "file_types").
intent_category("What kinds of files are here?", /query).

intent_definition("Show file extensions.", /stats, "file_types").
intent_category("Show file extensions.", /query).

intent_definition("List all file types.", /stats, "file_types").
intent_category("List all file types.", /query).

intent_definition("How many .go files vs .md files?", /stats, "file_types").
intent_category("How many .go files vs .md files?", /query).

intent_definition("What's the ratio of code to documentation?", /stats, "file_types").
intent_category("What's the ratio of code to documentation?", /query).

intent_definition("Break down the codebase by language.", /stats, "file_types").
intent_category("Break down the codebase by language.", /query).

intent_definition("What languages are used?", /stats, "file_types").
intent_category("What languages are used?", /query).

# --- File Counts ---
intent_definition("How many Go files are there?", /stats, "go_files").
intent_category("How many Go files are there?", /query).

intent_definition("How many files are in this project?", /stats, "file_count").
intent_category("How many files are in this project?", /query).

intent_definition("Count the files in the codebase.", /stats, "file_count").
intent_category("Count the files in the codebase.", /query).

intent_definition("How many markdown files?", /stats, "md_files").
intent_category("How many markdown files?", /query).

intent_definition("How many test files are there?", /stats, "test_files").
intent_category("How many test files are there?", /query).

intent_definition("Total file count.", /stats, "file_count").
intent_category("Total file count.", /query).

intent_definition("How big is this codebase?", /stats, "file_count").
intent_category("How big is this codebase?", /query).

intent_definition("Number of source files.", /stats, "file_count").
intent_category("Number of source files.", /query).

intent_definition("How many Python files?", /stats, "py_files").
intent_category("How many Python files?", /query).

intent_definition("How many TypeScript files?", /stats, "ts_files").
intent_category("How many TypeScript files?", /query).

intent_definition("How many JavaScript files?", /stats, "js_files").
intent_category("How many JavaScript files?", /query).

intent_definition("How many Rust files?", /stats, "rs_files").
intent_category("How many Rust files?", /query).

intent_definition("How many YAML files?", /stats, "yaml_files").
intent_category("How many YAML files?", /query).

intent_definition("How many JSON files?", /stats, "json_files").
intent_category("How many JSON files?", /query).

intent_definition("How many config files?", /stats, "config_files").
intent_category("How many config files?", /query).

intent_definition("Count Go files.", /stats, "go_files").
intent_category("Count Go files.", /query).

intent_definition("Count all files.", /stats, "file_count").
intent_category("Count all files.", /query).

intent_definition("How many Mangle files?", /stats, "mg_files").
intent_category("How many Mangle files?", /query).

intent_definition("How many .mg files?", /stats, "mg_files").
intent_category("How many .mg files?", /query).

# --- Lines of Code ---
intent_definition("How many lines of code?", /stats, "loc").
intent_category("How many lines of code?", /query).

intent_definition("What is the total LOC?", /stats, "loc").
intent_category("What is the total LOC?", /query).

intent_definition("Count the lines of code.", /stats, "loc").
intent_category("Count the lines of code.", /query).

intent_definition("How many lines in total?", /stats, "loc").
intent_category("How many lines in total?", /query).

intent_definition("LOC count.", /stats, "loc").
intent_category("LOC count.", /query).

intent_definition("Total lines.", /stats, "loc").
intent_category("Total lines.", /query).

intent_definition("How much code is there?", /stats, "loc").
intent_category("How much code is there?", /query).

intent_definition("Lines of Go code.", /stats, "loc_go").
intent_category("Lines of Go code.", /query).

intent_definition("How many lines of Go?", /stats, "loc_go").
intent_category("How many lines of Go?", /query).

intent_definition("Code size.", /stats, "loc").
intent_category("Code size.", /query).

intent_definition("Codebase size.", /stats, "loc").
intent_category("Codebase size.", /query).

intent_definition("How large is the codebase?", /stats, "loc").
intent_category("How large is the codebase?", /query).

# --- Project Structure ---
intent_definition("What is the project structure?", /stats, "structure").
intent_category("What is the project structure?", /query).

intent_definition("Show me the directory structure.", /stats, "structure").
intent_category("Show me the directory structure.", /query).

intent_definition("What directories are in this project?", /stats, "structure").
intent_category("What directories are in this project?", /query).

intent_definition("List the main folders.", /stats, "structure").
intent_category("List the main folders.", /query).

intent_definition("Project layout.", /stats, "structure").
intent_category("Project layout.", /query).

intent_definition("Folder structure.", /stats, "structure").
intent_category("Folder structure.", /query).

intent_definition("Directory tree.", /stats, "structure").
intent_category("Directory tree.", /query).

intent_definition("Show the tree.", /stats, "structure").
intent_category("Show the tree.", /query).

intent_definition("What's in the root directory?", /stats, "structure").
intent_category("What's in the root directory?", /query).

intent_definition("List top-level directories.", /stats, "structure").
intent_category("List top-level directories.", /query).

intent_definition("Show project hierarchy.", /stats, "structure").
intent_category("Show project hierarchy.", /query).

intent_definition("How is the code organized?", /stats, "structure").
intent_category("How is the code organized?", /query).

intent_definition("Show me the architecture.", /stats, "structure").
intent_category("Show me the architecture.", /query).

intent_definition("What packages exist?", /stats, "structure").
intent_category("What packages exist?", /query).

intent_definition("List all packages.", /stats, "structure").
intent_category("List all packages.", /query).

intent_definition("What modules are there?", /stats, "structure").
intent_category("What modules are there?", /query).

intent_definition("Show the module structure.", /stats, "structure").
intent_category("Show the module structure.", /query).

intent_definition("What's in internal/?", /stats, "structure").
intent_category("What's in internal/?", /query).

intent_definition("What's in cmd/?", /stats, "structure").
intent_category("What's in cmd/?", /query).

# --- Codebase Overview ---
intent_definition("Give me an overview of the codebase.", /stats, "overview").
intent_category("Give me an overview of the codebase.", /query).

intent_definition("Summarize this project.", /stats, "overview").
intent_category("Summarize this project.", /query).

intent_definition("What is this codebase about?", /stats, "overview").
intent_category("What is this codebase about?", /query).

intent_definition("Describe the project.", /stats, "overview").
intent_category("Describe the project.", /query).

intent_definition("Project summary.", /stats, "overview").
intent_category("Project summary.", /query).

intent_definition("What does this project do?", /stats, "overview").
intent_category("What does this project do?", /query).

intent_definition("Tell me about this codebase.", /stats, "overview").
intent_category("Tell me about this codebase.", /query).

intent_definition("What am I looking at?", /stats, "overview").
intent_category("What am I looking at?", /query).

intent_definition("Codebase summary.", /stats, "overview").
intent_category("Codebase summary.", /query).

intent_definition("High-level overview.", /stats, "overview").
intent_category("High-level overview.", /query).

intent_definition("What's the purpose of this project?", /stats, "overview").
intent_category("What's the purpose of this project?", /query).

# --- Dependency Stats ---
intent_definition("How many dependencies?", /stats, "deps").
intent_category("How many dependencies?", /query).

intent_definition("List dependencies.", /stats, "deps").
intent_category("List dependencies.", /query).

intent_definition("What packages are imported?", /stats, "deps").
intent_category("What packages are imported?", /query).

intent_definition("Show go.mod dependencies.", /stats, "deps").
intent_category("Show go.mod dependencies.", /query).

intent_definition("External dependencies.", /stats, "deps").
intent_category("External dependencies.", /query).

intent_definition("Third-party packages.", /stats, "deps").
intent_category("Third-party packages.", /query).

# --- Test Stats ---
intent_definition("How many tests?", /stats, "test_count").
intent_category("How many tests?", /query).

intent_definition("Test count.", /stats, "test_count").
intent_category("Test count.", /query).

intent_definition("Number of test functions.", /stats, "test_count").
intent_category("Number of test functions.", /query).

intent_definition("How many _test.go files?", /stats, "test_files").
intent_category("How many _test.go files?", /query).

intent_definition("Test file count.", /stats, "test_files").
intent_category("Test file count.", /query).

# --- Function/Symbol Stats ---
intent_definition("How many functions?", /stats, "func_count").
intent_category("How many functions?", /query).

intent_definition("Function count.", /stats, "func_count").
intent_category("Function count.", /query).

intent_definition("How many types are defined?", /stats, "type_count").
intent_category("How many types are defined?", /query).

intent_definition("How many structs?", /stats, "struct_count").
intent_category("How many structs?", /query).

intent_definition("How many interfaces?", /stats, "interface_count").
intent_category("How many interfaces?", /query).
