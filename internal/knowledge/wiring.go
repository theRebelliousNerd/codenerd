package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/types"
)

// NamedSearcher is a configured client that may offer grounded web search,
// named by its provider.
type NamedSearcher struct {
	Provider string
	Searcher types.GroundedWebSearcher
}

// WebChain is grounded web search across every configured provider that
// offers it: the first provider whose search answers with sources supplies
// the subject's atoms, so one subject costs one successful search; a
// provider that failed first is named in the outcome's detail.
type WebChain struct {
	Providers []WebSource
}

func (WebChain) Name() string { return "web" }

// Available reports whether any configured provider offers grounded search.
func (w WebChain) Available() bool {
	for _, p := range w.Providers {
		if p.Searcher != nil && p.Searcher.SupportsGroundedWebSearch() {
			return true
		}
	}
	return false
}

func (w WebChain) Acquire(ctx context.Context, s Subject) ([]Atom, error) {
	var notes []string
	unavailable := 0
	for _, p := range w.Providers {
		atoms, err := p.Acquire(ctx, s)
		switch {
		case errors.Is(err, ErrUnavailable):
			unavailable++
			notes = append(notes, err.Error())
		case err != nil:
			notes = append(notes, fmt.Sprintf("%s failed: %v", p.Name(), err))
		case len(atoms) == 0:
			notes = append(notes, p.Name()+" answered nothing")
		default:
			return atoms, nil
		}
	}
	if len(w.Providers) == 0 || unavailable == len(w.Providers) {
		return nil, fmt.Errorf("%w: no configured provider offers grounded web search", ErrUnavailable)
	}
	return nil, errors.New(strings.Join(notes, "; "))
}

// Config wires the sources an acquirer runs.
type Config struct {
	// Searchers are the configured clients, deduplicated by provider.
	Searchers []NamedSearcher
	// Context7 fetches Context7 documentation for a topic; nil when not
	// wired.
	Context7 func(ctx context.Context, topic string) (string, error)
	// Workspace is the project root go doc runs in.
	Workspace string
	// Kernel holds the world model's dependency_link facts, which the
	// repository and pinned-documentation sources read.
	Kernel types.Kernel
	// Run runs go; ExecRun when nil.
	Run RunFunc
}

// NewAcquirer builds the acquirer every knowledge build uses: grounded web
// search across the configured providers, Context7, the pinned Go module
// documentation, and the repository's own use.
func NewAcquirer(cfg Config) Acquirer {
	var web WebChain
	seen := map[string]bool{}
	for _, s := range cfg.Searchers {
		if s.Searcher == nil || seen[s.Provider] {
			continue
		}
		seen[s.Provider] = true
		web.Providers = append(web.Providers, WebSource{Provider: s.Provider, Searcher: s.Searcher})
	}
	imports := KernelImports(cfg.Kernel)
	repo := RepoSource{}
	if cfg.Kernel != nil {
		repo.Importers = imports.Importers
	}
	return Acquirer{Sources: []Source{
		web,
		Context7Source{Fetch: cfg.Context7},
		GoDocSource{Workspace: cfg.Workspace, Packages: imports.Packages, Run: cfg.Run},
		repo,
	}}
}

// Imports reads the world model's import edges.
type Imports struct {
	kernel types.Kernel
}

// KernelImports reads dependency_link(File, _, Import) from kernel; a nil
// kernel answers nothing.
func KernelImports(kernel types.Kernel) Imports { return Imports{kernel: kernel} }

func under(imp, module string) bool {
	return imp == module || strings.HasPrefix(imp, module+"/")
}

func (i Imports) rows() ([]types.Fact, error) {
	if i.kernel == nil {
		return nil, nil
	}
	return i.kernel.Query("dependency_link")
}

// Importers lists the files that import a package under module.
func (i Imports) Importers(_ context.Context, module string) ([]string, error) {
	if i.kernel == nil {
		return nil, fmt.Errorf("%w: no world model is wired", ErrUnavailable)
	}
	rows, err := i.rows()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var files []string
	for _, f := range rows {
		if len(f.Args) < 3 {
			continue
		}
		if under(types.ExtractString(f.Args[2]), module) {
			if file := types.ExtractString(f.Args[0]); file != "" && !seen[file] {
				seen[file] = true
				files = append(files, file)
			}
		}
	}
	sort.Strings(files)
	return files, nil
}

// Packages lists the distinct import paths under module the repository uses.
func (i Imports) Packages(_ context.Context, module string) ([]string, error) {
	rows, err := i.rows()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var pkgs []string
	for _, f := range rows {
		if len(f.Args) < 3 {
			continue
		}
		if imp := types.ExtractString(f.Args[2]); under(imp, module) && !seen[imp] {
			seen[imp] = true
			pkgs = append(pkgs, imp)
		}
	}
	sort.Strings(pkgs)
	return pkgs, nil
}
