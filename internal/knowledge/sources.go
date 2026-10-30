package knowledge

import (
	"context"
	"fmt"
	"go/build"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"codenerd/internal/processutil"
	"codenerd/internal/types"
)

// The sources. Each produces atoms with its own provenance and reports
// unavailability (ErrUnavailable) apart from failure.

var slugRun = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(slugRun.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// sections splits prose into its paragraphs of at least min characters.
func sections(text string, min int) []string {
	var out []string
	for _, part := range strings.Split(text, "\n\n") {
		if part = strings.TrimSpace(part); len(part) >= min {
			out = append(out, part)
		}
	}
	return out
}

// WebSource is grounded web search on one configured provider's client: an
// answer is kept only with the sources it cites, each section stamped with
// its question, sources and retrieval date (so age can be judged).
type WebSource struct {
	// Provider names the client's provider ("openai", "meta"): the source
	// is "web/<provider>".
	Provider string
	Searcher types.GroundedWebSearcher
	Now      func() time.Time
}

func (w WebSource) Name() string { return "web/" + w.Provider }

func (w WebSource) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	if w.Searcher == nil || !w.Searcher.SupportsGroundedWebSearch() {
		return nil, fmt.Errorf("%w: provider %s offers no grounded web search", ErrUnavailable, w.Provider)
	}
	question := webQuestion(s)
	result, err := w.Searcher.GroundedWebSearch(ctx, question)
	if err != nil {
		return nil, err
	}
	sources := citedSources(result)
	if len(sources) == 0 {
		return nil, fmt.Errorf("the answer cited no source; nothing ungrounded is stored")
	}
	now := time.Now
	if w.Now != nil {
		now = w.Now
	}
	footer := fmt.Sprintf("\n\nQuestion: %s\nSources: %s\nRetrieved: %s", question, strings.Join(sources, " ; "), now().UTC().Format("2006-01-02"))
	var atoms []Atom
	for i, sec := range sections(result.Text, 50) {
		atoms = append(atoms, Atom{
			Concept:    fmt.Sprintf("web/%s/section_%d", slug(s.Key()), i),
			Content:    sec + footer,
			Confidence: 0.8,
			Source:     w.Name(),
		})
	}
	return atoms, nil
}

func webQuestion(s Subject) string {
	if s.Gap {
		return fmt.Sprintf("An engineer asked: %q. Answer from current sources: the correct approach, the API usage it needs, and the common mistakes.", s.Topic)
	}
	q := fmt.Sprintf("What should an engineer working with %s know: current idioms, correct API usage, common mistakes and their fixes", s.Topic)
	if s.Module != "" {
		q += fmt.Sprintf(" (the Go module %s)", s.Module)
	}
	return q + "?"
}

func citedSources(result *types.GroundedWebSearchResult) []string {
	if result == nil {
		return nil
	}
	seen := map[string]bool{}
	var urls []string
	for _, c := range result.Citations {
		if u := strings.TrimSpace(c.URL); u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	for _, r := range result.Results {
		if u := strings.TrimSpace(r.URL); u != "" && !seen[u] {
			seen[u] = true
			urls = append(urls, u)
		}
	}
	sort.Strings(urls)
	return urls
}

// Context7Source is Context7 library documentation (keyed or public), through
// the research tool's fetch.
type Context7Source struct {
	Fetch func(ctx context.Context, topic string) (string, error)
}

func (Context7Source) Name() string { return "context7" }

// Available reports whether a fetch is wired.
func (c Context7Source) Available() bool { return c.Fetch != nil }

func (c Context7Source) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	if c.Fetch == nil {
		return nil, fmt.Errorf("%w: no Context7 fetch is wired", ErrUnavailable)
	}
	topic := s.Topic
	if s.Module != "" {
		topic = s.Module
	}
	body, err := c.Fetch(ctx, topic)
	if err != nil {
		return nil, err
	}
	var atoms []Atom
	for i, sec := range sections(body, 50) {
		atoms = append(atoms, Atom{
			Concept:    fmt.Sprintf("context7/%s/section_%d", slug(topic), i),
			Content:    sec,
			Confidence: 0.8,
			Source:     "context7",
		})
	}
	return atoms, nil
}

// RunFunc runs a command in dir and returns its combined output.
type RunFunc func(ctx context.Context, dir, name string, args ...string) ([]byte, error)

// ExecRun is RunFunc on the host, with whole-tree cancellation.
func ExecRun(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return processutil.CombinedOutput(cmd)
}

// GoDocSource is the documentation of the exact version the workspace pins:
// `go doc` of each package the repository imports from the module, run in
// the workspace so the module cache resolves the go.mod version. It is the
// version-accurate source a dependency upgrade re-reads.
type GoDocSource struct {
	Workspace string
	// Packages lists the import paths under a module the repository uses;
	// the module itself when nil or empty.
	Packages func(ctx context.Context, module string) ([]string, error)
	Run      RunFunc
}

func (GoDocSource) Name() string { return "godoc" }

// Available reports whether go doc can run: a workspace and a go toolchain.
func (g GoDocSource) Available() bool {
	if g.Workspace == "" {
		return false
	}
	if g.Run != nil {
		return true
	}
	_, err := exec.LookPath("go")
	return err == nil
}

func (g GoDocSource) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	if g.Workspace == "" {
		return nil, fmt.Errorf("%w: no workspace to run go doc in", ErrUnavailable)
	}
	if s.Module == "" || s.Ecosystem != "go" {
		return nil, fmt.Errorf("%w: no Go module pinned for %s", ErrInapplicable, s.Topic)
	}
	run := g.Run
	if run == nil {
		run = ExecRun
	}
	version := ModuleVersion(ctx, run, g.Workspace, s.Module)
	pkgs := []string{s.Module}
	if g.Packages != nil {
		if used, err := g.Packages(ctx, s.Module); err == nil && len(used) > 0 {
			pkgs = used
		}
	}
	source := "godoc/" + s.Module + "@" + version
	var atoms []Atom
	var failures []string
	for _, pkg := range pkgs {
		out, err := run(ctx, g.Workspace, "go", "doc", pkg)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", pkg, firstLine(string(out), err)))
			continue
		}
		doc := strings.TrimSpace(string(out))
		if doc == "" {
			continue
		}
		atoms = append(atoms, Atom{
			Concept:    fmt.Sprintf("godoc/%s@%s", pkg, version),
			Content:    fmt.Sprintf("%s at %s, as pinned in go.mod (go doc):\n\n%s", pkg, version, doc),
			Confidence: 0.95,
			Source:     source,
		})
	}
	if len(atoms) == 0 && len(failures) > 0 {
		return nil, fmt.Errorf("go doc failed: %s", strings.Join(failures, "; "))
	}
	return atoms, nil
}

// ModuleVersion is the version go.mod pins for module ("std" for the standard
// library, "unknown" when go list cannot say).
func ModuleVersion(ctx context.Context, run RunFunc, workspace, module string) string {
	if IsStdPackage(module) {
		return "std"
	}
	out, err := run(ctx, workspace, "go", "list", "-m", "-f", "{{.Version}}", module)
	if v := strings.TrimSpace(string(out)); err == nil && v != "" {
		return v
	}
	return "unknown"
}

// IsStdPackage reports whether path is a standard-library package of the
// local Go toolchain (its GOROOT holds it). A path whose first element has no
// dot is not enough: a vocabulary key may be spelled that way.
func IsStdPackage(path string) bool {
	if path == "" || strings.Contains(strings.SplitN(path, "/", 2)[0], ".") {
		return false
	}
	pkg, err := build.Default.Import(path, "", build.FindOnly)
	return err == nil && pkg.Goroot
}

func firstLine(out string, err error) string {
	if line, _, _ := strings.Cut(strings.TrimSpace(out), "\n"); line != "" {
		return line
	}
	return err.Error()
}

// RepoSource is the repository's own use of the technology, from the world
// model: the files that import it.
type RepoSource struct {
	// Importers lists the workspace files importing under the module.
	Importers func(ctx context.Context, module string) ([]string, error)
}

func (RepoSource) Name() string { return "repo/world" }

// Available reports whether a world model is wired.
func (r RepoSource) Available() bool { return r.Importers != nil }

func (r RepoSource) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	if s.Module == "" {
		return nil, fmt.Errorf("%w: no module to look up for %s", ErrInapplicable, s.Topic)
	}
	if r.Importers == nil {
		return nil, fmt.Errorf("%w: no world model is wired", ErrUnavailable)
	}
	files, err := r.Importers(ctx, s.Module)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}
	sort.Strings(files)
	return []Atom{{
		Concept: "repo/" + s.Module + "/importers",
		Content: fmt.Sprintf("In this repository %s is imported by %d file(s); read them for the project's own conventions with it:\n- %s",
			s.Module, len(files), strings.Join(files, "\n- ")),
		Confidence: 0.9,
		Source:     "repo/world",
	}}, nil
}
