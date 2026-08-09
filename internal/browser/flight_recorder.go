package browser

// The bounded flight recorder is adapted from BrowserNERD's Apache-2.0
// evidence contract. codeNERD keeps traces under its native .nerd authority
// root and records only already-redacted browser evidence. See
// THIRD_PARTY_NOTICES.md.

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	browsersecurity "codenerd/internal/browser/security"
	"codenerd/internal/mangle"
)

const (
	defaultEvidenceDir       = ".nerd/browser/traces"
	maxFlightRecordBytes     = 64 << 10
	maxFlightReadItems       = 100
	maxFlightExportItems     = 1000
	maxFlightReadScanBytes   = 8 << 20
	maxFlightScannerCapacity = 128 << 10
)

// FlightEvent is one privacy-safe JSONL record.
type FlightEvent struct {
	TimestampMS int64  `json:"timestamp_ms"`
	Type        string `json:"type"`
	SessionID   string `json:"session_id"`
	Data        any    `json:"data,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

// FlightReadOptions controls bounded trace reads.
type FlightReadOptions struct {
	SinceMS  int64
	Types    []string
	MaxItems int
}

// FlightReadResult preserves the scope and truncation of a trace read.
type FlightReadResult struct {
	SessionID    string        `json:"session_id"`
	Events       []FlightEvent `json:"events"`
	FilesRead    int           `json:"files_read"`
	ScannedBytes int64         `json:"scanned_bytes"`
	Truncated    bool          `json:"truncated"`
}

// FlightRecorder owns rotated, owner-only JSONL traces for one workspace.
type FlightRecorder struct {
	mu          sync.Mutex
	dir         string
	maxFiles    int
	maxFileSize int64
	redactor    *browsersecurity.Redactor
}

// NewFlightRecorder creates a confined workspace recorder.
func NewFlightRecorder(cfg Config, policy *browsersecurity.PathPolicy, redactor *browsersecurity.Redactor) (*FlightRecorder, error) {
	if policy == nil {
		return nil, fmt.Errorf("browser output path policy is not configured")
	}
	dir := strings.TrimSpace(cfg.EvidenceDir)
	if dir == "" {
		dir = defaultEvidenceDir
	}
	probe, err := policy.ResolveForWrite("", dir, "flight.jsonl")
	if err != nil {
		return nil, fmt.Errorf("resolve evidence directory: %w", err)
	}
	dir = filepath.Dir(probe)
	if err := browsersecurity.EnsurePrivateDir(dir); err != nil {
		return nil, fmt.Errorf("create evidence directory: %w", err)
	}
	if redactor == nil {
		redactor = browsersecurity.NewRedactor(cfg.ExtraSensitiveKeys)
	}
	return &FlightRecorder{
		dir: dir, maxFiles: cfg.GetMaxEvidenceFiles(),
		maxFileSize: cfg.GetMaxEvidenceFileBytes(), redactor: redactor,
	}, nil
}

// Record appends one bounded, redacted event.
func (r *FlightRecorder) Record(sessionID, eventType string, data any) (string, error) {
	if r == nil {
		return "", fmt.Errorf("browser flight recorder is disabled")
	}
	sessionID = strings.TrimSpace(sessionID)
	eventType = normalizeFlightType(eventType)
	if sessionID == "" || eventType == "" {
		return "", fmt.Errorf("session_id and event type are required")
	}
	event := FlightEvent{
		TimestampMS: time.Now().UnixMilli(), Type: eventType, SessionID: sessionID,
		Data: r.redactor.Sanitize(data),
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("encode browser evidence: %w", err)
	}
	if len(encoded) > maxFlightRecordBytes {
		digest := sha256.Sum256(encoded)
		event.Data = map[string]any{
			"summary": "event exceeded recorder row limit", "original_bytes": len(encoded),
			"sha256": hex.EncodeToString(digest[:]),
		}
		event.Truncated = true
		encoded, err = json.Marshal(event)
		if err != nil {
			return "", fmt.Errorf("encode truncated browser evidence: %w", err)
		}
	}
	encoded = append(encoded, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()
	path := r.sessionPath(sessionID)
	if err := r.rotateIfNeeded(path, int64(len(encoded))); err != nil {
		return "", err
	}
	_, statErr := os.Stat(path)
	newFile := os.IsNotExist(statErr)
	if statErr != nil && !newFile {
		return "", fmt.Errorf("inspect browser evidence: %w", statErr)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open browser evidence: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("append browser evidence: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close browser evidence: %w", err)
	}
	if newFile {
		if err := browsersecurity.ProtectPrivateFile(path); err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("protect browser evidence: %w", err)
		}
	}
	if err := r.pruneLocked(); err != nil {
		return "", err
	}
	return path, nil
}

// Read returns the newest matching events under hard item and byte ceilings.
func (r *FlightRecorder) Read(sessionID string, opts FlightReadOptions) (FlightReadResult, error) {
	result := FlightReadResult{SessionID: strings.TrimSpace(sessionID)}
	if r == nil {
		return result, fmt.Errorf("browser flight recorder is disabled")
	}
	if result.SessionID == "" {
		return result, fmt.Errorf("session_id is required")
	}
	maxItems := opts.MaxItems
	if maxItems <= 0 {
		maxItems = 20
	}
	if maxItems > maxFlightReadItems {
		maxItems = maxFlightReadItems
	}
	typeFilter := make(map[string]struct{}, len(opts.Types))
	for _, value := range opts.Types {
		if normalized := normalizeFlightType(value); normalized != "" {
			typeFilter[normalized] = struct{}{}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	paths, err := r.sessionFilesLocked(result.SessionID)
	if err != nil {
		return result, err
	}
	limit := maxItems + 1
	for _, path := range paths {
		if result.ScannedBytes >= maxFlightReadScanBytes {
			result.Truncated = true
			break
		}
		fileEvents, scanned, fileTruncated, readErr := readFlightFile(path, result.SessionID, opts.SinceMS, typeFilter, limit, maxFlightReadScanBytes-result.ScannedBytes)
		result.ScannedBytes += scanned
		if readErr != nil {
			return result, readErr
		}
		result.FilesRead++
		result.Truncated = result.Truncated || fileTruncated
		result.Events = append(fileEvents, result.Events...)
		if len(result.Events) > limit {
			result.Events = result.Events[len(result.Events)-limit:]
		}
		if len(result.Events) >= limit {
			result.Truncated = true
			break
		}
	}
	if len(result.Events) > maxItems {
		result.Events = result.Events[len(result.Events)-maxItems:]
		result.Truncated = true
	}
	return result, nil
}

// Export writes a bounded JSONL selection to a confined owner-only file.
func (r *FlightRecorder) Export(policy *browsersecurity.PathPolicy, sessionID, requested string, opts FlightReadOptions) (string, FlightReadResult, error) {
	if opts.MaxItems <= 0 || opts.MaxItems > maxFlightExportItems {
		opts.MaxItems = maxFlightExportItems
	}
	result, err := r.Read(sessionID, opts)
	if err != nil {
		return "", result, err
	}
	name := fmt.Sprintf("flight_%s_%d.jsonl", safeFlightSession(sessionID), time.Now().UnixNano())
	path, err := policy.ResolveForWrite(requested, filepath.Join(r.dir, "exports"), name)
	if err != nil {
		return "", result, err
	}
	if err := browsersecurity.EnsurePrivateDir(filepath.Dir(path)); err != nil {
		return "", result, err
	}
	var output strings.Builder
	for _, event := range result.Events {
		encoded, encodeErr := json.Marshal(event)
		if encodeErr != nil {
			return "", result, encodeErr
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	if err := browsersecurity.WritePrivateFileExclusive(path, []byte(output.String())); err != nil {
		return "", result, err
	}
	return path, result, nil
}

func (r *FlightRecorder) sessionPath(sessionID string) string {
	return filepath.Join(r.dir, "flight_"+safeFlightSession(sessionID)+".jsonl")
}

func (r *FlightRecorder) rotateIfNeeded(path string, incoming int64) error {
	info, err := os.Stat(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect browser evidence: %w", err)
	}
	if err != nil || info.Size()+incoming <= r.maxFileSize {
		return nil
	}
	rotated := strings.TrimSuffix(path, filepath.Ext(path)) + fmt.Sprintf("_%d.jsonl", time.Now().UnixNano())
	if err := os.Rename(path, rotated); err != nil {
		return fmt.Errorf("rotate browser evidence: %w", err)
	}
	return nil
}

func (r *FlightRecorder) pruneLocked() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("list browser evidence: %w", err)
	}
	type traceFile struct {
		path string
		mod  time.Time
	}
	files := make([]traceFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "flight_") || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil {
			files = append(files, traceFile{path: filepath.Join(r.dir, entry.Name()), mod: info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	for index := r.maxFiles; index < len(files); index++ {
		if err := os.Remove(files[index].path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("prune browser evidence: %w", err)
		}
	}
	return nil
}

// sessionFilesLocked returns newest files first.
func (r *FlightRecorder) sessionFilesLocked(sessionID string) ([]string, error) {
	prefix := "flight_" + safeFlightSession(sessionID)
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, fmt.Errorf("list browser evidence: %w", err)
	}
	type candidate struct {
		path string
		mod  time.Time
	}
	files := make([]candidate, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !(name == prefix+".jsonl" || strings.HasPrefix(name, prefix+"_")) || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr == nil {
			files = append(files, candidate{path: filepath.Join(r.dir, entry.Name()), mod: info.ModTime()})
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	paths := make([]string, len(files))
	for index := range files {
		paths[index] = files[index].path
	}
	return paths, nil
}

func readFlightFile(path, sessionID string, sinceMS int64, types map[string]struct{}, limit int, byteBudget int64) ([]FlightEvent, int64, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, false, fmt.Errorf("open browser evidence: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64<<10), maxFlightScannerCapacity)
	events := make([]FlightEvent, 0, limit)
	var scanned int64
	truncated := false
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		scanned += int64(len(line) + 1)
		if scanned > byteBudget {
			truncated = true
			break
		}
		var event FlightEvent
		if err := json.Unmarshal(line, &event); err != nil {
			truncated = true
			continue
		}
		if event.SessionID != sessionID || event.TimestampMS < sinceMS {
			continue
		}
		if len(types) > 0 {
			if _, ok := types[event.Type]; !ok {
				continue
			}
		}
		events = append(events, event)
		if len(events) > limit {
			events = events[len(events)-limit:]
			truncated = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, scanned, true, fmt.Errorf("scan browser evidence: %w", err)
	}
	return events, scanned, truncated, nil
}

func safeFlightSession(sessionID string) string {
	var clean strings.Builder
	for _, char := range sessionID {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			clean.WriteRune(char)
		}
	}
	if clean.Len() > 0 && clean.Len() <= 80 && clean.String() == sessionID {
		return clean.String()
	}
	digest := sha256.Sum256([]byte(sessionID))
	return hex.EncodeToString(digest[:8])
}

func normalizeFlightType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var result strings.Builder
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			result.WriteRune(char)
		}
	}
	return result.String()
}

var recordedFlightPredicates = map[string]struct{}{
	"navigation_event": {}, "console_event": {}, "net_request": {}, "net_response": {},
	"net_failure": {}, "toast_notification": {}, "click_event": {}, "input_event": {},
	"state_change": {}, "dom_updated": {}, "browser_page_state": {},
}

type flightFact struct {
	Predicate   string `json:"predicate"`
	Args        []any  `json:"args"`
	TimestampMS int64  `json:"timestamp_ms"`
}

func (m *SessionManager) recordFlightFacts(facts []mangle.Fact) {
	if m == nil || m.recorder == nil {
		return
	}
	bySession := make(map[string][]flightFact)
	for _, fact := range facts {
		if _, ok := recordedFlightPredicates[fact.Predicate]; !ok || len(fact.Args) == 0 {
			continue
		}
		sessionID := strings.TrimSpace(fmt.Sprint(fact.Args[0]))
		if sessionID == "" {
			continue
		}
		timestamp := fact.Timestamp.UnixMilli()
		if fact.Timestamp.IsZero() {
			timestamp = time.Now().UnixMilli()
		}
		bySession[sessionID] = append(bySession[sessionID], flightFact{
			Predicate: fact.Predicate, Args: append([]any(nil), fact.Args...), TimestampMS: timestamp,
		})
	}
	for sessionID, rows := range bySession {
		if _, err := m.recorder.Record(sessionID, "facts", map[string]any{"facts": rows}); err != nil {
			// Evidence is diagnostic and must not make the browser effect fail.
			continue
		}
	}
}

// RecordEvidence appends a bounded tool- or action-level evidence event.
func (m *SessionManager) RecordEvidence(sessionID, eventType string, data any) (string, error) {
	if m == nil || m.recorder == nil {
		return "", fmt.Errorf("browser flight recorder is disabled")
	}
	return m.recorder.Record(sessionID, eventType, data)
}

// ReadEvidence reads a bounded flight-recorder view.
func (m *SessionManager) ReadEvidence(sessionID string, opts FlightReadOptions) (FlightReadResult, error) {
	if m == nil || m.recorder == nil {
		return FlightReadResult{SessionID: sessionID}, fmt.Errorf("browser flight recorder is disabled")
	}
	return m.recorder.Read(sessionID, opts)
}

// ExportEvidence writes a bounded, owner-only trace selection.
func (m *SessionManager) ExportEvidence(sessionID, requested string, opts FlightReadOptions) (string, FlightReadResult, error) {
	if m == nil || m.recorder == nil {
		return "", FlightReadResult{SessionID: sessionID}, fmt.Errorf("browser flight recorder is disabled")
	}
	return m.recorder.Export(m.pathPolicy, sessionID, requested, opts)
}

// EvidenceEnabled reports whether this manager has a usable recorder.
func (m *SessionManager) EvidenceEnabled() bool { return m != nil && m.recorder != nil }
