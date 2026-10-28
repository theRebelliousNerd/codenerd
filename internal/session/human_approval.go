package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"codenerd/internal/approval"
	"codenerd/internal/jit/config"
	"codenerd/internal/logging"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// Request-bound human approval on the executor's path
// (Docs/audits/request-bound-human-approval.md §5).
//
// A denied call the constitution marks human_approval_required is frozen into
// an approval request: the exact action, target and payload the gate saw, the
// tool registration it resolves to, and a digest of every file it names. The
// call then blocks — no model turn runs, no kernel fact or lease is held —
// until the person decides. On approval the operation is rebuilt from the
// world as it is now; any difference supersedes the request. The claim is
// taken once, the gate reruns with human_approval_granted asserted for this
// exact pending_action, and the call continues through every later gate and
// the ordinary backend. Nothing here is a second executor.

// humanApprovalAdmission serializes every admission that carries a human
// approval against every admission that does not. permitted/3 is keyed by
// (Action, Target, Payload), so while an approved admission's fact is
// asserted, a concurrent call with a byte-identical tuple could otherwise
// observe the same permitted row. Ordinary admissions share the read lock and
// never wait on each other.
var humanApprovalAdmission sync.RWMutex

// humanApproval is the claimed approval an admission carries into the gate.
type humanApproval struct {
	grant   *approval.Grant
	subject approval.Subject
}

// SetApprovalBroker installs the process's human-approval broker. Nil leaves
// every approvable denial a refusal, which is what a headless run is.
func (e *Executor) SetApprovalBroker(b *approval.Broker) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.approvals = b
}

func (e *Executor) approvalBroker() *approval.Broker {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.approvals
}

// gatePayload is the payload string the constitution is asked about: the
// canonical generated arguments when there are some, the JSON of the call's
// arguments otherwise. checkSafetyWithLease and the approval subject both use
// it, so the reviewed payload is byte-for-byte the admitted one.
func gatePayload(call ToolCall, canonicalPayload string) (string, error) {
	if canonicalPayload != "" {
		return canonicalPayload, nil
	}
	args := call.Args
	if args == nil {
		args = map[string]any{}
	}
	b, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func gateAction(name string) string {
	if strings.HasPrefix(name, "/") {
		return name
	}
	return "/" + name
}

// fileRevision is one target read once: its revision digest and, when small
// enough to preview, its bytes.
type fileRevision struct {
	path    string
	digest  string
	content []byte
	exists  bool
	isDir   bool
	size    int64
}

func readRevision(path string, keepBytes int) (fileRevision, error) {
	rev := fileRevision{path: path}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		rev.digest = "absent"
		return rev, nil
	}
	if err != nil {
		return rev, err
	}
	rev.exists = true
	if info.IsDir() {
		rev.digest, rev.isDir = "dir", true
		return rev, nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return rev, err
		}
		sum := sha256.Sum256([]byte(target))
		rev.digest = "symlink:sha256:" + hex.EncodeToString(sum[:])
		return rev, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return rev, err
	}
	defer f.Close()
	h := sha256.New()
	var kept strings.Builder
	n, err := io.Copy(io.MultiWriter(h, limitWriter{&kept, keepBytes + 1}), f)
	if err != nil {
		return rev, err
	}
	rev.size = n
	rev.digest = "sha256:" + hex.EncodeToString(h.Sum(nil))
	if n <= int64(keepBytes) {
		rev.content = []byte(kept.String())
	}
	return rev, nil
}

// limitWriter keeps the first n bytes and discards the rest without error, so
// the hash still sees the whole file.
type limitWriter struct {
	b *strings.Builder
	n int
}

func (w limitWriter) Write(p []byte) (int, error) {
	if room := w.n - w.b.Len(); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		w.b.Write(p[:room])
	}
	return len(p), nil
}

// approvalAgent is the requesting agent's identity for the subject.
func approvalAgent(cfg *config.EffectiveAgentRuntimeConfig) string {
	if cfg != nil && strings.TrimSpace(cfg.Persona) != "" {
		return cfg.Persona
	}
	return "session"
}

// toolRegistrationDigest identifies the modular registration a call resolves
// to: re-registering the tool with another schema or description is a
// different tool, and an approval of the old one does not carry over.
func toolRegistrationDigest(name string) (string, error) {
	tool := tools.Global().Get(name)
	if tool == nil {
		return "", fmt.Errorf("tool %s has no modular registration", name)
	}
	encoded, err := json.Marshal(struct {
		Name, Description string
		Schema            tools.ToolSchema
	}{tool.Name, tool.Description, tool.Schema})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// approvalSubject freezes the call as the world stands now. It is computed
// once when the request opens and again when the approved call resumes; any
// difference between the two is a stale approval.
func (e *Executor) approvalSubject(call ToolCall, cfg *config.EffectiveAgentRuntimeConfig, canonicalPayload string, keepBytes int) (approval.Subject, []fileRevision, error) {
	payload, err := gatePayload(call, canonicalPayload)
	if err != nil {
		return approval.Subject{}, nil, err
	}
	version, err := toolRegistrationDigest(call.Name)
	if err != nil {
		return approval.Subject{}, nil, err
	}
	effect, err := tools.LookupEffect(call.Name)
	if err != nil {
		return approval.Subject{}, nil, err
	}
	workspace := e.workspaceForVerification()
	paths, err := projectdoc.TargetPaths(call.Args)
	if err != nil {
		return approval.Subject{}, nil, err
	}
	sort.Strings(paths)
	var revisions []approval.SourceRevision
	var reads []fileRevision
	for _, p := range paths {
		phys := p
		if !filepath.IsAbs(phys) && workspace != "" {
			phys = filepath.Join(workspace, phys)
		}
		rev, err := readRevision(filepath.Clean(phys), keepBytes)
		if err != nil {
			return approval.Subject{}, nil, fmt.Errorf("read %s for the approval request: %w", p, err)
		}
		rev.path = p
		reads = append(reads, rev)
		revisions = append(revisions, approval.SourceRevision{Path: p, Digest: rev.digest})
	}
	e.mu.RLock()
	sessionID := e.sessionID
	e.mu.RUnlock()
	return approval.Subject{
		Workspace:       workspace,
		Session:         sessionID,
		Agent:           approvalAgent(cfg),
		Backend:         "modular",
		Tool:            call.Name,
		ToolVersion:     version,
		Action:          gateAction(call.Name),
		Target:          e.extractTarget(call.Args),
		Payload:         payload,
		Effect:          string(effect),
		SourceRevisions: revisions,
	}, reads, nil
}

// approvalPreview is the operation-specific explanation a person approves.
// A deletion shows the content that goes away; anything else shows its
// arguments and says plainly that its effects beyond them are not modeled.
func approvalPreview(call ToolCall, subject approval.Subject, reads []fileRevision, maxBytes int) (approval.Preview, error) {
	in := approval.PreviewInput{Arguments: subject.Payload, MaxBytes: maxBytes}
	if call.Name == "delete_file" && len(reads) == 1 {
		r := reads[0]
		in.Summary = fmt.Sprintf("Delete %s", r.path)
		switch {
		case r.isDir:
			in.Effects = []string{fmt.Sprintf("removes the directory %s", r.path)}
		case !r.exists:
			in.Effects = []string{fmt.Sprintf("%s does not exist now; the delete will fail", r.path)}
		default:
			in.Effects = []string{
				fmt.Sprintf("removes %s (%d bytes)", r.path, r.size),
				"git cannot restore it: the file is untracked, has uncommitted changes, or is outside a repository",
			}
		}
		change := &approval.FileChange{Path: r.path, Exists: r.exists, IsDir: r.isDir, Before: r.content}
		if r.exists && !r.isDir && r.content == nil {
			// Over the preview bound: say so rather than show a partial file.
			change.Before = make([]byte, r.size)
		}
		in.File = change
		return approval.BuildPreview(in)
	}
	in.Summary = fmt.Sprintf("Run %s on %s", call.Name, subject.Target)
	in.Effects = []string{fmt.Sprintf("%s effect on %s", subject.Effect, subject.Target)}
	in.Unknowns = []string{fmt.Sprintf("what %s does beyond the arguments shown is not modeled", call.Name)}
	return approval.BuildPreview(in)
}

// awaitHumanApproval opens a request for the denied call, blocks for the
// person's decision, and claims the approval against the call as it stands
// when it resumes. Every refusal comes back as the tool error the model sees.
func (e *Executor) awaitHumanApproval(ctx context.Context, call ToolCall, cfg *config.EffectiveAgentRuntimeConfig, canonicalPayload, denial string) (*humanApproval, error) {
	broker := e.approvalBroker()
	action := gateAction(call.Name)
	target := e.extractTarget(call.Args)
	if !broker.HostAttached() && !broker.Autonomous() {
		return nil, fmt.Errorf("tool call blocked by safety gate: action %s target %s denied (%s); a person may approve this exact request, but no approval host is attached to this run — do not retry the same call; ask the user to perform it or to run interactively", call.Name, target, denial)
	}
	keep := broker.PreviewMaxBytes()
	subject, reads, err := e.approvalSubject(call, cfg, canonicalPayload, keep)
	if err != nil {
		return nil, fmt.Errorf("tool call blocked by safety gate: action %s target %s denied (%s); the approval request could not be frozen: %w", call.Name, target, denial, err)
	}
	preview, err := approvalPreview(call, subject, reads, keep)
	if err != nil {
		return nil, fmt.Errorf("tool call blocked by safety gate: approval preview for %s failed: %w", call.Name, err)
	}
	intent := "exec-" + call.ID
	if broker.Autonomous() {
		logging.Get(logging.CategorySession).Info("Yolo approves %s %s (denied for: %s)", call.Name, target, denial)
	} else {
		logging.Get(logging.CategorySession).Info("Awaiting human approval for %s %s", call.Name, target)
	}
	grant, req, err := broker.Await(ctx, intent, subject, preview)
	if err != nil {
		if errors.Is(err, approval.ErrRejected) {
			e.assertSecurityViolation(types.MangleAtom(action), "human rejected approval request "+req.ID)
		}
		return nil, fmt.Errorf("tool call blocked: approval request %s for %s %s did not authorize it: %w — do not retry the same call; it needs a new decision", req.ID, call.Name, target, err)
	}
	// Revalidate: the operation that runs is the one the person reviewed,
	// rebuilt from the world as it is now.
	current, _, err := e.approvalSubject(call, cfg, canonicalPayload, 0)
	if err != nil {
		grant.AdmissionFailed("could not rebuild the operation after approval: " + err.Error())
		return nil, fmt.Errorf("tool call blocked: approved request %s could not be revalidated: %w", req.ID, err)
	}
	if err := grant.Reserve(intent, current); err != nil {
		return nil, fmt.Errorf("tool call blocked: approved request %s cannot run: %w — it was approved for the operation as it was, not as it is", req.ID, err)
	}
	logging.Get(logging.CategorySession).Info("Human approval %s claimed for %s %s (decided by %s)", grant.RequestID(), call.Name, target, req.Decision.Actor)
	return &humanApproval{grant: grant, subject: current}, nil
}

// assertHumanApprovalFact asserts the consent fact for the exact pending
// action being admitted. The caller retracts it with that pending_action.
func (e *Executor) humanApprovalFact(actionID string, actionAtom types.MangleAtom, target, payload string, approved *humanApproval) (types.Fact, error) {
	if approved.subject.Action != string(actionAtom) || approved.subject.Target != target || approved.subject.Payload != payload {
		return types.Fact{}, fmt.Errorf("the admitted call does not match the approved request")
	}
	return types.Fact{
		Predicate: types.HumanApprovalPredicate,
		Args:      []any{actionID, actionAtom, target, payload, approved.grant.Digest()},
	}, nil
}
