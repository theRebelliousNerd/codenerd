package synth

import "encoding/json"

const FormatV1 = "mangle_synth_v1"

type Spec struct {
	Format  string      `json:"format"`
	Program ProgramSpec `json:"program"`
}

type ProgramSpec struct {
	Package *PackageSpec `json:"package,omitzero"`
	Use     []UseSpec    `json:"use,omitzero"`
	Decls   []DeclSpec   `json:"decls,omitzero"`
	Clauses []ClauseSpec `json:"clauses,omitzero"`
}

type PackageSpec struct {
	Name  string     `json:"name"`
	Atoms []AtomSpec `json:"atoms,omitzero"`
}

type UseSpec struct {
	Name  string     `json:"name"`
	Atoms []AtomSpec `json:"atoms,omitzero"`
}

type DeclSpec struct {
	Atom      AtomSpec    `json:"atom"`
	Descr     []AtomSpec  `json:"descr,omitzero"`
	Bounds    []BoundSpec `json:"bounds,omitzero"`
	Inclusion []AtomSpec  `json:"inclusion,omitzero"`
}

type BoundSpec struct {
	Terms []ExprSpec `json:"terms"`
}

type ClauseSpec struct {
	Head      AtomSpec       `json:"head"`
	Body      []TermSpec     `json:"body,omitzero"`
	Transform *TransformSpec `json:"transform,omitzero"`
}

type TransformSpec struct {
	Statements []TransformStmtSpec `json:"statements"`
}

type TransformStmtSpec struct {
	Kind string   `json:"kind"`
	Var  string   `json:"var,omitzero"`
	Fn   ExprSpec `json:"fn"`
}

type TermSpec struct {
	Kind  string    `json:"kind"`
	Atom  *AtomSpec `json:"atom,omitzero"`
	Left  *ExprSpec `json:"left,omitzero"`
	Right *ExprSpec `json:"right,omitzero"`
	Op    string    `json:"op,omitzero"`
}

type AtomSpec struct {
	Pred string     `json:"pred"`
	Args []ExprSpec `json:"args,omitzero"`
}

type ExprSpec struct {
	Kind     string      `json:"kind"`
	Value    string      `json:"value,omitzero"`
	Number   json.Number `json:"number,omitzero"`
	Float    *float64    `json:"float,omitzero"`
	Function string      `json:"function,omitzero"`
	Args     []ExprSpec  `json:"args,omitzero"`
	Arity    *int        `json:"arity,omitzero"`
}

type Options struct {
	RequireSingleClause bool
	AllowDecls          bool
	AllowPackage        bool
	AllowUse            bool
	SkipAnalysis        bool
}

func DefaultOptions() Options {
	return Options{
		AllowDecls:   true,
		AllowPackage: true,
		AllowUse:     true,
	}
}

type Result struct {
	Source  string
	Clauses []string
	Decls   []string
}

func (r Result) SingleClause() (string, error) {
	if len(r.Clauses) != 1 {
		return "", NewSpecError("program.clauses", "expected exactly one clause")
	}
	return r.Clauses[0], nil
}

type SpecError struct {
	Path    string
	Message string
}

func (e SpecError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

func NewSpecError(path, message string) SpecError {
	return SpecError{Path: path, Message: message}
}
