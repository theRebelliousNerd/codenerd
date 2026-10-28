package sessionlog

import (
	"context"
	"path/filepath"
	"time"
)

// CallMeta describes a model call at its start.
type CallMeta struct {
	Provider string
	Model    string
	Method   string
	Purpose  string // names the file: NNNN-<purpose>.json
	Phase    string
	Session  string // "" means the store's session
	// Folder forces the folder, whatever agent scope ctx carries (Jev calls).
	Folder string
	// Fallback is the folder when ctx carries no agent scope.
	Fallback   string
	Credential string
}

// Result describes a model call at its end.
type Result struct {
	Response  *Response
	Usage     *Usage
	Admission any
	Err       error
	Refused   bool
	// Logical is the request in the broker's terms; it is written only when
	// the call produced no wire capture.
	Logical *LogicalRequest
}

// Call is one in-flight exchange. A nil *Call (logging inactive) accepts End.
type Call struct {
	store   *Store
	meta    CallMeta
	scope   *Scope
	session string
	folder  string
	dir     string
	number  int
	purpose string
	capture *Capture
	started time.Time
	turn    int
	task    string
}

// Begin opens an exchange when exchange logging is active and returns a
// context whose transports record into it. When logging is off it returns ctx
// unchanged and a nil Call, so the disabled path allocates nothing.
//
// The exchange number is taken here, so files sort in the order calls
// started, whatever order they finish in.
func Begin(ctx context.Context, meta CallMeta) (context.Context, *Call) {
	s := Current()
	if s == nil || !s.enabled() {
		return ctx, nil
	}
	scope := ScopeFrom(ctx)
	folder := meta.Fallback
	if scope != nil {
		folder = scope.Folder
	}
	if meta.Folder != "" {
		folder = meta.Folder
		scope = nil
	}
	if folder == "" {
		folder = UnattributedFolder
	}
	session := s.resolve(meta.Session)
	dir, err := s.folderDir(session, folder)
	if err != nil {
		s.fail(err)
		return ctx, nil
	}
	n, err := s.next(dir)
	if err != nil {
		s.fail(err)
		return ctx, nil
	}
	purpose := meta.Purpose
	if purpose == "" {
		purpose = "call"
	}
	if meta.Phase != "" {
		purpose += "-" + meta.Phase
	}
	c := &Call{
		store: s, meta: meta, scope: scope, session: session, folder: folder, dir: dir,
		number: n, purpose: purpose, capture: &Capture{}, started: time.Now(),
		turn: TurnFrom(ctx), task: TaskFrom(ctx),
	}
	return WithCapture(ctx, c.capture), c
}

// File is the exchange's file name.
func (c *Call) File() string {
	if c == nil {
		return ""
	}
	return exchangeFileName(c.number, c.purpose)
}

// End writes the exchange file. It never returns an error and never panics
// into the caller: a failed write is counted on the store.
func (c *Call) End(r Result) {
	if c == nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			c.store.fail(panicError{p})
		}
	}()
	ex := Exchange{
		Schema:       ExchangeSchema,
		TextEncoding: TextEncoding,
		Number:       c.number,
		Purpose:      c.purpose,
		Correlation: Correlation{
			Session: c.session, Turn: c.turn, Task: c.task, Agent: c.folder,
			Phase: c.meta.Phase, Credential: c.meta.Credential,
		},
		Provider:  c.meta.Provider,
		Model:     c.meta.Model,
		Method:    c.meta.Method,
		Started:   c.started,
		LatencyMS: time.Since(c.started).Milliseconds(),
		Outcome:   OutcomeOK,
		Wire:      c.capture.Wires(),
		Response:  r.Response,
		Usage:     r.Usage,
		Admission: r.Admission,
	}
	if c.scope != nil {
		ex.Correlation.Role = c.scope.Role
		ex.Correlation.AgentID = c.scope.ID
		ex.Decisions = takeDecisions(c.scope, c.File())
	}
	switch {
	case r.Refused:
		ex.Outcome = OutcomeRefused
	case r.Err != nil:
		ex.Outcome = OutcomeError
	}
	if r.Err != nil {
		ex.Error = r.Err.Error()
	}
	if len(ex.Wire) == 0 {
		ex.LogicalRequest = r.Logical
	}
	data, err := MarshalFile(ex)
	if err != nil {
		c.store.fail(err)
		return
	}
	c.store.writeFile(filepath.Join(c.dir, c.File()), data)
}

type panicError struct{ v any }

func (p panicError) Error() string { return "panic while writing an exchange: " + sprint(p.v) }

func sprint(v any) string {
	if e, ok := v.(error); ok {
		return e.Error()
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "unknown"
}
