package prompt

import (
	"fmt"
	"sort"
	"strings"

	userconfig "codenerd/internal/config"
	"codenerd/internal/observation"
)

// retrievedKnowledgeShare is jit.retrieved_knowledge_percent and
// jit.retrieved_knowledge_max_tokens as one compile reads them.
type retrievedKnowledgeShare struct {
	percent   int
	maxTokens int
	// budget is the compile's token budget, the one fitting uses: the
	// context's available tokens, or the compiler's default without them.
	budget int
}

// resolved fills unset (non-positive) fields from the JIT defaults.
func (s retrievedKnowledgeShare) resolved() retrievedKnowledgeShare {
	d := userconfig.DefaultJITConfig()
	if s.percent <= 0 {
		s.percent = d.RetrievedKnowledgePercent
	}
	if s.maxTokens <= 0 {
		s.maxTokens = d.RetrievedKnowledgeMaxTokens
	}
	return s
}

func (c CompilerConfig) retrievedKnowledgeShare() retrievedKnowledgeShare {
	return retrievedKnowledgeShare{percent: c.RetrievedKnowledgePercent, maxTokens: c.RetrievedKnowledgeMaxTokens}.resolved()
}

// knowledgeWithheldAtomID names the pointer atom.
const knowledgeWithheldAtomID = "knowledge-withheld/pointer"

// withheldKnowledgePointer reads the retrieved rows the kernel withheld
// (retrieved_withheld) and returns one mandatory atom naming each with a
// recall handle, or nil when none was withheld. A withheld row's full text is
// retained (observation.SharedKnowledge) so recall_context returns it whole.
func withheldKnowledgePointer(kernel KernelQuerier, candidates []*PromptAtom, share retrievedKnowledgeShare) (*ScoredAtom, error) {
	if kernel == nil {
		return nil, nil
	}
	rows, err := kernel.Query("retrieved_withheld(Atom, Reason)")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	byID := make(map[string]*PromptAtom, len(candidates))
	for _, a := range candidates {
		if a != nil && a.RetrievedContext {
			byID[a.ID] = a
		}
	}
	var withheld []*PromptAtom
	for _, row := range rows {
		if len(row.Args) == 0 {
			continue
		}
		id, err := extractStringArg(row.Args[0])
		if err != nil {
			continue
		}
		if a := byID[id]; a != nil {
			withheld = append(withheld, a)
		}
	}
	if len(withheld) == 0 {
		return nil, nil
	}
	sort.Slice(withheld, func(i, j int) bool { return withheld[i].ID < withheld[j].ID })

	share = share.resolved()
	var b strings.Builder
	fmt.Fprintf(&b, "[knowledge withheld] %d retrieved knowledge row(s) did not fit this prompt's retrieved-knowledge share (%d%% of the budget, at most %d tokens) and are left out whole. Each is kept verbatim: read one with recall_context id=<handle> when the task needs it.\n",
		len(withheld), share.percent, share.maxTokens)
	for _, a := range withheld {
		header, _, _ := strings.Cut(a.Content, "\n")
		handle := observation.SharedKnowledge().Retain(a.Content)
		if handle == "" {
			handle = "(not retained: too large for the recall store)"
		}
		fmt.Fprintf(&b, "- %s %d chars: %s\n", header, len(a.Content), handle)
	}
	pointer := NewPromptAtom(knowledgeWithheldAtomID, CategoryKnowledge, strings.TrimRight(b.String(), "\n"))
	pointer.IsMandatory = true
	return &ScoredAtom{Atom: pointer, LogicScore: 1, Combined: 1, SelectionReason: "retrieved_withheld", Source: "kernel"}, nil
}
