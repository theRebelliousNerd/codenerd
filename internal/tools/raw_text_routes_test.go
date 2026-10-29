package tools_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/world"
)

// rawRoutesBaseline lists every production tool that reaches Go source text
// without an element address. It is a ratchet: it may only shrink.
const rawRoutesBaseline = "testdata/raw_text_routes.txt"

// The probe workspace. RATCHETQUERY is what a search asks for; RATCHETPAYLOAD
// is never passed in any argument, so it appears in a tool's answer only if
// the tool handed back the file's own text.
const (
	probeQuery   = "RATCHETQUERY"
	probePayload = "RATCHETPAYLOAD"
	probeWrite   = "RATCHETWRITE"
	probeDir     = "ratchetpkg"
	probeFile    = "ratchetpkg/holder.go"
	probeSecond  = "ratchetpkg/second.go"
	// probeRef is the element that holds the payload; a tool whose every
	// payload line names it answered by element, not by text.
	probeRef = "ratchetpkg.SentinelHolder"
	// otherRef is the element the cross-element probe addresses: an edit
	// aimed at it must leave the sentinel's element byte-identical.
	otherRef = "ratchetpkg.Other"
)

const probeHolderCommitted = `package ratchetpkg

// SentinelHolder returns the probe's marker.
func SentinelHolder() string {
	// RATCHETPAYLOAD inside a comment
	return "RATCHETQUERY RATCHETPAYLOAD"
}

// Other is the element the cross-element probe addresses.
func Other() int {
	return 1
}
`

// The working copy differs from the commit in the sentinel's comment, so a
// diff has payload lines to show.
var probeHolder = strings.Replace(probeHolderCommitted, "inside a comment", "inside a comment, edited", 1)

// sentinelElement is the element text every probe must leave intact unless
// it reached it by a raw route: its doc comment through its closing brace. The
// blank line after it belongs to no element; deleting Other rightly takes it.
var sentinelElement = func() string {
	start := strings.Index(probeHolder, "// SentinelHolder")
	return probeHolder[start : start+strings.Index(probeHolder[start:], "\n}\n")+3]
}()

const probeSecondSource = `package ratchetpkg

// Second exists so a multi-file edit has a second file.
func Second() int {
	return 2
}
`

// elementAddressArgs are arguments that name an element (or a name the
// structure index resolves) rather than a file, a line or a text pattern.
// The raw probes leave them out: a tool that can still reach the sentinel
// without one is a raw route.
var elementAddressArgs = map[string]bool{
	"ref": true, "refs": true, "anchor": true, "part": true, "revision": true,
	"from": true, "to": true, "replace_with": true,
	"symbol": true, "package": true, "predicate": true, "name": true, "kind": true,
}

// pathArgs mark a tool as path-taking; the probe points them at the sentinel.
var pathArgs = map[string]bool{
	"path": true, "file": true, "file_path": true, "paths": true,
	"base_path": true, "working_dir": true, "repo_root": true, "save_path": true,
}

// freeFormExecArgs are arguments through which an execute tool runs whatever
// the model writes: such a tool reads and writes any file.
var freeFormExecArgs = []string{"command", "script", "args"}

// handleProducers names, for a tool that redeems a handle, the probe that
// produces one.
var handleProducers = map[string]string{
	"search_expand": "search_code",
}

var handleRe = regexp.MustCompile(`handle=(\S+)`)

type routeVerdict struct {
	verdict string // raw-read, raw-write, "raw-read raw-write", exec, element, structural, unprobed-*
	detail  string
}

func (v routeVerdict) raw() bool {
	return strings.HasPrefix(v.verdict, "raw-") || v.verdict == "exec"
}

// TestRawTextRoutes_OnlyShrink is the R8 ratchet. Every tool production
// registers is classified by what it does to a Go file holding a sentinel:
//
//   - raw-read: its answer carries the file's text on a line that names no
//     element (read_file, grep, a diff);
//   - raw-write: it changes the file without an element address, or changes
//     an element other than the one it was addressed to;
//   - exec: an execute tool that runs a free-form command, and so reads and
//     writes anything;
//   - element: it takes an element address, and without one it reaches
//     nothing (its text answers name the element they came from);
//   - structural: it takes a path but answers with rows, names or refusals,
//     never the file's text.
//
// The raw verdicts are the baseline file. A tool that becomes raw, or a new
// raw tool, fails the test; so does a baseline line that is no longer true,
// so the list only ever shrinks toward the CodeDOM-only surface.
func TestRawTextRoutes_OnlyShrink(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH: the probe workspace is a repository so git_diff has a change to show")
	}
	template := probeTemplate(t)

	reg := fullyHydratedRegistry(t)
	names := reg.Names()
	sort.Strings(names)

	got := make(map[string]routeVerdict, len(names))
	for _, name := range names {
		got[name] = classifyTool(t, reg.Get(name), template)
	}

	var table strings.Builder
	for _, name := range names {
		v := got[name]
		fmt.Fprintf(&table, "  %-22s %-22s %s\n", name, v.verdict, v.detail)
	}
	t.Logf("tool surface against a Go sentinel:\n%s", table.String())

	baseline := readRawBaseline(t)
	var failures []string
	for _, name := range names {
		v := got[name]
		want, listed := baseline[name]
		switch {
		case v.raw() && !listed:
			failures = append(failures, fmt.Sprintf("%s is a new raw text route (%s: %s). Make it element-addressed; a new raw route is not added to %s", name, v.verdict, v.detail, rawRoutesBaseline))
		case v.raw() && want != v.verdict:
			failures = append(failures, fmt.Sprintf("%s: baseline says %q, the probe finds %q (%s). If it widened, narrow it; if it narrowed, change its line to %q", name, want, v.verdict, v.detail, name+" "+v.verdict))
		case !v.raw() && listed:
			failures = append(failures, fmt.Sprintf("%s is no longer a raw route (%s). Delete its line from %s: the ratchet only shrinks", name, v.verdict, rawRoutesBaseline))
		}
	}
	for name := range baseline {
		if _, ok := got[name]; !ok {
			failures = append(failures, fmt.Sprintf("%s is in %s but production no longer registers it. Delete its line", name, rawRoutesBaseline))
		}
	}
	sort.Strings(failures)
	for _, f := range failures {
		t.Error(f)
	}
}

func readRawBaseline(t *testing.T) map[string]string {
	t.Helper()
	f, err := os.Open(rawRoutesBaseline)
	if err != nil {
		t.Fatalf("open %s: %v", rawRoutesBaseline, err)
	}
	defer f.Close()
	out := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, verdict, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("%s: %q is not \"<tool> <verdict>\"", rawRoutesBaseline, line)
		}
		out[name] = strings.TrimSpace(verdict)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// probeTemplate builds the sentinel workspace once, as a git repository whose
// working copy differs from its commit; each probe runs on a copy.
func probeTemplate(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module ratchet\n\ngo 1.24\n")
	write(probeFile, probeHolderCommitted)
	write(probeSecond, probeSecondSource)
	for _, args := range [][]string{
		{"init", "-q"},
		// No background maintenance: a detached "git maintenance run --auto"
		// after the commit writes .git/objects while copyWorkspace walks it,
		// and a lock file that vanishes between the listing and the open
		// failed the copy.
		{"config", "maintenance.auto", "false"},
		{"config", "gc.auto", "0"},
		{"-c", "core.autocrlf=false", "add", "-A"},
		{"-c", "user.email=ratchet@example.invalid", "-c", "user.name=ratchet", "commit", "-q", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write(probeFile, probeHolder)
	return root
}

// copyWorkspace copies the template, .git included, to a fresh directory.
func copyWorkspace(t *testing.T, template string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(template, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(template, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy probe workspace: %v", err)
	}
	return dst
}

func classifyTool(t *testing.T, tool *tools.Tool, template string) routeVerdict {
	t.Helper()
	effect, err := tool.DeclaredEffect()
	if err != nil {
		return routeVerdict{verdict: "unprobed-no-effect", detail: err.Error()}
	}
	props := tool.Schema.Properties
	switch effect {
	case tools.EffectExecute:
		for _, arg := range freeFormExecArgs {
			if p, ok := props[arg]; ok && p.Type == "string" {
				return routeVerdict{verdict: "exec", detail: fmt.Sprintf("runs a free-form %q", arg)}
			}
		}
		return routeVerdict{verdict: "unprobed-exec", detail: "fixed command (build/test runner), not probed"}
	case tools.EffectExternal:
		return routeVerdict{verdict: "unprobed-external", detail: "network or browser"}
	}
	if _, ok := props["session_id"]; ok {
		return routeVerdict{verdict: "unprobed-browser", detail: "needs a browser session"}
	}
	pathTaking := false
	addressed := false
	for arg := range props {
		pathTaking = pathTaking || pathArgs[arg]
		addressed = addressed || elementAddressArgs[arg]
	}
	if !pathTaking {
		return routeVerdict{verdict: "structural", detail: "takes no path"}
	}

	var reads, writes []string
	record := func(label string, r probeOutcome) {
		if r.unfillable != "" {
			t.Errorf("%s: the probe cannot fill required argument %q; teach fillProbeArg what it means", tool.Name, r.unfillable)
		}
		if r.readRaw {
			reads = append(reads, label)
		}
		if r.wroteRaw {
			writes = append(writes, label)
		}
	}
	record("path=file", runProbe(t, tool, template, probeFile, ""))
	record("path=dir", runProbe(t, tool, template, probeDir, ""))
	for _, arg := range []string{"ref", "refs", "anchor"} {
		if _, ok := props[arg]; ok {
			record(arg+"="+otherRef, runProbe(t, tool, template, probeFile, arg))
		}
	}

	var parts, details []string
	if len(reads) > 0 {
		parts = append(parts, "raw-read")
		details = append(details, "sentinel text returned ("+strings.Join(reads, ", ")+")")
	}
	if len(writes) > 0 {
		parts = append(parts, "raw-write")
		details = append(details, "sentinel file changed ("+strings.Join(writes, ", ")+")")
	}
	if len(parts) > 0 {
		return routeVerdict{verdict: strings.Join(parts, " "), detail: strings.Join(details, "; ")}
	}
	if addressed {
		return routeVerdict{verdict: "element", detail: "reaches the sentinel only through an element address"}
	}
	return routeVerdict{verdict: "structural", detail: "rows, names or refusals only"}
}

type probeOutcome struct {
	readRaw    bool
	wroteRaw   bool
	unfillable string
}

// runProbe calls the tool once on a fresh copy of the workspace. pathValue is
// what path-like arguments name; crossArg, when set, addresses the tool to
// the element that does NOT hold the sentinel.
func runProbe(t *testing.T, tool *tools.Tool, template, pathValue, crossArg string) probeOutcome {
	t.Helper()
	root := copyWorkspace(t, template)
	codedom.RegisterStructureProvider(world.NewStructureIndex(root).Provider())
	defer codedom.RegisterStructureProvider(nil)
	reg := fullyHydratedRegistry(t)
	reg.SetWorkspaceRoot(root)

	args, unfillable := probeArgs(tool, root, pathValue, crossArg)
	if unfillable != "" {
		return probeOutcome{unfillable: unfillable}
	}
	if producer, ok := handleProducers[tool.Name]; ok {
		res, err := reg.Execute(probeContext(t), producer, map[string]any{"pattern": probeQuery, "path": probeDir})
		if err == nil && res != nil {
			if m := handleRe.FindStringSubmatch(res.Result); m != nil {
				args["handle"] = m[1]
			}
		}
	}

	before := snapshotSources(t, root)
	res, err := reg.Execute(probeContext(t), tool.Name, args)
	var answer strings.Builder
	if res != nil {
		answer.WriteString(res.Result)
		if res.Error != nil {
			answer.WriteString("\n" + res.Error.Error())
		}
	}
	if err != nil {
		answer.WriteString("\n" + err.Error())
	}
	after := snapshotSources(t, root)

	out := probeOutcome{readRaw: payloadWithoutElement(answer.String())}
	if crossArg == "" {
		for rel, data := range before {
			if after[rel] != data {
				out.wroteRaw = true
			}
		}
	} else if !strings.Contains(strings.ReplaceAll(after[probeFile], "\r\n", "\n"), sentinelElement) {
		out.wroteRaw = true
	}
	return out
}

func probeContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return ctx
}

// payloadWithoutElement reports whether some line of the answer carries the
// file's text without naming the element it came from.
func payloadWithoutElement(answer string) bool {
	for _, line := range strings.Split(answer, "\n") {
		if strings.Contains(line, probePayload) && !strings.Contains(line, probeRef) {
			return true
		}
	}
	return false
}

// snapshotSources reads every source file of the probe workspace ("" for a
// file that no longer exists).
func snapshotSources(t *testing.T, root string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, rel := range []string{probeFile, probeSecond} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			out[rel] = ""
			continue
		}
		out[rel] = string(data)
	}
	return out
}

// probeArgs fills every argument the probe understands. Element addresses are
// left out (except the one crossArg names); a required argument the probe
// does not understand is reported, so a new tool cannot slip past unprobed.
func probeArgs(tool *tools.Tool, root, pathValue, crossArg string) (map[string]any, string) {
	args := make(map[string]any)
	required := make(map[string]bool, len(tool.Schema.Required))
	for _, r := range tool.Schema.Required {
		required[r] = true
	}
	for name, prop := range tool.Schema.Properties {
		if name == crossArg {
			if prop.Type == "array" {
				args[name] = []any{otherRef}
			} else {
				args[name] = otherRef
			}
			continue
		}
		if elementAddressArgs[name] {
			continue
		}
		v, ok := fillProbeArg(tool.Name, name, prop, root, pathValue)
		if ok {
			args[name] = v
			continue
		}
		if required[name] {
			return nil, name
		}
	}
	return args, ""
}

func fillProbeArg(toolName, name string, prop tools.Property, root, pathValue string) (any, bool) {
	switch name {
	case "path", "file", "file_path":
		return pathValue, true
	case "paths":
		return []any{pathValue}, true
	case "base_path", "working_dir", "repo_root":
		return root, true
	case "save_path":
		return filepath.Join(root, "probe.out"), true
	case "pattern", "text", "query":
		return probeQuery, true
	case "file_pattern":
		return "*.go", true
	case "old_text", "old":
		return probeQuery, true
	case "new_text", "new", "content", "new_content", "source":
		return probeWrite, true
	case "start_line", "after_line":
		return 1, true
	case "end_line":
		return strings.Count(probeHolder, "\n"), true
	case "position":
		return "after", true
	case "confirmed", "replace_all", "create_dirs":
		return true, true
	case "handle":
		// Filled from the producer's answer when the tool has one; a handle
		// the probe cannot produce leaves the tool unable to reach anything.
		return "", true
	case "edits":
		if toolName != "apply_edits" {
			return nil, false
		}
		return []any{
			map[string]any{"operation": "edit_lines", "path": probeFile, "start_line": 1, "end_line": 1, "new_content": "package ratchetpkg // " + probeWrite},
			map[string]any{"operation": "edit_lines", "path": probeSecond, "start_line": 1, "end_line": 1, "new_content": "package ratchetpkg // " + probeWrite},
		}, true
	}
	if len(prop.Enum) > 0 {
		return prop.Enum[0], true
	}
	return nil, false
}
