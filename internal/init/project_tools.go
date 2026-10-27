package init

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"codenerd/internal/atomicfile"
	"codenerd/internal/perception"
)

// Phase 7e: project command tools.
//
// Phase 10 writes the language's generic tools (go test, npm run build, ...)
// to available_tools.json, which boot registers as static command tools. What
// it cannot know is the project's own vocabulary: the Makefile target that
// regenerates protobufs, the package script that runs the integration suite.
// Those are declared in the repository, so they are measured here, and the
// planner chooses which of them an agent should be able to run. The planner
// chooses among declared commands only; anything else it names is dropped.
//
// Until 2026-10-27 this phase recorded a fixed per-language list (a
// golangci-lint wrapper whether or not the project used golangci) as
// missing_tool_for(/project_init, Cap) facts in profile.mg. autopoiesis.mg
// and delegation.mg treat any missing_tool_for row as a standing order to
// generate the tool, so every interactive boot ran Ouroboros code generation
// for wrappers around commands phase 10 already registers.

// maxProjectToolCandidates bounds the commands offered to the planner in one
// request, and maxProjectRecipeRunes bounds each command's recipe excerpt. A
// repository that declares more is told so in the init warnings.
const (
	maxProjectToolCandidates = 80
	maxProjectRecipeRunes    = 400
)

// projectToolCategories are the categories a project command tool may take.
var projectToolCategories = []string{"build", "test", "lint", "format", "generate", "run", "deps"}

// projectCommand is one command the repository itself declares.
type projectCommand struct {
	Command string `json:"command"`
	Source  string `json:"source"`
	Recipe  string `json:"recipe,omitempty"`
}

// projectToolSelection is phase 7e's outcome.
type projectToolSelection struct {
	candidates []projectCommand
	tools      []ToolDefinition
	source     string
	notes      []string
}

// projectToolsRecord is .nerd/tools/project_tools.json: the last selection
// and the digest of the commands it was chosen from, so a re-init whose
// repository declares the same commands reuses it instead of asking again.
type projectToolsRecord struct {
	CandidatesDigest string           `json:"candidates_digest"`
	ChosenBy         string           `json:"chosen_by"`
	Tools            []ToolDefinition `json:"tools"`
}

func projectToolsRecordPath(nerdDir string) string {
	return filepath.Join(nerdDir, "tools", "project_tools.json")
}

// selectProjectTools measures the declared commands and chooses the project
// command tools: reused from the last init when the commands are unchanged,
// otherwise chosen by the planner route.
func (i *Initializer) selectProjectTools(ctx context.Context, nerdDir string, profile ProjectProfile) (projectToolSelection, error) {
	var selection projectToolSelection
	candidates, total, err := declaredProjectCommands(i.config.Workspace)
	if err != nil {
		return selection, err
	}
	selection.candidates = candidates
	if total > len(candidates) {
		selection.notes = append(selection.notes, fmt.Sprintf(
			"The repository declares %d commands; the first %d (by file and name) were offered for project tool selection",
			total, len(candidates)))
	}
	if len(candidates) == 0 {
		return selection, nil
	}

	digest, err := projectCommandsDigest(candidates)
	if err != nil {
		return selection, err
	}
	previous, prevErr := loadProjectToolsRecord(nerdDir)
	if prevErr != nil {
		selection.notes = append(selection.notes, prevErr.Error())
	}
	if previous != nil && previous.CandidatesDigest == digest {
		selection.tools = previous.Tools
		selection.source = "declared commands unchanged since the last init; chosen by " + previous.ChosenBy
		return selection, nil
	}

	route := i.route(slotPlanner)
	if route.client == nil {
		if previous != nil {
			selection.tools = stillDeclared(previous.Tools, candidates)
			selection.source = "kept from the last init; no LLM client to choose again"
		}
		return selection, fmt.Errorf("no LLM client to choose among %d declared commands", len(candidates))
	}

	tools, rejected, err := i.chooseProjectTools(ctx, profile, candidates)
	if err != nil {
		if previous != nil {
			selection.tools = stillDeclared(previous.Tools, candidates)
			selection.source = "kept from the last init; choosing again failed"
		}
		return selection, err
	}
	if rejected > 0 {
		selection.notes = append(selection.notes, fmt.Sprintf(
			"Project tool selection named %d command(s) the repository does not declare; they were dropped", rejected))
	}
	chosenBy := string(route.slot) + " client " + formatLLMIdentity(route.provider, route.model)
	selection.tools = tools
	selection.source = "chosen by the " + chosenBy
	if err := saveProjectToolsRecord(nerdDir, projectToolsRecord{CandidatesDigest: digest, ChosenBy: chosenBy, Tools: tools}); err != nil {
		selection.notes = append(selection.notes, fmt.Sprintf("Project tool selection was not recorded for the next init: %v", err))
	}
	return selection, nil
}

// chooseProjectTools asks the planner route which declared commands to expose
// and keeps only valid choices of declared commands.
func (i *Initializer) chooseProjectTools(ctx context.Context, profile ProjectProfile, candidates []projectCommand) ([]ToolDefinition, int, error) {
	task := projectToolsTask(profile, candidates)
	response, err := i.withJITPrompt(ctx, initCall{slot: slotPlanner, phase: "project_tools", task: task, profile: &profile},
		func(ctx context.Context, client perception.LLMClient, prompt string) (string, error) {
			return client.Complete(ctx, prompt)
		})
	if err != nil {
		return nil, 0, fmt.Errorf("project tool selection request failed: %w", err)
	}
	return parseProjectToolChoice(response, candidates)
}

func projectToolsTask(profile ProjectProfile, candidates []projectCommand) string {
	var list strings.Builder
	for index, candidate := range candidates {
		fmt.Fprintf(&list, "%d. %s  (declared in %s)\n", index+1, candidate.Command, candidate.Source)
		if candidate.Recipe != "" {
			for _, line := range strings.Split(candidate.Recipe, "\n") {
				fmt.Fprintf(&list, "     | %s\n", line)
			}
		}
	}
	return fmt.Sprintf(`Choose which of this repository's own commands an AI coding agent working in it should be able to run as tools.

Project: language %s, build system %s.

Every candidate is a command the repository declares, with where it is declared and what it runs:
%s
Rules:
- Choose only commands from the list, copied exactly. Never invent, combine or edit a command.
- Choose commands that build, test, lint, format, type-check, generate code, manage dependencies or run the project locally.
- Do not choose commands that deploy, release, publish, push, upload, delete data, or need credentials or a remote service.
- Prefer the command the repository's own developers run; skip near-duplicates.

Respond with JSON only:
{"tools":[{"command":"<exact candidate command>","name":"<short snake_case name>","category":"%s","description":"<one sentence: what the command does in this repository>"}]}
An empty list is a valid answer.`,
		profileValue(profile.Language), profileValue(profile.BuildSystem), list.String(), strings.Join(projectToolCategories, "|"))
}

func profileValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}

// parseProjectToolChoice validates the planner's answer: every tool must name
// a declared command exactly, a known category and a description. It returns
// the accepted tools and how many choices named undeclared commands. A reply
// that is not the requested JSON is an error.
func parseProjectToolChoice(response string, candidates []projectCommand) ([]ToolDefinition, int, error) {
	var reply struct {
		Tools []struct {
			Command     string `json:"command"`
			Name        string `json:"name"`
			Category    string `json:"category"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(extractJSON(response)), &reply); err != nil {
		return nil, 0, fmt.Errorf("project tool selection reply is not the requested JSON: %w", err)
	}
	declared := make(map[string]projectCommand, len(candidates))
	for _, candidate := range candidates {
		declared[candidate.Command] = candidate
	}
	var tools []ToolDefinition
	seenCommand := map[string]bool{}
	seenName := map[string]bool{}
	rejected := 0
	for _, choice := range reply.Tools {
		command := strings.TrimSpace(choice.Command)
		candidate, ok := declared[command]
		if !ok {
			rejected++
			continue
		}
		category := strings.ToLower(strings.TrimSpace(choice.Category))
		description := strings.TrimSpace(choice.Description)
		if seenCommand[command] || !slices.Contains(projectToolCategories, category) || description == "" {
			continue
		}
		name := sanitizeToolName(choice.Name)
		if name == "" {
			name = sanitizeToolName(command)
		}
		name = "project_" + strings.TrimPrefix(name, "project_")
		if seenName[name] {
			continue
		}
		seenCommand[command], seenName[name] = true, true
		tools = append(tools, ToolDefinition{
			Name:          name,
			DisplayName:   command,
			Category:      category,
			Description:   description,
			ShardAffinity: projectToolAffinity(category),
			Conditions:    []string{"declared in " + candidate.Source},
			Command:       command,
			WorkingDir:    ".",
			InputType:     "none",
			OutputType:    "stdout",
		})
	}
	return tools, rejected, nil
}

func projectToolAffinity(category string) string {
	switch category {
	case "test":
		return "TesterShard"
	case "lint":
		return "ReviewerShard"
	default:
		return "CoderShard"
	}
}

// sanitizeToolName renders a name as lower snake_case.
func sanitizeToolName(name string) string {
	var b strings.Builder
	lastUnderscore := true // suppresses a leading underscore
	for _, c := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			b.WriteRune(c)
			lastUnderscore = false
		case !lastUnderscore:
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// stillDeclared keeps the tools whose command the repository still declares.
func stillDeclared(tools []ToolDefinition, candidates []projectCommand) []ToolDefinition {
	declared := map[string]bool{}
	for _, candidate := range candidates {
		declared[candidate.Command] = true
	}
	var kept []ToolDefinition
	for _, tool := range tools {
		if declared[tool.Command] {
			kept = append(kept, tool)
		}
	}
	return kept
}

// mergeProjectTools appends the project command tools to the language tools.
// A project tool whose name or command a language tool already has is the
// same tool and is not added twice.
func mergeProjectTools(languageTools, projectTools []ToolDefinition) []ToolDefinition {
	names := map[string]bool{}
	commands := map[string]bool{}
	for _, tool := range languageTools {
		names[tool.Name] = true
		commands[tool.Command] = true
	}
	merged := append([]ToolDefinition(nil), languageTools...)
	for _, tool := range projectTools {
		if names[tool.Name] || commands[tool.Command] {
			continue
		}
		names[tool.Name], commands[tool.Command] = true, true
		merged = append(merged, tool)
	}
	return merged
}

func projectCommandsDigest(candidates []projectCommand) (string, error) {
	body, err := json.Marshal(candidates)
	if err != nil {
		return "", fmt.Errorf("digest declared commands: %w", err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func loadProjectToolsRecord(nerdDir string) (*projectToolsRecord, error) {
	data, err := os.ReadFile(projectToolsRecordPath(nerdDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read the last project tool selection: %w", err)
	}
	var record projectToolsRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("the last project tool selection is unreadable and is chosen again: %w", err)
	}
	return &record, nil
}

func saveProjectToolsRecord(nerdDir string, record projectToolsRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	path := projectToolsRecordPath(nerdDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return atomicfile.WriteFile(path, append(data, '\n'), 0o644)
}

// declaredProjectCommands returns the commands the workspace root declares in
// its Makefile and package.json scripts, sorted by source, bounded at
// maxProjectToolCandidates, and the total declared.
func declaredProjectCommands(workspace string) ([]projectCommand, int, error) {
	var commands []projectCommand
	makeCommands, err := makefileCommands(workspace)
	if err != nil {
		return nil, 0, err
	}
	commands = append(commands, makeCommands...)
	scriptCommands, err := packageScriptCommands(workspace)
	if err != nil {
		return nil, 0, err
	}
	commands = append(commands, scriptCommands...)
	total := len(commands)
	if len(commands) > maxProjectToolCandidates {
		commands = commands[:maxProjectToolCandidates]
	}
	return commands, total, nil
}

// makeTargetLine matches a rule line: one or more target names, a colon that
// is not part of an assignment (:=, ::=), and the prerequisites.
var makeTargetLine = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9_./+-]*(?:[ \t]+[A-Za-z0-9][A-Za-z0-9_./+-]*)*)[ \t]*:([^:=].*)?$`)

// makefileCommands reads explicit targets from the root Makefile. Special
// targets (.PHONY), pattern rules, variable-named targets and assignments are
// not commands.
func makefileCommands(workspace string) ([]projectCommand, error) {
	var name string
	var file *os.File
	for _, candidate := range []string{"GNUmakefile", "makefile", "Makefile"} {
		f, err := os.Open(filepath.Join(workspace, candidate))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", candidate, err)
		}
		name, file = candidate, f
		break
	}
	if file == nil {
		return nil, nil
	}
	defer file.Close()

	var commands []projectCommand
	index := map[string]int{}
	var current []int // the targets of the rule whose recipe is being read
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") {
			for _, at := range current {
				commands[at].Recipe = appendRecipe(commands[at].Recipe, strings.TrimSpace(line))
			}
			continue
		}
		current = nil
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		match := makeTargetLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		for _, target := range strings.Fields(match[1]) {
			if strings.HasPrefix(target, ".") || strings.ContainsAny(target, "%$") {
				continue
			}
			command := "make " + target
			at, ok := index[command]
			if !ok {
				at = len(commands)
				index[command] = at
				commands = append(commands, projectCommand{Command: command, Source: fmt.Sprintf("%s:%d", name, lineNo)})
			}
			current = append(current, at)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return commands, nil
}

// lifecycleScripts run on their own around install and publish; exposing
// them as tools invites running them out of order.
var lifecycleScripts = map[string]bool{
	"preinstall": true, "install": true, "postinstall": true,
	"prepare": true, "prepublish": true, "prepublishOnly": true, "prepack": true, "postpack": true,
	"preversion": true, "version": true, "postversion": true,
}

// packageScriptCommands reads the root package.json scripts, run with the
// package manager whose lockfile the workspace carries.
func packageScriptCommands(workspace string) ([]projectCommand, error) {
	data, err := os.ReadFile(filepath.Join(workspace, "package.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read package.json: %w", err)
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse package.json scripts: %w", err)
	}
	runner := "npm run"
	for _, lock := range []struct{ file, runner string }{
		{"pnpm-lock.yaml", "pnpm run"}, {"yarn.lock", "yarn run"}, {"bun.lockb", "bun run"}, {"bun.lock", "bun run"},
	} {
		if _, err := os.Stat(filepath.Join(workspace, lock.file)); err == nil {
			runner = lock.runner
			break
		}
	}
	names := make([]string, 0, len(manifest.Scripts))
	for name := range manifest.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	var commands []projectCommand
	for _, name := range names {
		if lifecycleScripts[name] || strings.ContainsAny(name, " \t\n") {
			continue
		}
		if base, ok := strings.CutPrefix(name, "pre"); ok && manifest.Scripts[base] != "" {
			continue
		}
		if base, ok := strings.CutPrefix(name, "post"); ok && manifest.Scripts[base] != "" {
			continue
		}
		commands = append(commands, projectCommand{
			Command: runner + " " + name,
			Source:  "package.json scripts." + name,
			Recipe:  appendRecipe("", manifest.Scripts[name]),
		})
	}
	return commands, nil
}

// appendRecipe adds a line to a recipe excerpt bounded at
// maxProjectRecipeRunes; a cut excerpt ends with an ellipsis.
func appendRecipe(recipe, line string) string {
	if line == "" || strings.HasSuffix(recipe, "…") {
		return recipe
	}
	if recipe != "" {
		recipe += "\n"
	}
	recipe += line
	if runes := []rune(recipe); len(runes) > maxProjectRecipeRunes {
		recipe = string(runes[:maxProjectRecipeRunes]) + "…"
	}
	return recipe
}
