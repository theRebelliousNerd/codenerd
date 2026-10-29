# Project Document Schema (nerd.md)
#
# These predicates carry the machine-readable half of nerd.md into the kernel.
# The Go loader is internal/projectdoc; policy over them is
# internal/core/defaults/policy/projectdoc.mg.
#
# The distinction that matters: CLAUDE.md and AGENTS.md are prose the model may
# reinterpret. Everything declared here is a fact the executive reasons over, so
# a forbidden path is a denied tool call rather than a strongly-worded request.
#
# Every numeric slot in this corpus is /number and every tag is /name; there are
# no /float64 bounds anywhere (see internal/types/mangle_scale.go for why a
# float here would abort the whole fixpoint). Nothing below carries a number,
# so that hazard does not arise, but new fields must respect it.

# project_doc(Path, Schema)
# One fact per loaded nerd.md, recording where it came from and which schema
# version it declared. Absence means no nerd.md; nerd.md is optional.
Decl project_doc(Path, Schema) bound [/string, /string].

# project_name(Name)
Decl project_name(Name) bound [/string].

# project_language is NOT declared here. It already exists in schemas_project.mg
# with the same /name bound, populated by `nerd init` from the codebase scan.
# nerd.md writes to that same predicate on purpose: a rule that cares about the
# project's language should not have to know whether the answer came from a scan
# or from the user stating it. Declaring it twice is a hard analysis error
# ("declared more than once"), which takes the whole kernel down at boot.

# project_command(Kind, Command)
# Kind is /build, /test, /lint or /run. The canonical invocation for this
# project, so the agent runs the real command instead of inferring one that
# happens to work on a different machine.
Decl project_command(Kind, Command) bound [/name, /string].

# project_command_env(Name, Value)
# Environment a project_command requires. CGO_CFLAGS-style prerequisites are
# invisible in the command string and their absence fails far from the cause.
Decl project_command_env(Name, Value) bound [/string, /string].

# project_forbidden_path(Match, Reason)
# The enforced one. Match is a substring tested against the slash-normalized,
# lowercased target path of a write-mutation tool call. Substring rather than
# glob so the Go gate and this corpus cannot disagree about what a pattern
# means; a matcher that disagrees with itself across layers is a gate that
# sometimes opens.
Decl project_forbidden_path(Match, Reason) bound [/string, /string].

# project_requirement(Text)
# A non-negotiable step ("run go test ./... before handoff").
Decl project_requirement(Text) bound [/string].

# project_convention(ID, Rule)
Decl project_convention(ID, Rule) bound [/string, /string].

# --- Derived ---------------------------------------------------------------

# has_project_doc()
# Zero-arity marker so rules can branch on "this project declared instructions"
# without binding Path.
Decl has_project_doc() bound [].

# project_write_protected()
# True when at least one path is write-protected. Lets a prompt or policy ask
# the cheap question before enumerating rules.
Decl project_write_protected() bound [].

# project_has_command(Kind)
Decl project_has_command(Kind) bound [/name].
