package projectdoc

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"codenerd/internal/types"
)

// Fact predicates emitted by a nerd.md. Declared in
// internal/core/defaults/schemas_projectdoc.mg; policy over them lives in
// internal/core/defaults/policy/projectdoc.mg.
const (
	// PredPresent records that a nerd.md was loaded, and from where.
	// project_doc(Path, Schema)
	PredPresent = "project_doc"

	// PredName records the declared project name. project_name(Name)
	PredName = "project_name"

	// PredLanguage records the declared primary language, as a Mangle atom
	// (/go, /python). project_language(Lang)
	PredLanguage = "project_language"

	// PredCommand records a canonical invocation.
	// project_command(Kind, Command) where Kind is /build, /test, /lint, /run.
	PredCommand = "project_command"

	// PredCommandEnv records an environment variable those commands require.
	// project_command_env(Name, Value)
	PredCommandEnv = "project_command_env"

	// PredForbiddenPath is the ENFORCED one. project_forbidden_path(Match, Reason)
	PredForbiddenPath = "project_forbidden_path"

	// PredRequirement records a non-negotiable step. project_requirement(Text)
	PredRequirement = "project_requirement"

	// PredConvention records a named project rule. project_convention(ID, Rule)
	PredConvention = "project_convention"

	// PredModuleNorthstar records a module's declared purpose.
	// module_northstar(ModulePath, Purpose)
	PredModuleNorthstar = "module_northstar"

	// PredModuleRequirement records one requirement a module declares.
	// module_requirement(ModulePath, ID, Statement, Severity)
	PredModuleRequirement = "module_requirement"
)

// Facts projects the document's frontmatter into kernel facts.
//
// Only the frontmatter is projected. The Markdown body is prose and belongs in
// the prompt, not the fact store — asserting free text as a fact would invite
// policy to pattern-match natural language, which is the one thing the Mangle
// guardrails explicitly forbid.
//
// Returns nil for a nil document so callers can pass the result of Load
// straight through without a nil check.
func (d *Document) Facts() []types.Fact {
	if d == nil {
		return nil
	}

	facts := []types.Fact{
		{Predicate: PredPresent, Args: []any{d.Path, d.Spec.Schema}},
	}

	if name := strings.TrimSpace(d.Spec.Project); name != "" {
		facts = append(facts, types.Fact{Predicate: PredName, Args: []any{name}})
	}

	if lang := normalizeAtom(d.Spec.Language); lang != "" {
		// A Mangle name constant, not a string: current_context(/lang, /go) and
		// every language-gated atom tag are atoms, and the two types are
		// disjoint in Mangle — a quoted "go" would silently never unify.
		facts = append(facts, types.Fact{Predicate: PredLanguage, Args: []any{types.MangleAtom(lang)}})
	}

	for kind, command := range map[string]string{
		"/build": d.Spec.Commands.Build,
		"/test":  d.Spec.Commands.Test,
		"/lint":  d.Spec.Commands.Lint,
		"/run":   d.Spec.Commands.Run,
	} {
		if strings.TrimSpace(command) == "" {
			continue
		}
		facts = append(facts, types.Fact{
			Predicate: PredCommand,
			Args:      []any{types.MangleAtom(kind), command},
		})
	}

	for name, value := range d.Spec.Commands.Env {
		if strings.TrimSpace(name) == "" {
			continue
		}
		facts = append(facts, types.Fact{Predicate: PredCommandEnv, Args: []any{name, value}})
	}

	for _, rule := range d.Spec.Forbid {
		facts = append(facts, types.Fact{
			Predicate: PredForbiddenPath,
			Args:      []any{rule.Match, rule.Reason},
		})
	}

	for _, req := range d.Spec.Require {
		facts = append(facts, types.Fact{Predicate: PredRequirement, Args: []any{req}})
	}

	for _, c := range d.Spec.Conventions {
		facts = append(facts, types.Fact{Predicate: PredConvention, Args: []any{c.ID, c.Rule}})
	}

	if d.Spec.Northstar != nil {
		modulePath := filepath.ToSlash(filepath.Dir(d.Path))
		if modulePath == "" {
			modulePath = "."
		}
		if purpose := strings.TrimSpace(d.Spec.Northstar.Purpose); purpose != "" {
			facts = append(facts, types.Fact{Predicate: PredModuleNorthstar, Args: []any{modulePath, purpose}})
		}
		for _, req := range d.Spec.Northstar.Requirements {
			sev := strings.TrimSpace(req.Severity)
			var sevAtom types.MangleAtom
			if sev == "" {
				sevAtom = types.MangleAtom("/unspecified")
			} else {
				sevAtom = types.MangleAtom("/" + sev)
			}
			facts = append(facts, types.Fact{Predicate: PredModuleRequirement, Args: []any{modulePath, req.ID, req.Statement, sevAtom}})
		}
	}

	return facts
}

// CommandCount reports how many canonical commands the document declares.
func (d *Document) CommandCount() int {
	if d == nil {
		return 0
	}
	n := 0
	for _, c := range []string{d.Spec.Commands.Build, d.Spec.Commands.Test, d.Spec.Commands.Lint, d.Spec.Commands.Run} {
		if strings.TrimSpace(c) != "" {
			n++
		}
	}
	return n
}

// normalizeAtom converts a user-written tag into Mangle atom form: lowercase,
// slash-prefixed, with anything that is not a name character dropped. Returns ""
// when nothing usable survives, so the caller emits no fact rather than an
// unparseable one.
func normalizeAtom(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "/")))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.WriteByte('/')
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-' || r == ' ' || r == '.':
			b.WriteByte('_')
		}
	}
	if b.Len() == 1 {
		return ""
	}
	return b.String()
}

// PromptSection renders the document for prompt injection.
//
// The frontmatter is restated in prose alongside the body because the model
// cannot read the fact store directly, and a forbidden path it learns about
// only by being denied mid-edit wastes a turn. The enforcement is still the
// kernel's; this is so the model does not have to be surprised by it.
func (d *Document) PromptSection() string {
	if d == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Project Instructions (")
	b.WriteString(d.Path)
	b.WriteString(")\n\n")

	if name := strings.TrimSpace(d.Spec.Project); name != "" {
		b.WriteString("**Project**: ")
		b.WriteString(name)
		b.WriteString("\n\n")
	}

	if c := d.Spec.Commands; c.Build != "" || c.Test != "" || c.Lint != "" || c.Run != "" {
		b.WriteString("### Canonical commands\n\n")
		b.WriteString("Use these exactly. Do not infer a build or test command.\n\n")
		for _, pair := range [][2]string{
			{"build", c.Build}, {"test", c.Test}, {"lint", c.Lint}, {"run", c.Run},
		} {
			if strings.TrimSpace(pair[1]) == "" {
				continue
			}
			b.WriteString("- `")
			b.WriteString(pair[0])
			b.WriteString("`: `")
			b.WriteString(pair[1])
			b.WriteString("`\n")
		}
		if len(c.Env) > 0 {
			b.WriteString("\nRequired environment for those commands:\n")
			for name, value := range c.Env {
				b.WriteString("- `")
				b.WriteString(name)
				b.WriteString("=")
				b.WriteString(value)
				b.WriteString("`\n")
			}
		}
		b.WriteString("\n")
	}

	if len(d.Spec.Forbid) > 0 {
		b.WriteString("### Write-protected paths (ENFORCED)\n\n")
		b.WriteString("These are denied by the kernel before the tool runs, not by your judgement. " +
			"Attempting one costs a turn and changes nothing.\n\n")
		shown := min(len(d.Spec.Forbid), maxPromptListEntries)
		for _, rule := range d.Spec.Forbid[:shown] {
			b.WriteString("- any path containing `")
			b.WriteString(rule.Match)
			b.WriteString("` — ")
			b.WriteString(rule.Reason)
			b.WriteString("\n")
		}
		b.WriteString(listTruncationNotice(shown, len(d.Spec.Forbid), "forbidden paths (still ENFORCED by the kernel)"))
		b.WriteString("\n")
	}

	if len(d.Spec.Require) > 0 {
		b.WriteString("### Required steps\n\n")
		shownReq := min(len(d.Spec.Require), maxPromptListEntries)
		for _, req := range d.Spec.Require[:shownReq] {
			b.WriteString("- ")
			b.WriteString(req)
			b.WriteString("\n")
		}
		b.WriteString(listTruncationNotice(shownReq, len(d.Spec.Require), "required steps"))
		b.WriteString("\n")
	}

	if len(d.Spec.Conventions) > 0 {
		b.WriteString("### Conventions\n\n")
		shownConv := min(len(d.Spec.Conventions), maxPromptListEntries)
		for _, c := range d.Spec.Conventions[:shownConv] {
			b.WriteString("- **")
			b.WriteString(c.ID)
			b.WriteString("**: ")
			b.WriteString(c.Rule)
			b.WriteString("\n")
		}
		b.WriteString(listTruncationNotice(shownConv, len(d.Spec.Conventions), "conventions"))
		b.WriteString("\n")
	}

	if body := strings.TrimSpace(d.Body); body != "" {
		b.WriteString(clampDocText(body, maxPromptBodyChars, "nerd.md body"))
		b.WriteString("\n")
	}

	return clampDocText(b.String(), maxPromptSectionChars, "nerd.md")
}

// Bounds on the rendered project-instruction section.
//
// nerd.md is user content read with an unbounded os.ReadFile and appended to
// the system prompt AFTER the JIT compiler has already fitted and reported its
// budget (session executor, withProjectInstructions). Nothing downstream
// re-measures it, so a 2 MB nerd.md — a generated one, a pasted design doc, a
// vendored style guide — went to the provider whole, on every single turn, and
// the compiler's "42% of budget used" line was simply false.
//
// The cap is generous on purpose. nerd.md is the project's own rules and is
// the LAST thing that should be shed; the point here is a ceiling that stops a
// pathological file, not a budget that shapes a normal one. A well-formed
// nerd.md is a few KB (this repo's is 4.7 KB), so the caps below are ~8x
// typical and will never fire in practice.
const (
	// maxPromptBodyChars caps the advisory Markdown prose (~8k tokens).
	// Head+tail: the top of a project doc states the rules and the bottom is
	// where "one more thing, never touch X" lives.
	maxPromptBodyChars = 32 * 1024

	// maxPromptSectionChars caps the whole rendered section (~12k tokens),
	// including the restated frontmatter, so a nerd.md with thousands of
	// forbid rules cannot route around the body cap.
	maxPromptSectionChars = 48 * 1024

	// maxPromptListEntries caps any one restated frontmatter list. The
	// frontmatter is ENFORCED by the kernel regardless of what is rendered
	// here, so a truncated list costs the model foreknowledge, never
	// protection — the tool call is still denied.
	maxPromptListEntries = 50
)

// clampDocText bounds text keeping head and tail, leaving a visible marker.
//
// Duplicated from internal/prompt rather than imported: internal/prompt
// already imports this package, so the dependency cannot run the other way.
func clampDocText(text string, maxChars int, label string) string {
	if maxChars <= 0 {
		return ""
	}
	if len(text) <= maxChars {
		return text
	}
	tail := maxChars / 3
	head := maxChars - tail
	marker := fmt.Sprintf("\n\n[codenerd: truncated %d of %d chars from %s] …\n\n",
		len(text)-maxChars, len(text), label)
	return trimPartialRuneSuffix(text[:head]) + marker + trimPartialRunePrefix(text[len(text)-tail:])
}

// listTruncationNotice renders the marker for a capped frontmatter list.
func listTruncationNotice(shown, total int, unit string) string {
	if total <= shown {
		return ""
	}
	return fmt.Sprintf("- [codenerd: truncated %d of %d %s] …\n", total-shown, total, unit)
}

func trimPartialRuneSuffix(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError && size <= 1 {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

func trimPartialRunePrefix(s string) string {
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError && size <= 1 {
			s = s[1:]
			continue
		}
		break
	}
	return s
}
