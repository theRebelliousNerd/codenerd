package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// This file is the wire boundary of the MCP client: it turns bytes a remote
// server sent into typed observations, and refuses to turn anything it cannot
// interpret into a zero-value success. Transports own moving bytes; this file
// owns what the bytes mean. Connectivity, protocol compatibility, capability
// observation, catalog coverage and tool outcome are different claims, and none
// of them is allowed to stand in for another.

// The one protocol profile this client implements: the initialization
// handshake of the 2024-11-05 revision (initialize request, initialize result
// naming a version and object-valued capabilities, then the initialized
// notification). Revisions that change the lifecycle or the HTTP binding (the
// 2025 session header, the 2026 per-request metadata) are separate profiles and
// are deliberately not listed: a server that answers with another version is
// refused with an UnsupportedProtocolError instead of being half-spoken to.
const protocolVersionLegacyInit = "2024-11-05"

var supportedProtocolVersions = []string{protocolVersionLegacyInit}

// maxWireMessageBytes bounds one JSON-RPC message, one SSE event and one HTTP
// response body. Exceeding it is an explicit transport error, never a cut: a
// truncated JSON document is not a smaller result.
var maxWireMessageBytes = 64 << 20

// Catalog collection bounds. A server that never stops paging, or pages
// forever through a repeating cursor, yields an explicit incomplete catalog.
const (
	maxCatalogPages = 1000
	maxCatalogItems = 200000
)

// maxDiagnosticBody bounds the remote payload quoted into an error message.
const maxDiagnosticBody = 512

// UnsupportedProtocolError reports a server that answered initialize with a
// protocol version this client does not implement.
type UnsupportedProtocolError struct {
	Requested string
	Returned  string
}

func (e *UnsupportedProtocolError) Error() string {
	return fmt.Sprintf("MCP server selected protocol version %q; this client implements %s (requested %q) — configure a server that supports it",
		e.Returned, strings.Join(supportedProtocolVersions, ", "), e.Requested)
}

// IncompleteCatalogError reports a list operation that stopped before the
// server said it was exhausted. The items collected so far are returned
// alongside it by the list helpers; absence from a partial list is not
// evidence that the server withdrew anything.
type IncompleteCatalogError struct {
	Kind      string // "tools", "resources" or "prompts"
	Collected int
	Pages     int
	Reason    string
	Cause     error
}

func (e *IncompleteCatalogError) Error() string {
	msg := fmt.Sprintf("incomplete %s catalog: %d item(s) over %d page(s), %s", e.Kind, e.Collected, e.Pages, e.Reason)
	if e.Cause != nil {
		msg += ": " + e.Cause.Error()
	}
	return msg
}

func (e *IncompleteCatalogError) Unwrap() error { return e.Cause }

// rpcMessage is any JSON-RPC 2.0 message as it arrives. Which of the four
// shapes it is — request, notification, response or garbage — is decided by
// kind(), not assumed by the caller.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitzero"`
	Method  string          `json:"method,omitzero"`
	Params  json.RawMessage `json:"params,omitzero"`
	Result  json.RawMessage `json:"result,omitzero"`
	Error   *mcpError       `json:"error,omitzero"`
}

type rpcKind int

const (
	rpcInvalid rpcKind = iota
	rpcResponse
	rpcRequest
	rpcNotification
)

func (k rpcKind) String() string {
	switch k {
	case rpcResponse:
		return "response"
	case rpcRequest:
		return "request"
	case rpcNotification:
		return "notification"
	}
	return "invalid message"
}

func rawIsNull(raw json.RawMessage) bool {
	return len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func decodeRPCMessage(data []byte) (*rpcMessage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		return nil, errors.New("JSON-RPC batch messages are not supported by this client")
	}
	var msg rpcMessage
	if err := json.Unmarshal(trimmed, &msg); err != nil {
		return nil, fmt.Errorf("malformed JSON-RPC message: %w", err)
	}
	if msg.JSONRPC != "2.0" {
		return nil, fmt.Errorf("malformed JSON-RPC message: jsonrpc is %q, want \"2.0\"", msg.JSONRPC)
	}
	return &msg, nil
}

func (m *rpcMessage) kind() rpcKind {
	switch {
	case m.Method != "" && !rawIsNull(m.ID):
		return rpcRequest
	case m.Method != "":
		return rpcNotification
	case !rawIsNull(m.Result) && m.Error == nil:
		return rpcResponse
	case rawIsNull(m.Result) && m.Error != nil:
		return rpcResponse
	}
	return rpcInvalid
}

// intID reads the message ID as the integer this client issued. A string,
// fractional or null ID is not one of ours.
func (m *rpcMessage) intID() (int, bool) {
	if rawIsNull(m.ID) {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(m.ID)), 10, 64)
	if err != nil || n < 0 || n > int64(^uint(0)>>1) {
		return 0, false
	}
	return int(n), true
}

// response converts a classified response into the form pending-call channels
// carry.
func (m *rpcMessage) response(id int) *mcpResponse {
	return &mcpResponse{JSONRPC: m.JSONRPC, ID: id, Result: m.Result, Error: m.Error}
}

// responseFor validates that raw is the terminal response to request id and
// returns it. A wrong ID, a request or notification where a response belongs,
// and a malformed envelope are all errors; none is a success.
func responseFor(id int, data []byte) (*mcpResponse, error) {
	msg, err := decodeRPCMessage(data)
	if err != nil {
		return nil, err
	}
	if msg.kind() != rpcResponse {
		return nil, fmt.Errorf("expected a response to request %d, got a %s", id, msg.kind())
	}
	got, ok := msg.intID()
	if !ok {
		if msg.Error != nil {
			// A server that could not even read the request answers with a
			// null ID; its error is still the answer.
			return msg.response(id), nil
		}
		return nil, fmt.Errorf("response to request %d carries unusable id %s", id, string(msg.ID))
	}
	if got != id {
		return nil, fmt.Errorf("response id %d does not match request id %d", got, id)
	}
	return msg.response(id), nil
}

// readBounded reads at most limit bytes and fails when the source has more.
func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds the %d byte wire limit", limit)
	}
	return data, nil
}

// diagnosticBody quotes a remote payload into an error message: bounded and
// redacted, because server error bodies echo credentials.
func diagnosticBody(r io.Reader) string {
	data, _ := io.ReadAll(io.LimitReader(r, maxDiagnosticBody+1))
	return redactForLog(string(data), maxDiagnosticBody)
}

// refuseCrossHostRedirect keeps configured credentials (static headers) bound
// to the configured host: a redirect to another host is an error, not a silent
// forward.
func refuseCrossHostRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("refusing redirect from %s to %s: configured credentials stay bound to the configured host", via[0].URL.Host, req.URL.Host)
	}
	return nil
}

// --- initialize -------------------------------------------------------------

func initializeParams() map[string]any {
	return map[string]any{
		"protocolVersion": protocolVersionLegacyInit,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "codeNERD",
			"version": "1.0.0",
		},
	}
}

// parseInitializeResult decodes the initialize result by the specification's
// shape: a protocol version this client implements and an object of
// object-valued capabilities. A capability that is present is an object even
// when it is empty; a boolean, string or null is malformed. Unknown
// capabilities are forward-compatible and ignored. Nothing here falls back to
// "no capabilities".
func parseInitializeResult(raw json.RawMessage) (*MCPCapabilities, error) {
	if rawIsNull(raw) {
		return nil, errors.New("initialize result is empty")
	}
	var envelope struct {
		ProtocolVersion string          `json:"protocolVersion"`
		Capabilities    json.RawMessage `json:"capabilities"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("malformed initialize result: %w", err)
	}
	if envelope.ProtocolVersion == "" {
		return nil, errors.New("malformed initialize result: protocolVersion is missing")
	}
	if !slicesContains(supportedProtocolVersions, envelope.ProtocolVersion) {
		return nil, &UnsupportedProtocolError{Requested: protocolVersionLegacyInit, Returned: envelope.ProtocolVersion}
	}
	if rawIsNull(envelope.Capabilities) {
		return nil, errors.New("malformed initialize result: capabilities object is missing")
	}
	var declared map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Capabilities, &declared); err != nil || declared == nil {
		return nil, fmt.Errorf("malformed initialize result: capabilities must be an object: %s", diagnosticBody(strings.NewReader(string(envelope.Capabilities))))
	}
	caps := &MCPCapabilities{
		ProtocolVersion: envelope.ProtocolVersion,
		ServerName:      envelope.ServerInfo.Name,
		ServerVersion:   envelope.ServerInfo.Version,
	}
	for name, target := range map[string]*bool{
		"tools": &caps.Tools, "resources": &caps.Resources, "prompts": &caps.Prompts, "logging": &caps.Logging,
	} {
		value, present := declared[name]
		if !present {
			continue
		}
		var body map[string]json.RawMessage
		if err := json.Unmarshal(value, &body); err != nil || body == nil {
			return nil, fmt.Errorf("malformed initialize result: capability %q must be an object, got %s", name, diagnosticBody(strings.NewReader(string(value))))
		}
		*target = true
	}
	return caps, nil
}

func slicesContains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// initializedNotification is the notification that completes the handshake;
// requests follow it, not the initialize result.
func initializedNotification() mcpNotification {
	return mcpNotification{JSONRPC: "2.0", Method: "notifications/initialized"}
}

// --- tool call results ------------------------------------------------------

// callResultFromRPC projects a terminal tools/call response into the host's
// outcome. A JSON-RPC error and a tool-level isError are remote failures; a
// result that is not a tool result is a malformed response, not a success. The
// full original result stays in Output either way so shaping and retrieval keep
// the remote's own error content.
func callResultFromRPC(resp *mcpResponse, latencyMs int64) *MCPCallResult {
	if resp.Error != nil {
		return &MCPCallResult{Success: false, Error: resp.Error.Message, LatencyMs: latencyMs}
	}
	var result struct {
		Content *json.RawMessage `json:"content"`
		IsError *json.RawMessage `json:"isError"`
	}
	if rawIsNull(resp.Result) || json.Unmarshal(resp.Result, &result) != nil {
		return &MCPCallResult{Success: false, Output: resp.Result, LatencyMs: latencyMs,
			Error: "malformed tools/call result: the result is not a JSON object"}
	}
	isError := false
	if result.IsError != nil {
		if err := json.Unmarshal(*result.IsError, &isError); err != nil {
			return &MCPCallResult{Success: false, Output: resp.Result, LatencyMs: latencyMs,
				Error: "malformed tools/call result: isError must be a boolean"}
		}
	}
	if result.Content != nil && !rawIsNull(*result.Content) {
		var blocks []json.RawMessage
		if err := json.Unmarshal(*result.Content, &blocks); err != nil {
			return &MCPCallResult{Success: false, Output: resp.Result, LatencyMs: latencyMs,
				Error: "malformed tools/call result: content must be an array"}
		}
	}
	if isError {
		return &MCPCallResult{Success: false, ToolError: true, Output: resp.Result, LatencyMs: latencyMs,
			Error: toolErrorText(result.Content)}
	}
	return &MCPCallResult{Success: true, Output: resp.Result, LatencyMs: latencyMs}
}

// toolErrorText collects the text blocks of an isError result so the summary
// says what the remote tool said.
func toolErrorText(content *json.RawMessage) string {
	const fallback = "the remote tool reported isError without a message"
	if content == nil {
		return fallback
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(*content, &blocks) != nil {
		return fallback
	}
	var parts []string
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, "\n")
}

// --- catalog pagination -----------------------------------------------------

// listPaged consumes every page of a list method. It stops only when the server
// reports no further cursor. A cursor that repeats, a page or item bound, a
// cancellation or a failing later page ends collection with an
// IncompleteCatalogError and the items gathered so far; a failure on the first
// page, where nothing was gathered, is an ordinary error.
func listPaged[T any](ctx context.Context, caller rpcCaller, kind, method, key string) ([]T, error) {
	var items []T
	seen := make(map[string]struct{})
	cursor := ""
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return incompleteOrFail(items, kind, page-1, "collection canceled", err)
		}
		var params any
		if cursor != "" {
			params = map[string]any{"cursor": cursor}
		}
		resp, err := caller.call(ctx, method, params)
		if err != nil {
			return incompleteOrFail(items, kind, page-1, "a page request failed", fmt.Errorf("%s: %w", method, err))
		}
		batch, next, err := decodeCatalogPage[T](resp.Result, key)
		if err != nil {
			return incompleteOrFail(items, kind, page-1, "a page could not be parsed", fmt.Errorf("%s: %w", method, err))
		}
		items = append(items, batch...)
		if next == "" {
			return items, nil
		}
		if _, repeated := seen[next]; repeated {
			return items, &IncompleteCatalogError{Kind: kind, Collected: len(items), Pages: page, Reason: "the server repeated a page cursor"}
		}
		seen[next] = struct{}{}
		if page >= maxCatalogPages {
			return items, &IncompleteCatalogError{Kind: kind, Collected: len(items), Pages: page, Reason: fmt.Sprintf("the %d page bound was reached", maxCatalogPages)}
		}
		if len(items) >= maxCatalogItems {
			return items, &IncompleteCatalogError{Kind: kind, Collected: len(items), Pages: page, Reason: fmt.Sprintf("the %d item bound was reached", maxCatalogItems)}
		}
		cursor = next
	}
}

func incompleteOrFail[T any](items []T, kind string, pages int, reason string, cause error) ([]T, error) {
	if len(items) == 0 && pages == 0 {
		return nil, fmt.Errorf("failed to list %s: %w", kind, cause)
	}
	return items, &IncompleteCatalogError{Kind: kind, Collected: len(items), Pages: pages, Reason: reason, Cause: cause}
}

func decodeCatalogPage[T any](raw json.RawMessage, key string) ([]T, string, error) {
	if rawIsNull(raw) {
		return nil, "", errors.New("the result is empty")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, "", errors.New("the result is not a JSON object")
	}
	list, present := fields[key]
	if !present || rawIsNull(list) {
		return nil, "", fmt.Errorf("the result has no %q array", key)
	}
	var batch []T
	if err := json.Unmarshal(list, &batch); err != nil {
		return nil, "", fmt.Errorf("%q is not a valid array: %w", key, err)
	}
	var next string
	if cursor, ok := fields["nextCursor"]; ok && !rawIsNull(cursor) {
		if err := json.Unmarshal(cursor, &next); err != nil {
			return nil, "", fmt.Errorf("nextCursor must be a string: %w", err)
		}
	}
	return batch, next, nil
}

// --- Server-Sent Events -----------------------------------------------------

// scanSSE reads an event stream per the SSE field grammar (a field is
// "name:value" with one optional space after the colon, events end at a blank
// line, multi-line data joins with newlines). onEvent returns true to stop. An
// event or line larger than maxBytes is an error, not a truncation.
func scanSSE(r io.Reader, maxBytes int, onEvent func(event, data string) (stop bool)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxBytes)
	event := ""
	var data bytes.Buffer
	hasData := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if hasData {
				payload := strings.TrimSuffix(data.String(), "\n")
				name := event
				if name == "" {
					name = "message"
				}
				if onEvent(name, payload) {
					return nil
				}
			}
			event, hasData = "", false
			data.Reset()
			continue
		}
		if strings.HasPrefix(line, ":") {
			continue
		}
		field, value, _ := strings.Cut(line, ":")
		value = strings.TrimPrefix(value, " ")
		switch field {
		case "event":
			event = value
		case "data":
			if data.Len()+len(value)+1 > maxBytes {
				return fmt.Errorf("SSE event exceeds the %d byte wire limit", maxBytes)
			}
			data.WriteString(value)
			data.WriteByte('\n')
			hasData = true
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return fmt.Errorf("SSE line exceeds the %d byte wire limit", maxBytes)
		}
		return err
	}
	return nil
}
