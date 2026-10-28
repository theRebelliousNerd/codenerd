package verification

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"slices"
	"strings"
)

// The judge's reply contract (Docs/audits/verification-evidence-and-judge-
// contracts.md, Gap A). JSON syntax is not a verification contract: Go's
// decoder ignores unknown keys, keeps the last of duplicate keys, and leaves an
// omitted field at its zero value, so "{}" decoded to a judgment -- a failure
// at confidence 0 that withheld under the default threshold and was accepted
// under any other, and an omitted confidence was indistinguishable from a
// genuine 0. Every reply now passes this contract before any judge_verdict
// fact exists, and one that does not is a judge that could not be had: the
// delegation fails closed without rerunning the worker.

// JudgeContract is the reply contract version the judge's atoms teach
// (internal/prompt/atoms/eval/delegation_judge.yaml). A reply naming any other
// is refused rather than read under a guessed interpretation.
const JudgeContract = "judge/v2"

// JudgeStatus is what the judge concluded. Unavailable is not one: it is the
// host's state for a reply that broke the contract, never a model vote.
type JudgeStatus string

const (
	// JudgePass: the attempt did what was asked; nothing to withhold.
	JudgePass JudgeStatus = "pass"
	// JudgeDefect: a defect grounded in packet evidence.
	JudgeDefect JudgeStatus = "defect"
	// JudgeInsufficientEvidence: the packet does not settle the question.
	// Never a worker retry: the worker did nothing the judge could fault.
	JudgeInsufficientEvidence JudgeStatus = "insufficient_evidence"
)

// Atom is the status as judge_verdict carries it (policy/delegation.mg).
func (s JudgeStatus) Atom() string { return "/" + string(s) }

// maxJudgeListItems bounds every list in a reply.
const maxJudgeListItems = 32

// knownViolations is every quality violation the contract admits.
var knownViolations = map[QualityViolation]bool{
	MockCode: true, PlaceholderCode: true, HallucinatedAPI: true, IncompleteImpl: true,
	HardcodedValues: true, EmptyFunction: true, MissingErrors: true, FakeTests: true,
}

var knownCorrective = map[CorrectiveType]bool{
	CorrectiveResearch: true, CorrectiveDocs: true, CorrectiveTool: true, CorrectiveDecompose: true,
}

// Judgment is the judge's reply after the contract and the host's grounding:
// what it claimed, and what of that the packet supports.
type Judgment struct {
	Contract string
	// Claimed is the status the judge replied with. Status is the status
	// the host stands behind after grounding: a defect none of whose
	// citations resolves to usable packet evidence is insufficient evidence,
	// never a confirmed defect.
	Claimed     JudgeStatus
	Status      JudgeStatus
	Confidence  float64
	Explanation string
	Violations  []QualityViolation
	// Citations are the evidence IDs the judge cited; Grounded the ones
	// that resolve to usable packet items, Ungrounded the rest (unknown or
	// stale).
	Citations  []string
	Grounded   []string
	Ungrounded []string
	// UnsupportedSuspicion is a defect the judge could not tie to evidence,
	// labeled as such. It is kept for the record and never becomes a fact.
	UnsupportedSuspicion string
	// RequestedEvidence is what an insufficient-evidence reply asked to
	// see; Expandable the requested items the host can supply, and
	// Unsupplied each other one with why ("E7: every byte of it has been
	// shown") -- a request that cannot progress is said, not repeated.
	RequestedEvidence []string
	Expandable        []string
	Unsupplied        []string
	Suggestions       []string
	CorrectiveAction  *CorrectiveAction
	// Downgraded says why the host's Status differs from Claimed.
	Downgraded string
}

// judgeWire is the reply as decoded: pointer fields so that presence is
// tracked apart from zero values.
type judgeWire struct {
	Contract             *string            `json:"contract"`
	Status               *string            `json:"status"`
	Confidence           *float64           `json:"confidence"`
	Explanation          *string            `json:"explanation"`
	Violations           []QualityViolation `json:"violations"`
	Citations            []string           `json:"citations"`
	UnsupportedSuspicion string             `json:"unsupported_suspicion"`
	RequestedEvidence    []string           `json:"requested_evidence"`
	Suggestions          []string           `json:"suggestions"`
	CorrectiveAction     *CorrectiveAction  `json:"corrective_action"`
}

// parseJudgment holds a reply to the contract. Every error is a reason the
// reply is not a judgment.
func parseJudgment(reply string, maxBytes int) (*Judgment, error) {
	if maxBytes > 0 && len(reply) > maxBytes {
		return nil, fmt.Errorf("the reply is %d bytes, over delegation.judge_reply_bytes (%d)", len(reply), maxBytes)
	}
	body := strings.TrimSpace(reply)
	body = strings.TrimPrefix(body, "```json")
	body = strings.TrimPrefix(body, "```")
	body = strings.TrimSuffix(body, "```")
	body = strings.TrimSpace(body)
	if err := checkMembers([]byte(body)); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	var w *judgeWire
	if err := dec.Decode(&w); err != nil {
		return nil, fmt.Errorf("not a %s object: %w", JudgeContract, err)
	}
	if w == nil {
		return nil, errors.New("the reply is null, not a judgment")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("the reply carries more than one JSON value")
	}

	switch {
	case w.Contract == nil:
		return nil, errors.New(`"contract" is missing`)
	case *w.Contract != JudgeContract:
		return nil, fmt.Errorf(`"contract" is %q, not %q`, *w.Contract, JudgeContract)
	case w.Status == nil:
		return nil, errors.New(`"status" is missing`)
	case w.Confidence == nil:
		return nil, errors.New(`"confidence" is missing; an omitted confidence is not a confidence of 0`)
	case math.IsNaN(*w.Confidence) || *w.Confidence < 0 || *w.Confidence > 1:
		return nil, fmt.Errorf(`"confidence" %v is outside [0,1]; it is refused, not clamped`, *w.Confidence)
	case w.Explanation == nil || strings.TrimSpace(*w.Explanation) == "":
		return nil, errors.New(`"explanation" is missing or blank`)
	}
	for name, n := range map[string]int{"violations": len(w.Violations), "citations": len(w.Citations), "requested_evidence": len(w.RequestedEvidence), "suggestions": len(w.Suggestions)} {
		if n > maxJudgeListItems {
			return nil, fmt.Errorf("%q has %d items, over %d", name, n, maxJudgeListItems)
		}
	}
	for _, v := range w.Violations {
		if !knownViolations[v] {
			return nil, fmt.Errorf("unknown violation %q", v)
		}
	}
	if a := w.CorrectiveAction; a != nil {
		if !knownCorrective[a.Type] || strings.TrimSpace(a.Query) == "" {
			return nil, fmt.Errorf("corrective_action needs a known type and a query (got %q, %q)", a.Type, a.Query)
		}
	}

	j := &Judgment{
		Contract:             *w.Contract,
		Claimed:              JudgeStatus(*w.Status),
		Confidence:           *w.Confidence,
		Explanation:          strings.TrimSpace(*w.Explanation),
		Violations:           w.Violations,
		Citations:            trimIDs(w.Citations),
		UnsupportedSuspicion: strings.TrimSpace(w.UnsupportedSuspicion),
		RequestedEvidence:    trimIDs(w.RequestedEvidence),
		Suggestions:          w.Suggestions,
		CorrectiveAction:     w.CorrectiveAction,
	}
	j.Status = j.Claimed
	// Contradictory combinations are refused: a pass that names a defect,
	// a defect that names none.
	switch j.Claimed {
	case JudgePass:
		if len(j.Violations) > 0 || len(j.Citations) > 0 || j.UnsupportedSuspicion != "" || len(j.RequestedEvidence) > 0 || j.CorrectiveAction != nil {
			return nil, errors.New("a pass carries violations, citations, a suspicion, requested evidence or a corrective action")
		}
	case JudgeDefect:
		if len(j.Citations) == 0 && j.UnsupportedSuspicion == "" {
			return nil, errors.New("a defect cites no evidence and labels no unsupported suspicion")
		}
		if len(j.RequestedEvidence) > 0 {
			return nil, errors.New("a defect requests evidence; a judge that needs more evidence replies insufficient_evidence")
		}
	case JudgeInsufficientEvidence:
		if len(j.Violations) > 0 || j.CorrectiveAction != nil {
			return nil, errors.New("insufficient_evidence carries violations or a corrective action it cannot ground")
		}
	default:
		return nil, fmt.Errorf(`"status" %q is not pass, defect or insufficient_evidence`, j.Claimed)
	}
	return j, nil
}

func trimIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// ground resolves the judgment's citations and requests against the frozen
// packet. A defect stands only on evidence the packet still supports: one
// whose every citation is unknown or stale -- or that cites nothing and only
// labels a suspicion -- is insufficient evidence. An unsupported suspicion is
// never laundered into a host fact.
func (j *Judgment) ground(p *EvidencePacket) {
	j.Grounded, j.Ungrounded, j.Expandable, j.Unsupplied = nil, nil, nil, nil
	for _, id := range j.Citations {
		if p.usable(id) {
			j.Grounded = append(j.Grounded, id)
		} else {
			j.Ungrounded = append(j.Ungrounded, id)
		}
	}
	for _, id := range j.RequestedEvidence {
		switch why := p.unexpandable(id); {
		case slices.Contains(j.Expandable, id):
		case why == "":
			j.Expandable = append(j.Expandable, id)
		default:
			j.Unsupplied = append(j.Unsupplied, id+": "+why)
		}
	}
	if j.Claimed == JudgeDefect && len(j.Grounded) == 0 {
		j.Status = JudgeInsufficientEvidence
		if len(j.Ungrounded) > 0 {
			j.Downgraded = "the defect's citations (" + strings.Join(j.Ungrounded, ", ") + ") resolve to no usable evidence"
		} else {
			j.Downgraded = "the defect is an unsupported suspicion"
		}
	}
}

// replyMembers is the exact member names of each object the reply contract
// has, read off judgeWire's json tags so the two cannot drift: a member's
// value is the nested object's names when it holds an object, nil when it
// holds none.
var replyMembers = membersOf(reflect.TypeOf(judgeWire{}))

type memberNames map[string]memberNames

func membersOf(t reflect.Type) memberNames {
	names := memberNames{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		ft := f.Type
		for ft.Kind() == reflect.Pointer || ft.Kind() == reflect.Slice {
			ft = ft.Elem()
		}
		if ft.Kind() == reflect.Struct {
			names[name] = membersOf(ft)
		} else {
			names[name] = nil
		}
	}
	return names
}

// checkMembers walks every object in a JSON document and refuses one whose
// member names are not exactly the contract's, or that repeats one. Exact
// matters: encoding/json maps "Status" onto the status field too, so a reply
// with "status":"insufficient_evidence" and "Status":"pass" passed an
// exact-duplicate check and was read as whichever came last (Docs/audits/
// verification-evidence-and-judge-contracts.md, follow-up 2026-10-28). An
// escaped spelling that decodes to a contract name is that name.
func checkMembers(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var walk func(names memberNames, where string) error
	walk = func(names memberNames, where string) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			if names == nil {
				return fmt.Errorf("%s holds an object; the contract has none there", where)
			}
			seen := map[string]bool{}
			for dec.More() {
				keyTok, err := dec.Token()
				if err != nil {
					return err
				}
				key, _ := keyTok.(string)
				nested, known := names[key]
				switch {
				case !known:
					return fmt.Errorf("%s has the member %q, which is not one of the contract's names (they are exact)", where, key)
				case seen[key]:
					return fmt.Errorf("the reply repeats the key %q", key)
				}
				seen[key] = true
				if err := walk(nested, fmt.Sprintf("%q", key)); err != nil {
					return err
				}
			}
		case '[':
			for dec.More() {
				if err := walk(names, where); err != nil {
					return err
				}
			}
		}
		_, err = dec.Token() // the closing delimiter
		return err
	}
	if err := walk(replyMembers, "the reply"); err != nil {
		var syntax *json.SyntaxError
		if errors.As(err, &syntax) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("not a %s object: %w", JudgeContract, err)
		}
		return err
	}
	return nil
}
