package campaign

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/atomicfile"
	"codenerd/internal/config"
	"codenerd/internal/deliberation"
	"codenerd/internal/evidence"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/types"
	"codenerd/internal/usage"

	"github.com/google/uuid"
)

var errMissionStopped = errors.New("mission stopped by policy")

// RecurseMission adds deliberation and receipts to the existing recurse driver.
// It is single-use; a new invocation has a new explicit resource envelope.
type RecurseMission struct {
	Text        string
	Limits      config.MissionConfig
	Deliberator *deliberation.Service
	State       MissionState
	path        string
	started     time.Time
	meter       missionMeter
}

type MissionState struct {
	ID             string               `json:"id"`
	Session        string               `json:"session"`
	Mission        string               `json:"mission"`
	Version        string               `json:"version"`
	Limits         config.MissionConfig `json:"limits"`
	Started        time.Time            `json:"started"`
	Status         string               `json:"status"`
	StopFacts      []string             `json:"stop_facts,omitempty"`
	Cycles         int                  `json:"cycles"`
	ElapsedSeconds int64                `json:"elapsed_seconds"`
	Stalls         int                  `json:"stalls"`
	SpendMicroUSD  int64                `json:"spend_microusd"`
	UnknownSpend   bool                 `json:"unknown_spend"`
	UsageReports   int                  `json:"usage_reports"`
	Prices         map[string]int64     `json:"price_versions_microusd,omitempty"`
	Question       string               `json:"question,omitempty"`
	Options        []string             `json:"options,omitempty"`
	Pending        *MissionReceipt      `json:"pending,omitempty"`
	Receipts       []MissionReceipt     `json:"receipts,omitempty"`
}

// MissionReceipt is historical evidence at an exact source snapshot. It never
// asserts that a free-text mission or a North Star requirement is satisfied.
type MissionReceipt struct {
	MissionID                string                  `json:"mission_id"`
	Version                  string                  `json:"version"`
	VisionVersion            string                  `json:"vision_version"`
	Mission                  string                  `json:"mission"`
	Cycle                    int                     `json:"cycle"`
	Node                     string                  `json:"node"`
	Scope                    []string                `json:"scope"`
	ProtectedPaths           []string                `json:"protected_paths"`
	Finding                  string                  `json:"finding,omitempty"`
	Thread                   string                  `json:"thread"`
	ThreadPath               string                  `json:"thread_path"`
	Design                   *deliberation.Candidate `json:"design,omitempty"`
	Check                    []string                `json:"check,omitempty"`
	OKExitCodes              []int                   `json:"ok_exit_codes,omitempty"`
	BaseRevision             string                  `json:"base_revision"`
	Before                   string                  `json:"before"`
	After                    string                  `json:"after,omitempty"`
	Outcome                  string                  `json:"outcome"`
	Evidence                 string                  `json:"evidence"`
	Commit                   string                  `json:"commit,omitempty"`
	Detail                   string                  `json:"detail,omitempty"`
	VerificationInputsRef    string                  `json:"verification_inputs_ref"`
	VerificationInputsDigest string                  `json:"verification_inputs_digest"`
	VerificationInputs       map[string]string       `json:"-"`
	OwnedWritesRef           string                  `json:"owned_writes_ref,omitempty"`
	ChecksRef                string                  `json:"checks_ref,omitempty"`
}

func (m *RecurseMission) begin(ctx context.Context, root string) (context.Context, context.CancelFunc, error) {
	if m.Deliberator == nil || m.Deliberator.Kernel == nil || strings.TrimSpace(m.Text) == "" || m.path != "" {
		return nil, nil, fmt.Errorf("mission requires a fresh conductor, explicit mission and deliberator")
	}
	serviceRoot, err := tools.CanonicalWorkspaceRoot(m.Deliberator.Workspace)
	if err != nil || filepath.Clean(serviceRoot) != filepath.Clean(root) {
		return nil, nil, fmt.Errorf("mission and deliberation workspaces differ")
	}
	m.Deliberator.Workspace = root
	m.Limits = m.Limits.WithDefaults()
	if problems := m.Limits.Check("mission"); len(problems) > 0 {
		return nil, nil, fmt.Errorf("invalid mission limits: %v", problems)
	}
	if err := config.EnsureParams(m.Deliberator.Kernel, m.Limits.Params()); err != nil {
		return nil, nil, err
	}
	missing, err := m.Deliberator.Kernel.Query("config_param_missing")
	if err != nil {
		return nil, nil, err
	}
	for _, row := range missing {
		if len(row.Args) > 0 && types.ExtractString(row.Args[0]) == "/mission" {
			return nil, nil, fmt.Errorf("mission policy parameter missing: %s", row.String())
		}
	}
	dir, err := sessionlog.SessionDir(root, m.Deliberator.Session)
	if err != nil {
		return nil, nil, err
	}
	if err := m.checkInterrupted(); err != nil {
		return nil, nil, err
	}
	m.started = time.Now()
	m.State = MissionState{ID: "mission-" + uuid.NewString(), Session: m.Deliberator.Session,
		Mission: m.Text, Version: fmt.Sprintf("%x", sha256.Sum256([]byte(m.Text))),
		Limits: m.Limits, Started: m.started.UTC(), Status: "running"}
	m.path = filepath.Join(dir, "missions", m.State.ID+".json")
	if err := m.save(); err != nil {
		return nil, nil, err
	}
	remaining, err := m.admit()
	if err != nil {
		return nil, nil, err
	}
	// The duration is a kernel output. Cancellation uses recurse's existing
	// owned-write compensation; no second timer loop owns effects.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(remaining)*time.Second)
	ctx = sessionlog.WithAgent(ctx, "mission", m.State.ID)
	ctx = usage.WithObserver(ctx, &m.meter)
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "plan", Text: "Mission: " + m.Text})
	return ctx, cancel, nil
}

func (m *RecurseMission) prepare(ctx context.Context, a *RecurseAttempt) (err error) {
	vision, err := m.loadVision()
	if err != nil {
		return fmt.Errorf("mission north star: %w", err)
	}
	visionBytes, err := json.Marshal(vision)
	if err != nil {
		return err
	}
	visionVersion := fmt.Sprintf("%x", sha256.Sum256(visionBytes))
	request := struct {
		Mission       string            `json:"mission"`
		Version       string            `json:"mission_version"`
		Attempt       *RecurseAttempt   `json:"attempt"`
		History       string            `json:"history"`
		Previous      *MissionReceipt   `json:"previous,omitempty"`
		Vision        *northstar.Vision `json:"north_star,omitempty"`
		VisionVersion string            `json:"vision_version"`
	}{Mission: m.Text, Version: m.State.Version, Attempt: a, History: m.path, Vision: vision, VisionVersion: visionVersion}
	if len(m.State.Receipts) > 0 {
		request.Previous = &m.State.Receipts[len(m.State.Receipts)-1]
	}
	body, err := json.Marshal(request)
	if err != nil {
		return err
	}
	f := types.Fact{Predicate: "mission_context_size", Args: []any{m.State.ID, int64(len(body))}}
	if err := m.Deliberator.Kernel.Assert(f); err != nil {
		return err
	}
	defer func() {
		if cleanupErr := m.Deliberator.Kernel.RetractExactFactsBatch([]types.Fact{f}); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	if _, err := m.admit(); err != nil {
		return err
	}
	before, err := evidence.Snapshot(ctx, m.Deliberator.Workspace)
	if err != nil {
		return err
	}
	receipt := &MissionReceipt{MissionID: m.State.ID, Version: m.State.Version, VisionVersion: visionVersion, Mission: m.Text,
		Cycle: a.Cycle, Node: a.Node.ID, Scope: append([]string(nil), a.Node.Paths...),
		ProtectedPaths: append([]string(nil), m.Limits.ProtectedPaths...), Finding: a.Finding.ID,
		Check: append([]string(nil), a.Check...), Before: before, Outcome: "deliberating", Evidence: "unknown"}
	receipt.OKExitCodes = append([]int(nil), a.OKExitCodes...)
	receipt.BaseRevision = a.BaseRevision
	inputs, err := evidence.VerificationInputs(ctx, m.Deliberator.Workspace)
	if err != nil {
		return err
	}
	inputBytes, err := json.Marshal(inputs)
	if err != nil {
		return err
	}
	inputDir := filepath.Join(filepath.Dir(m.path), m.State.ID)
	if err := os.MkdirAll(inputDir, 0700); err != nil {
		return err
	}
	receipt.VerificationInputsRef = filepath.Join(inputDir, fmt.Sprintf("inputs-%d.json", a.Cycle))
	receipt.VerificationInputsDigest = fmt.Sprintf("%x", sha256.Sum256(inputBytes))
	receipt.VerificationInputs = inputs
	if err := atomicfile.WriteFile(receipt.VerificationInputsRef, inputBytes, 0600); err != nil {
		return err
	}
	m.State.Pending = receipt
	if err := m.save(); err != nil {
		return err
	}
	thread := &deliberation.Thread{Request: string(body), Mode: "propose", DesignOnly: true,
		Intent: perception.Intent{Category: "/mutation", Verb: "/fix", Target: a.Node.ID}}
	_, _, reports := m.meter.snapshot()
	out, err := m.Deliberator.Run(ctx, thread)
	m.meter.phaseEnded(reports)
	receipt.Thread = thread.ID
	if dir, e := sessionlog.SessionDir(m.Deliberator.Workspace, m.Deliberator.Session); e == nil {
		receipt.ThreadPath = filepath.Join(dir, "deliberation", thread.ID+".json")
	}
	if err != nil {
		receipt.Detail = err.Error()
		return err
	}
	disposition := "/stopped"
	switch out.Action {
	case "proceed", "proceed_assuming":
		disposition = "/ready"
	case "ask_once":
		disposition = "/ask_once"
	}
	if out.Action == "ask_once" {
		m.State.Question, m.State.Options = out.Question, out.Options
		sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant, Text: out.Question + "\n" + strings.Join(out.Options, "\n")})
	}
	design := types.Fact{Predicate: "mission_design", Args: []any{m.State.ID, types.MangleAtom(disposition)}}
	if err := m.Deliberator.Kernel.Assert(design); err != nil {
		return err
	}
	if _, err := m.admit(); err != nil {
		receipt.Outcome = disposition
		m.State.Receipts = append(m.State.Receipts, *receipt)
		m.State.Pending = nil // read-only deliberation cannot leave effects
		if saveErr := m.save(); saveErr != nil {
			return errors.Join(err, saveErr)
		}
		return err
	}
	if out.Candidate == nil {
		return fmt.Errorf("mission design has no candidate")
	}
	receipt.Design = out.Candidate
	current, err := evidence.Snapshot(ctx, m.Deliberator.Workspace)
	if err != nil {
		return err
	}
	currentVision, err := m.loadVision()
	if err != nil {
		return err
	}
	currentVisionBytes, err := json.Marshal(currentVision)
	if err != nil {
		return err
	}
	if err := m.Deliberator.Kernel.Assert(types.Fact{Predicate: "mission_source", Args: []any{m.State.ID, before, current, visionVersion, fmt.Sprintf("%x", sha256.Sum256(currentVisionBytes))}}); err != nil {
		return err
	}
	if _, err := m.admit(); err != nil {
		return err
	}
	rows, err := m.Deliberator.Kernel.Query("mission_execute")
	if err != nil {
		return err
	}
	if !missionHas(rows, m.State.ID) {
		return fmt.Errorf("mission design has no execution admission")
	}
	receipt.Outcome = "executing"
	a.Mission = receipt
	if err := m.save(); err != nil {
		return err
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: "step", Text: "Mission design: " + out.Candidate.Brief})
	return nil
}

func (m *RecurseMission) finish(ctx context.Context, rec recurseRecord, after string, witness string) (err error) {
	p := m.State.Pending
	if p == nil {
		return fmt.Errorf("mission cycle has no durable intent")
	}
	p.After, p.Outcome, p.Commit, p.Detail = after, rec.Outcome, rec.Commit, strings.TrimSpace(rec.Why+" "+rec.Detail)
	f := types.Fact{Predicate: "mission_observation", Args: []any{m.State.ID, int64(p.Cycle), types.MangleAtom(rec.Outcome), types.MangleAtom(witness), p.Before, p.After}}
	if err := m.Deliberator.Kernel.Assert(f); err != nil {
		return err
	}
	defer func() {
		if cleanupErr := m.Deliberator.Kernel.RetractExactFactsBatch([]types.Fact{f}); cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	rows, err := m.Deliberator.Kernel.Query("mission_evidence")
	if err != nil {
		return err
	}
	var dispositions []string
	for _, row := range rows {
		if len(row.Args) == 3 && types.ExtractString(row.Args[0]) == m.State.ID && argInt(row.Args[1]) == p.Cycle {
			dispositions = append(dispositions, types.ExtractString(row.Args[2]))
		}
	}
	if len(dispositions) != 1 {
		return fmt.Errorf("mission evidence requires one disposition, got %v", dispositions)
	}
	p.Evidence = dispositions[0]
	m.State.Cycles++
	// Streak is an observation over ratchet outcomes; policy decides its bound.
	if rec.Outcome == outcomeKept {
		m.State.Stalls = 0
	} else {
		m.State.Stalls++
	}
	m.State.Receipts = append(m.State.Receipts, *p)
	m.State.Pending = nil
	if err := m.Deliberator.Kernel.Retract("mission_design"); err != nil {
		return err
	}
	if err := m.Deliberator.Kernel.Retract("mission_source"); err != nil {
		return err
	}
	if err := m.save(); err != nil {
		return err
	}
	sessionlog.EmitLiveProgress(ctx, sessionlog.LiveEvent{Kind: sessionlog.EventAssistant,
		Text: fmt.Sprintf("Mission cycle %d: %s; evidence %s", rec.Cycle, rec.Outcome, p.Evidence)})
	return nil
}
