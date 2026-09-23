package campaign

import (
	"context"
	"errors"

	"codenerd/internal/broker"
	"codenerd/internal/observation"
	"codenerd/internal/session"
)

// A failed attempt is recorded with typed signals -- closed-vocabulary names
// read off the error's type and the turn's kernel verdict, never off its
// wording -- and the kernel derives what the task does next from them
// (task_next_move, policy/campaign_decisions.mg). What decided this before was
// a substring classifier over the error text ("connection", "eof" and "i/o"
// meant transient), which called a compile error mentioning "network" a
// network fault.
//
// The vocabulary, in the order failureSignals reports it:
//
//	/refused              the inference broker declined the request
//	/deadline, /canceled  the attempt's context ended
//	/verification_failed  post-edit verification failed (session)
//	/turn_not_done        the turn ran but its verdict was not /done; the
//	                      verdict (/hollow, /failed, /unverified ...) and each
//	                      unmet evidence atom (/tests_not_green ...) follow
//	/tests_red            a campaign test run found the suite red
//	/build_failed         a campaign verify build exited non-zero
//	/unclassified         none of the above: the error carried no type
const signalUnclassified = "/unclassified"

// signalError attaches typed signals to an error without changing its message
// or what it wraps.
type signalError struct {
	err     error
	signals []string
}

func (e *signalError) Error() string { return e.err.Error() }
func (e *signalError) Unwrap() error { return e.err }

// withSignals returns err carrying signals; nil stays nil.
func withSignals(err error, signals ...string) error {
	if err == nil || len(signals) == 0 {
		return err
	}
	return &signalError{err: err, signals: signals}
}

// returnSignals are the signals a turn's own verdict carries: its outcome and
// every evidence atom the kernel found missing.
func returnSignals(ret observation.Return) []string {
	var out []string
	if ret.Outcome != "" && !ret.Done() {
		out = append(out, ret.Outcome)
	}
	return append(out, ret.Missing...)
}

// failureSignals reads the typed signals off err, de-duplicated, in the order
// the vocabulary above lists them. An error that carries none is
// /unclassified: an attempt always has at least one signal.
func failureSignals(err error) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	if err == nil {
		return []string{signalUnclassified}
	}
	// IsAdmissionError reaches through wrapping: a refusal raised inside
	// perception arrives as "observation failed: %w".
	if _, refused := broker.IsAdmissionError(err); refused {
		add("/refused")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		add("/deadline")
	}
	if errors.Is(err, context.Canceled) {
		add("/canceled")
	}
	if errors.Is(err, session.ErrVerificationFailed) {
		add("/verification_failed")
	}
	if errors.Is(err, ErrTaskNotDone) {
		add("/turn_not_done")
	}
	walkSignalErrors(err, func(e *signalError) {
		for _, s := range e.signals {
			add(s)
		}
	})
	if len(out) == 0 {
		add(signalUnclassified)
	}
	return out
}

// walkSignalErrors visits every signalError in err's tree, through both
// single and joined wrapping.
func walkSignalErrors(err error, visit func(*signalError)) {
	if err == nil {
		return
	}
	if se, ok := err.(*signalError); ok {
		visit(se)
	}
	switch u := err.(type) {
	case interface{ Unwrap() error }:
		walkSignalErrors(u.Unwrap(), visit)
	case interface{ Unwrap() []error }:
		for _, e := range u.Unwrap() {
			walkSignalErrors(e, visit)
		}
	}
}
