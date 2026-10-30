// Package shell provides modular shell execution tools for the JIT Clean Loop.
//
// These tools wrap command execution and make them available
// to any agent based on intent-driven JIT selection.
//
// Tools:
//   - run_command: Execute a shell command
//   - bash: Execute a bash script
//   - run_build: Execute project build command
//   - run_tests: Execute project test command
//   - run_check: Run the campaign's acceptance check
//   - run_lint: Run go vet, staticcheck or golangci-lint and return typed findings
//   - regenerate: Run a generator the workspace's nerd.md declares; its declared outputs are the writes
//   - git_diff: Show a diff for files, staged changes, or a commit range
//   - git_log: Show commit history, optionally filtered by path/author/date
//   - git_operation: Run a whitelisted git operation (status, add, commit, ...)
//
// Every tool here contains working_dir to the workspace root; an omitted
// working_dir means the workspace root, not the process working directory.
package shell
