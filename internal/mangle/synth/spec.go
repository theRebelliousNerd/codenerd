package synth

import "encoding/json"

const FormatV1 = "mangle_synth_v1"

type Spec struct {
	Format  string      `json:"format"`
	Program ProgramSpec `json:"program"`
}

type ProgramSpec struct {
	Package *PackageSpec `json:"package,omitempty"`
	Use     []UseSpec    `json:"use,omitempty"`
	Decls   []DeclSpec   `json:"decls,omitempty"`
	Clauses []ClauseSpec `json:"clauses,omitempty"`
}

type PackageSpec struct {
	Name  string     `json:"name"`
	Atoms []AtomSpec `json:"atoms,omitempty"`
}

type UseSpec struct {
	Name  string     `json:"name"`
	Atoms []AtomSpec `json:"atoms,omitempty"`
}

type DeclSpec struct {
	Atom      AtomSpec    `json:"atom"`
	Descr     []AtomSpec  `json:"descr,omitempty"`
	Bounds    []BoundSpec `json:"bounds,omitempty"`
	Inclusion []AtomSpec  `json:"inclusion,omitempty"`
}

type BoundSpec struct {
	Terms []ExprSpec `json:"terms"`
}

type ClauseSpec struct {
	Head      AtomSpec       `json:"head"`
	Body      []TermSpec     `json:"body,omitempty"`
	Transform *TransformSpec `json:"transform,omitempty"`
}

type TransformSpec struct {
	Statements []TransformStmtSpec `json:"statements"`
}

type TransformStmtSpec struct {
	Kind string   `json:"kind"`
	Var  string   `json:"var,omitempty"`
	Fn   ExprSpec `json:"fn"`
}

type TermSpec struct {
	Kind  string    `json:"kind"`
	Atom  *AtomSpec `json:"atom,omitempty"`
	Left  *ExprSpec `json:"left,omitempty"`
	Right *ExprSpec `json:"right,omitempty"`
	Op    string    `json:"op,omitempty"`
}

type AtomSpec struct {
	Pred string     `json:"pred"`
	Args []ExprSpec `json:"args,omitempty"`
}

type ExprSpec struct {
	Kind     string      `json:"kind"`
	Value    string      `json:"value,omitempty"`
	Number   json.Number `json:"number,omitempty"`
	Float    *float64    `json:"float,omitempty"`
	Function string      `json:"function,omitempty"`
	Args     []ExprSpec  `json:"args,omitempty"`
	Arity    *int        `json:"arity,omitempty"`
}

type Options struct {
	RequireSingleClause bool
	AllowDecls          bool
	AllowPackage        bool
	AllowUse            bool
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
