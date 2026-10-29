// Package catalog is the one list of tool names production can register.
//
// Package tools cannot import the family packages (they import tools), so
// the list lives here, beside them. VirtualStore.HydrateModularTools walks
// Families for both registries, then calls RegisterGroundedWebSearch.
// Names is that same sequence with the conditional tool included, so the
// atom validator and the action linter do not keep their own copies.
package catalog

import (
	"context"
	"fmt"

	"codenerd/internal/tools"
	"codenerd/internal/tools/codedom"
	"codenerd/internal/tools/core"
	"codenerd/internal/tools/mcpctl"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"codenerd/internal/types"
)

// Family is one production RegisterAll.
//
// Name is the qualifier HydrateModularTools puts in its error and log
// ("core", "mcpctl"). Register installs that family's tools.
type Family struct {
	Name     string
	Register func(*tools.Registry) error
}

// families is the RegisterAll sequence production installs, in order.
// Families returns a copy; Names and HydrateModularTools both walk that
// copy, so a caller cannot append a family onto the sequence the process
// will install next.
var families = []Family{
	{Name: "core", Register: core.RegisterAll},
	{Name: "shell", Register: shell.RegisterAll},
	{Name: "codedom", Register: codedom.RegisterAll},
	// Unconditional, like the browser tools: the verbs are the stable
	// surface. Whether any MCP server is connected is a runtime fact the
	// atlas reports, not a reason to hide the tools.
	{Name: "mcpctl", Register: mcpctl.RegisterAll},
	{Name: "research", Register: research.RegisterAll},
}

// Families returns the production RegisterAll sequence.
//
// The slice is a copy of the canonical list. The function values are the
// family RegisterAll functions themselves.
func Families() []Family {
	out := make([]Family, len(families))
	copy(out, families)
	return out
}

// RegisterGroundedWebSearch is the conditional registrar production runs
// after Families.
//
// HydrateModularTools and Names both call this function, so they cannot
// name different registrars. The implementation stays
// research.RegisterGroundedWebSearchIfSupported, which owns the tool and
// its unexported name: a nil searcher, or one that does not support
// grounded search, skips registration and returns false, nil.
func RegisterGroundedWebSearch(registry *tools.Registry, searcher types.GroundedWebSearcher) (bool, error) {
	return research.RegisterGroundedWebSearchIfSupported(registry, searcher)
}

// Names returns every tool name production can register, sorted and unique.
//
// The conditional name is whatever RegisterGroundedWebSearch installs for
// a supporting searcher, not a second string list.
func Names() ([]string, error) {
	reg := tools.NewRegistry()
	for _, family := range Families() {
		if err := registerFamily(reg, family); err != nil {
			return nil, err
		}
	}
	if err := includeConditional(reg); err != nil {
		return nil, err
	}
	names := reg.Names()
	if len(names) == 0 {
		return nil, fmt.Errorf("production tool catalog is empty")
	}
	return names, nil
}

func registerFamily(reg *tools.Registry, family Family) error {
	if family.Name == "" {
		return fmt.Errorf("production family has an empty name")
	}
	if family.Register == nil {
		return fmt.Errorf("%s: nil registrar", family.Name)
	}
	if err := family.Register(reg); err != nil {
		return fmt.Errorf("%s.RegisterAll: %w", family.Name, err)
	}
	return nil
}

// includeConditional installs the tool RegisterGroundedWebSearch registers
// for a supporting searcher. A family that already registered that name is
// fine. A registrar that skips the supporting searcher is not: Names would
// then omit a tool HydrateModularTools can install.
func includeConditional(reg *tools.Registry) error {
	before := reg.Count()
	ok, err := RegisterGroundedWebSearch(reg, namesSearcher{})
	if err != nil {
		return fmt.Errorf("register grounded web search: %w", err)
	}
	if ok || reg.Count() > before {
		return nil
	}
	probe := tools.NewRegistry()
	pok, perr := RegisterGroundedWebSearch(probe, namesSearcher{})
	if perr != nil {
		return fmt.Errorf("register grounded web search: %w", perr)
	}
	installed := probe.Names()
	if !pok || len(installed) == 0 {
		return fmt.Errorf("register grounded web search skipped a supporting searcher")
	}
	for _, name := range installed {
		if !reg.Has(name) {
			return fmt.Errorf("conditional tool %q missing from production catalog", name)
		}
	}
	return nil
}

// namesSearcher exists so Names can ask RegisterGroundedWebSearch to
// install the conditional tool and read the name it actually registered.
// GroundedWebSearch is never called: Names discards the registry.
type namesSearcher struct{}

func (namesSearcher) SupportsGroundedWebSearch() bool { return true }

func (namesSearcher) GroundedWebSearch(context.Context, string) (*types.GroundedWebSearchResult, error) {
	return nil, fmt.Errorf("catalog.Names does not execute grounded web search")
}
