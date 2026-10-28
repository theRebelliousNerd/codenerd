package mcp

import (
	"bytes"
	"codenerd/internal/approval"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync/atomic"
	"time"

	"codenerd/internal/logging"
)

type RemoteEffect string

type RemoteOperation string

const (
	RemoteTool     RemoteOperation = "tool"
	RemoteResource RemoteOperation = "resource"
	RemotePrompt   RemoteOperation = "prompt"
)

const (
	RemoteRead    RemoteEffect = "read"
	RemoteWrite   RemoteEffect = "write"
	RemoteDelete  RemoteEffect = "delete"
	RemoteExecute RemoteEffect = "execute"
)

type ReviewedToolEffect struct {
	Operation      RemoteOperation
	ToolID         string
	SchemaHash     string
	Effect         RemoteEffect
	TargetArgument string
	RiskOverride   RiskClass
}

type RemoteCallIdentity struct {
	Scope  string
	CallID string
}

type remoteCallState struct {
	identity RemoteCallIdentity
	used     atomic.Bool
}

type remoteIdentityKey struct{}
type remoteConfirmationKey struct{}

func WithRemoteCallIdentity(ctx context.Context, identity RemoteCallIdentity) context.Context {
	return context.WithValue(ctx, remoteIdentityKey{}, &remoteCallState{identity: identity})
}

func WithRiskConfirmation(ctx context.Context, confirmed bool) context.Context {
	return context.WithValue(ctx, remoteConfirmationKey{}, confirmed)
}

type RemoteCallEnvelope struct {
	Operation         RemoteOperation
	RequestID         string
	Scope             string
	CallID            string
	ServerID          string
	ToolID            string
	SchemaHash        string
	Effect            RemoteEffect
	Action            string
	Target            string
	CanonicalArgs     string
	ArgsDigest        string
	PermissionPayload string
	Confirmed         bool
}

func remoteAction(effect RemoteEffect) string {
	switch effect {
	case RemoteRead:
		return "/read_file"
	case RemoteWrite:
		return "/write_file"
	case RemoteDelete:
		return "/delete_file"
	case RemoteExecute:
		return "/run_arbitrary_command"
	default:
		return ""
	}
}

// auditRemoteAuthorization records the final policy verdict without including
// the target argument, canonical arguments, or permission payload. ToolID and
// the reviewed action are stable catalog metadata; reasons are static so a
// kernel or transport error cannot smuggle request data into the audit log.
func auditRemoteAuthorization(envelope RemoteCallEnvelope, allowed bool, reason string) {
	logging.Audit().SafetyCheck(
		fmt.Sprintf("MCP %s %s %s", envelope.Operation, envelope.ToolID, envelope.Action),
		allowed,
		reason,
	)
}

func exactMangleString(value string) string { return strconv.Quote(value) }

// humanApprovalPredicate is the host-asserted consent fact constitution.mg
// joins against the exact pending_action it names.
const humanApprovalPredicate = "human_approval_granted"

func nilKernel(kernel KernelInterface) bool {
	if kernel == nil {
		return true
	}
	value := reflect.ValueOf(kernel)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	}
	return false
}

func remoteSchemaHash(tool *MCPTool) string {
	return ToolSchemaHash(MCPToolSchema{
		Name: tool.Name, Description: tool.Description, InputSchema: tool.InputSchema,
		OutputSchema: tool.OutputSchema, Annotations: tool.Annotations,
	})
}

func knownRiskSource(source ClassificationSource) bool {
	switch source {
	case SourceAnnotation, SourceCapability, SourceName, SourceSchema, SourceDefault:
		return true
	}
	return false
}

func (manager *MCPClientManager) SetReviewedToolEffect(review ReviewedToolEffect) error {
	manager.authorityMu.Lock()
	defer manager.authorityMu.Unlock()
	delete(manager.reviewedEffects, review.ToolID)
	if review.Operation == "" {
		review.Operation = RemoteTool
	}
	if review.Operation != RemoteTool && review.Operation != RemoteResource && review.Operation != RemotePrompt {
		return fmt.Errorf("unknown reviewed MCP operation %q", review.Operation)
	}
	if review.Operation != RemoteTool && review.Effect != RemoteRead {
		return fmt.Errorf("MCP resource and prompt reviews require an explicit read effect")
	}
	if remoteAction(review.Effect) == "" || review.ToolID == "" || review.SchemaHash == "" || (review.RiskOverride != "" && !review.RiskOverride.Valid()) {
		return fmt.Errorf("invalid reviewed MCP effect for %q", review.ToolID)
	}
	if manager.reviewedEffects == nil {
		manager.reviewedEffects = make(map[string]ReviewedToolEffect)
	}
	manager.reviewedEffects[review.ToolID] = review
	return nil
}

func (manager *MCPClientManager) RevokeReviewedToolEffect(toolID string) {
	manager.authorityMu.Lock()
	defer manager.authorityMu.Unlock()
	delete(manager.reviewedEffects, toolID)
}

func canonicalRemoteArgs(args map[string]any) ([]byte, map[string]any, error) {
	if args == nil {
		args = map[string]any{}
	}
	canonical, err := json.Marshal(args)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid arguments: cannot serialize to JSON: %w", err)
	}
	var frozen map[string]any
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.UseNumber()
	if err := decoder.Decode(&frozen); err != nil {
		return nil, nil, fmt.Errorf("freeze MCP arguments: %w", err)
	}
	return canonical, frozen, nil
}

// remoteApprovalNeed is what one admission attempt hands back when the
// constitution marks the exact remote request human_approval_required: the
// frozen subject and preview, and the call identity the retry must reuse.
type remoteApprovalNeed struct {
	broker   *approval.Broker
	identity RemoteCallIdentity
	intent   string
	subject  approval.Subject
	preview  approval.Preview
}

// dispatchRemoteCall admits one remote call and dispatches it. When the
// kernel derives no permission but a person may approve the exact request, it
// waits for that decision with no lock, lease or fact held, then runs ONE more
// admission carrying the claimed approval. confirm_risk is still checked by
// that admission and never stands in for the decision.
func (manager *MCPClientManager) dispatchRemoteCall(ctx context.Context, operation RemoteOperation, serverID, toolID string, args map[string]any, canonical []byte, frozen map[string]any, invoke func(MCPTransport, map[string]any) (*MCPCallResult, error)) (*MCPCallResult, error) {
	result, need, err := manager.attemptRemoteCall(ctx, operation, serverID, toolID, args, canonical, frozen, invoke, nil, nil)
	if need == nil {
		return result, err
	}
	grant, req, err := need.broker.Await(ctx, need.intent, need.subject, need.preview)
	if err != nil {
		return nil, fmt.Errorf("MCP remote effect %s refused: approval request %s did not authorize it: %w", toolID, req.ID, err)
	}
	result, _, err = manager.attemptRemoteCall(ctx, operation, serverID, toolID, args, canonical, frozen, invoke, &need.identity, grant)
	return result, err
}

// attemptRemoteCall is one admission under authorityMu. prior is the call
// identity an earlier attempt of the same call consumed; grant is a person's
// approval of the exact request, which this attempt must claim against the
// request as it stands now before the kernel sees it.
func (manager *MCPClientManager) attemptRemoteCall(ctx context.Context, operation RemoteOperation, serverID, toolID string, args map[string]any, canonical []byte, frozen map[string]any, invoke func(MCPTransport, map[string]any) (*MCPCallResult, error), prior *RemoteCallIdentity, grant *approval.Grant) (result *MCPCallResult, need *remoteApprovalNeed, err error) {
	// An approval claimed by this attempt is spent whatever happens: refused
	// before dispatch is a known no-dispatch failure; an attempt that never
	// reached the claim abandons it so it cannot linger approved.
	reserved, dispatched := false, false
	if grant != nil {
		defer func() {
			if dispatched {
				return
			}
			reason := "the approved MCP call was refused before dispatch"
			if err != nil {
				reason = err.Error()
			}
			if reserved {
				grant.AdmissionFailed(reason)
			} else {
				grant.Abandon(reason)
			}
		}()
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	manager.authorityMu.Lock()
	defer manager.authorityMu.Unlock()
	manager.mu.RLock()
	_, toolName := parseToolID(toolID)
	connection := manager.servers[serverID]
	emitter := manager.facts
	tool, matches := manager.remoteSubjectLocked(operation, serverID, toolID)
	manager.mu.RUnlock()
	if matches != 1 {
		return nil, nil, fmt.Errorf("MCP subject %s has missing or conflicting registrations", toolID)
	}
	if emitter == nil || nilKernel(emitter.kernel) {
		return nil, nil, fmt.Errorf("MCP remote authority unavailable: kernel is nil")
	}
	if connection == nil || connection.Server == nil || connection.Server.ID != serverID || connection.Server.Status != ServerStatusConnected || connection.Transport == nil || !connection.Transport.IsConnected() {
		return nil, nil, fmt.Errorf("MCP server %s has no current connected registration", serverID)
	}
	if tool == nil || tool.ServerID != serverID || tool.Name != toolName || !tool.Risk.Valid() || !knownRiskSource(tool.RiskSource) {
		return nil, nil, fmt.Errorf("MCP tool %s has missing or conflicting metadata", toolID)
	}
	snapshot := *tool
	tool = &snapshot
	transport := connection.Transport
	server := *connection.Server
	var schema map[string]json.RawMessage
	if schemaErr := json.Unmarshal(tool.InputSchema, &schema); schemaErr != nil || schema == nil {
		return nil, nil, fmt.Errorf("MCP tool %s has no valid current input schema", toolID)
	}
	review, reviewed := manager.reviewedEffects[toolID]
	if !reviewed || review.Operation != operation || remoteAction(review.Effect) == "" || review.SchemaHash != tool.SchemaHash || review.SchemaHash != remoteSchemaHash(tool) {
		return nil, nil, fmt.Errorf("MCP tool %s has no current host-reviewed effect", toolID)
	}
	if err := ValidateArgs(tool, frozen); err != nil {
		return nil, nil, err
	}
	requestID := "exec-mcp-" + rand.Text()
	var identity RemoteCallIdentity
	if prior != nil {
		// The same call's retry after a decision: its identity was consumed
		// by the first attempt and is not consumed again.
		identity = *prior
	} else {
		identity = RemoteCallIdentity{Scope: requestID, CallID: requestID}
		if state, supplied := ctx.Value(remoteIdentityKey{}).(*remoteCallState); supplied {
			if state.identity.Scope == "" || state.identity.CallID == "" || !state.used.CompareAndSwap(false, true) {
				return nil, nil, fmt.Errorf("MCP call identity is missing or already consumed")
			}
			identity = state.identity
		}
	}
	target := toolID
	if review.TargetArgument != "" {
		value, valid := frozen[review.TargetArgument].(string)
		if !valid || value == "" {
			return nil, nil, fmt.Errorf("MCP reviewed target argument %q must be a nonempty string", review.TargetArgument)
		}
		target = value
	}
	permissionPayload, marshalErr := json.Marshal(struct {
		RequestID string          `json:"request_id"`
		Scope     string          `json:"scope"`
		CallID    string          `json:"call_id"`
		ServerID  string          `json:"server_id"`
		ToolID    string          `json:"tool_id"`
		Args      json.RawMessage `json:"args"`
	}{requestID, identity.Scope, identity.CallID, serverID, toolID, canonical})
	if marshalErr != nil {
		return nil, nil, marshalErr
	}
	digest := sha256.Sum256(canonical)
	confirmed, _ := ctx.Value(remoteConfirmationKey{}).(bool)
	envelope := RemoteCallEnvelope{
		Operation: operation,
		RequestID: requestID, Scope: identity.Scope, CallID: identity.CallID,
		ServerID: serverID, ToolID: toolID, SchemaHash: review.SchemaHash,
		Effect: review.Effect, Action: remoteAction(review.Effect), Target: target,
		CanonicalArgs: string(canonical), ArgsDigest: hex.EncodeToString(digest[:]),
		PermissionPayload: string(permissionPayload), Confirmed: confirmed,
	}
	effectiveRisk := tool.Risk
	if review.RiskOverride != "" {
		effectiveRisk = review.RiskOverride
	}
	confirmationAtom := "/false"
	if confirmed {
		confirmationAtom = "/true"
	}
	endpoint := server.Endpoint
	if endpoint == "" {
		endpoint = serverID
	}
	protocol := string(server.Protocol)
	if protocol == "" {
		protocol = "unknown"
	}
	// The reviewed subject: everything that identifies the remote effect,
	// with the canonical arguments rather than the permission payload (which
	// carries this attempt's random request ID).
	subject := approval.Subject{
		Session: identity.Scope, Agent: "mcp", Backend: "mcp",
		Tool: toolID, ToolVersion: review.SchemaHash,
		Action: envelope.Action, Target: target, Payload: string(canonical),
		Effect: string(review.Effect), Endpoint: endpoint + "|" + protocol,
	}
	intent := identity.Scope + "/" + identity.CallID
	facts := []string{
		fmt.Sprintf("mcp_remote_operation(%s, /%s)", exactMangleString(requestID), operation),
		fmt.Sprintf("mcp_remote_reviewed(%s, %s, %s, %s, /%s)", exactMangleString(requestID), exactMangleString(serverID), exactMangleString(toolID), exactMangleString(review.SchemaHash), review.Effect),
		fmt.Sprintf("mcp_remote_request(%s, %s, %s, %s, %s, %s, /%s, %s, %s, %s, %s, %s, %s)",
			exactMangleString(requestID), exactMangleString(identity.Scope), exactMangleString(identity.CallID),
			exactMangleString(serverID), exactMangleString(toolID), exactMangleString(review.SchemaHash),
			review.Effect, envelope.Action, exactMangleString(target), exactMangleString(envelope.PermissionPayload),
			exactMangleString(envelope.ArgsDigest), effectiveRisk.Atom(), confirmationAtom),
		fmt.Sprintf("pending_action(%s, %s, %s, %s, %d)", exactMangleString(requestID), envelope.Action, exactMangleString(target), exactMangleString(envelope.PermissionPayload), time.Now().Unix()),
	}
	if grant != nil {
		// Claim the approval against the request as it is now: a changed
		// registration, schema, endpoint, target or argument is stale.
		if reserveErr := grant.Reserve(intent, subject); reserveErr != nil {
			auditRemoteAuthorization(envelope, false, "approved MCP request could not be claimed (stale, expired or already used); remote action denied")
			return nil, nil, fmt.Errorf("MCP remote effect %s refused: the approval cannot be used: %w", toolID, reserveErr)
		}
		reserved = true
		facts = append(facts, fmt.Sprintf("%s(%s, %s, %s, %s, %s)", humanApprovalPredicate,
			exactMangleString(requestID), envelope.Action, exactMangleString(target),
			exactMangleString(envelope.PermissionPayload), exactMangleString(grant.Digest())))
	}
	emitter.mu.Lock()
	defer emitter.mu.Unlock()
	if emitter.failure != nil {
		return nil, nil, fmt.Errorf("MCP fact publication is unhealthy: %w", emitter.failure)
	}
	serverRows, serverErr := emitter.kernel.Query(fmt.Sprintf("mcp_server_registered(%s, %s, %s, RegisteredAt)", exactMangleString(serverID), exactMangleString(endpoint), mangleAtom(protocol)))
	if serverErr != nil {
		return nil, nil, fmt.Errorf("query current MCP server registration: %w", serverErr)
	}
	if len(serverRows) != 1 {
		return nil, nil, fmt.Errorf("MCP server registration does not exactly match the current connection")
	}
	for _, registration := range []string{
		fmt.Sprintf("mcp_server_registered(%s, Endpoint, Protocol, RegisteredAt)", exactMangleString(serverID)),
		fmt.Sprintf("mcp_server_status(%s, Status)", exactMangleString(serverID)),
		fmt.Sprintf("mcp_server_status(%s, /connected)", exactMangleString(serverID)),
	} {
		rows, queryErr := emitter.kernel.Query(registration)
		if queryErr != nil {
			return nil, nil, fmt.Errorf("query MCP connection authority: %w", queryErr)
		}
		if len(rows) != 1 {
			return nil, nil, fmt.Errorf("MCP connection has no unique current positive registration")
		}
	}
	if operation == RemoteTool {
		for _, metadata := range []string{
			fmt.Sprintf("mcp_tool_registered(%s, Server, RegisteredAt)", exactMangleString(toolID)),
			fmt.Sprintf("mcp_tool_registered(%s, %s, RegisteredAt)", exactMangleString(toolID), exactMangleString(serverID)),
			fmt.Sprintf("mcp_tool_name(%s, Name)", exactMangleString(toolID)),
			fmt.Sprintf("mcp_tool_name(%s, %s)", exactMangleString(toolID), exactMangleString(tool.Name)),
			fmt.Sprintf("mcp_tool_risk(%s, Risk)", exactMangleString(toolID)),
			fmt.Sprintf("mcp_tool_risk(%s, %s)", exactMangleString(toolID), tool.Risk.Atom()),
			fmt.Sprintf("mcp_tool_risk_source(%s, Source)", exactMangleString(toolID)),
			fmt.Sprintf("mcp_tool_risk_source(%s, %s)", exactMangleString(toolID), mangleAtom(string(tool.RiskSource))),
			fmt.Sprintf("mcp_tool_schema_hash(%s, Schema)", exactMangleString(toolID)),
			fmt.Sprintf("mcp_tool_schema_hash(%s, %s)", exactMangleString(toolID), exactMangleString(review.SchemaHash)),
		} {
			rows, queryErr := emitter.kernel.Query(metadata)
			if queryErr != nil {
				return nil, nil, fmt.Errorf("query current MCP metadata: %w", queryErr)
			}
			if len(rows) != 1 {
				return nil, nil, fmt.Errorf("MCP metadata does not exactly match the current registration")
			}
		}
	} else {
		predicate := "mcp_remote_resource"
		if operation == RemotePrompt {
			predicate = "mcp_remote_prompt"
		}
		rows, queryErr := emitter.kernel.Query(fmt.Sprintf("%s(%s, %s, Name, %s)", predicate, exactMangleString(serverID), exactMangleString(toolID), exactMangleString(review.SchemaHash)))
		if queryErr != nil {
			return nil, nil, fmt.Errorf("query MCP resource or prompt registration: %w", queryErr)
		}
		if len(rows) != 1 {
			return nil, nil, fmt.Errorf("MCP resource or prompt has no exact positive current registration")
		}
	}
	var attempted []string
	defer func() {
		var cleanupErr error
		for index := len(attempted) - 1; index >= 0; index-- {
			if retractErr := emitter.kernel.Retract(attempted[index]); retractErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("retract MCP authority: %w", retractErr))
			}
		}
		if cleanupErr != nil {
			emitter.failure = errors.Join(emitter.failure, cleanupErr)
			err = errors.Join(err, cleanupErr)
		}
	}()
	for _, fact := range facts {
		attempted = append(attempted, fact)
		if assertErr := emitter.kernel.Assert(fact); assertErr != nil {
			emitter.failure = errors.Join(emitter.failure, assertErr)
			return nil, nil, fmt.Errorf("assert MCP authority: %w", assertErr)
		}
	}
	confirmationRequired, confirmationErr := emitter.kernel.Query(fmt.Sprintf("mcp_remote_confirmation_required(%s)", exactMangleString(requestID)))
	if confirmationErr != nil {
		auditRemoteAuthorization(envelope, false, "kernel query mcp_remote_confirmation_required/1 failed; remote action denied")
		return nil, nil, fmt.Errorf("query MCP confirmation requirement: %w", confirmationErr)
	}
	if len(confirmationRequired) > 0 && !confirmed {
		auditRemoteAuthorization(envelope, false, "kernel requires separate risk confirmation; remote action denied")
		return nil, nil, fmt.Errorf("MCP remote effect %s requires separate risk confirmation", toolID)
	}
	query := fmt.Sprintf("mcp_remote_permitted(%s, %s, %s, %s, %s, %s, %s)",
		exactMangleString(requestID), exactMangleString(identity.Scope), exactMangleString(identity.CallID),
		exactMangleString(serverID), exactMangleString(toolID), exactMangleString(review.SchemaHash), exactMangleString(envelope.ArgsDigest))
	permission, queryErr := emitter.kernel.Query(query)
	if queryErr != nil {
		auditRemoteAuthorization(envelope, false, "kernel query mcp_remote_permitted/7 failed; remote action denied")
		return nil, nil, fmt.Errorf("query MCP remote authority: %w", queryErr)
	}
	if len(permission) != 1 {
		if grant == nil {
			// Asked while this request's facts are still asserted: only the
			// constitution decides whether a person may approve it.
			required, requiredErr := emitter.kernel.Query(fmt.Sprintf("human_approval_required(%s)", exactMangleString(requestID)))
			if requiredErr == nil && len(required) == 1 {
				broker := manager.approvals
				if !broker.HostAttached() {
					auditRemoteAuthorization(envelope, false, "kernel derived no unique mcp_remote_permitted/7 fact; human-approvable but no approval host is attached; remote action denied")
					return nil, nil, fmt.Errorf("MCP remote effect %s refused: a person may approve this exact request, but no approval host is attached — do not retry the same call; ask the user to perform it or to run interactively", toolID)
				}
				preview, previewErr := approval.BuildPreview(approval.PreviewInput{
					Summary:   fmt.Sprintf("Run remote MCP tool %s (%s)", toolID, review.Effect),
					Effects:   []string{fmt.Sprintf("%s on %s through %s", envelope.Action, target, endpoint)},
					Unknowns:  []string{"what the remote server does with these arguments is not observable by codeNERD"},
					Arguments: string(canonical),
					MaxBytes:  broker.PreviewMaxBytes(),
				})
				if previewErr != nil {
					auditRemoteAuthorization(envelope, false, "approval preview could not be built; remote action denied")
					return nil, nil, fmt.Errorf("MCP remote effect %s refused: approval preview: %w", toolID, previewErr)
				}
				auditRemoteAuthorization(envelope, false, "kernel derived no unique mcp_remote_permitted/7 fact; human approval of the exact request requested")
				return nil, &remoteApprovalNeed{broker: broker, identity: identity, intent: intent, subject: subject, preview: preview}, nil
			}
		}
		auditRemoteAuthorization(envelope, false, "kernel derived no unique mcp_remote_permitted/7 fact; remote action denied")
		return nil, nil, fmt.Errorf("MCP remote effect %s refused: no unique positive per-call permission", toolID)
	}
	// The authorized canonical tree is never rebuilt from caller-owned data.
	// Serialize only for a mutation check, then dispatch the original frozen tree.
	currentArgs := args
	if currentArgs == nil {
		currentArgs = map[string]any{}
	}
	current, snapshotErr := json.Marshal(currentArgs)
	if snapshotErr != nil || !bytes.Equal(current, canonical) {
		auditRemoteAuthorization(envelope, false, "arguments changed after Mangle authorization; remote action denied")
		return nil, nil, fmt.Errorf("MCP arguments changed after authorization")
	}
	if err := ctx.Err(); err != nil {
		auditRemoteAuthorization(envelope, false, "call context canceled after Mangle authorization; remote action denied")
		return nil, nil, err
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.servers[serverID] != connection || connection.Transport != transport || connection.Server == nil || connection.Server.ID != serverID || connection.Server.Endpoint != server.Endpoint || connection.Server.Protocol != server.Protocol || connection.Server.Status != ServerStatusConnected || !transport.IsConnected() || remoteSchemaHash(tool) != review.SchemaHash {
		auditRemoteAuthorization(envelope, false, "MCP connection changed after Mangle authorization; remote action denied")
		return nil, nil, fmt.Errorf("MCP registration changed after authorization")
	}
	currentTool, currentMatches := manager.remoteSubjectLocked(operation, serverID, toolID)
	if currentMatches != 1 || currentTool == nil || currentTool.ServerID != serverID || currentTool.Name != toolName || currentTool.Risk != tool.Risk || currentTool.RiskSource != tool.RiskSource || currentTool.SchemaHash != review.SchemaHash || remoteSchemaHash(currentTool) != review.SchemaHash {
		auditRemoteAuthorization(envelope, false, "MCP catalog changed after Mangle authorization; remote action denied")
		return nil, nil, fmt.Errorf("MCP tool catalog changed after authorization")
	}
	if grant == nil {
		auditRemoteAuthorization(envelope, true, "kernel derived exact mcp_remote_permitted/7 authorization; its rule matched the exact permitted/3 action, target, and payload tuple; final host checks passed")
		result, err = invoke(transport, frozen)
		return result, nil, err
	}
	// Durable before the effect: a crash from here on is an unknown outcome
	// on restart, never a claim to run again.
	if startErr := grant.DispatchStarted(); startErr != nil {
		auditRemoteAuthorization(envelope, false, "approved MCP dispatch could not be recorded durably; remote action denied")
		return nil, nil, fmt.Errorf("MCP remote effect %s not dispatched: its dispatch record could not be made durable: %w", toolID, startErr)
	}
	dispatched = true
	auditRemoteAuthorization(envelope, true, "kernel derived exact mcp_remote_permitted/7 authorization through a claimed request-bound human approval; its rule matched the exact permitted/3 action, target, and payload tuple; final host checks passed")
	result, err = invoke(transport, frozen)
	outcome := err
	if outcome == nil && (result == nil || !result.Success) {
		outcome = errors.New("the remote tool reported failure")
	}
	grant.Finish(outcome)
	return result, nil, err
}

// invalidateRemoteCatalog removes executable registrations after a failed refresh.
// Cached store rows remain available for discovery and analysis reuse.
func (manager *MCPClientManager) invalidateRemoteCatalog(operation RemoteOperation, serverID string) error {
	manager.mu.Lock()
	var previous []*MCPTool
	switch operation {
	case RemoteTool:
		if connection := manager.servers[serverID]; connection != nil {
			previous = connection.Tools
			connection.Tools = nil
		}
	case RemoteResource:
		delete(manager.resources, serverID)
	case RemotePrompt:
		delete(manager.prompts, serverID)
	}
	emitter := manager.facts
	manager.mu.Unlock()
	if emitter == nil {
		return nil
	}
	var invalidateErr error
	switch operation {
	case RemoteTool:
		for _, tool := range previous {
			if tool != nil {
				emitter.RetractTool(tool.ToolID)
			}
		}
	case RemoteResource:
		invalidateErr = emitter.EmitResources(serverID, nil)
	case RemotePrompt:
		invalidateErr = emitter.EmitPrompts(serverID, nil)
	}
	return invalidateErr
}

// Revoke changed or removed subjects as soon as the live catalog arrives,
// before analysis or publication can fail. Unchanged subjects stay callable.
func (manager *MCPClientManager) beginRemoteToolRefresh(serverID string, connection *MCPServerConnection, schemas []MCPToolSchema) error {
	hashes := make(map[string]string, len(schemas))
	counts := make(map[string]int, len(schemas))
	for _, schema := range schemas {
		toolID := serverID + "/" + schema.Name
		hashes[toolID] = ToolSchemaHash(schema)
		counts[toolID]++
	}
	manager.mu.Lock()
	if manager.servers[serverID] != connection {
		manager.mu.Unlock()
		return fmt.Errorf("MCP connection changed during tool discovery")
	}
	retained := make([]*MCPTool, 0, len(connection.Tools))
	var stale []*MCPTool
	for _, tool := range connection.Tools {
		if tool != nil && tool.ServerID == serverID && counts[tool.ToolID] == 1 && hashes[tool.ToolID] == tool.SchemaHash && hashes[tool.ToolID] == remoteSchemaHash(tool) {
			retained = append(retained, tool)
		} else if tool != nil {
			stale = append(stale, tool)
		}
	}
	connection.Tools = retained
	emitter := manager.facts
	manager.mu.Unlock()
	for _, tool := range stale {
		emitter.RetractTool(tool.ToolID)
	}
	if err := emitter.Error(); err != nil {
		return fmt.Errorf("revoke stale MCP catalog: %w", err)
	}
	return nil
}

func ResourceAuthorityTool(serverID string, resource MCPResource) *MCPTool {
	encoded, _ := json.Marshal(resource)
	digest := sha256.Sum256([]byte(resource.URI))
	name := "@resource-" + hex.EncodeToString(digest[:])
	tool := &MCPTool{ToolID: serverID + "/" + name, ServerID: serverID, Name: name,
		Description: string(encoded), InputSchema: json.RawMessage(`{"type":"object","required":["uri"],"properties":{"uri":{"type":"string"}}}`), Risk: RiskSafe, RiskSource: SourceAnnotation}
	tool.SchemaHash = remoteSchemaHash(tool)
	return tool
}

func PromptAuthorityTool(serverID string, prompt MCPPrompt) *MCPTool {
	encoded, _ := json.Marshal(prompt)
	digest := sha256.Sum256([]byte(prompt.Name))
	name := "@prompt-" + hex.EncodeToString(digest[:])
	tool := &MCPTool{ToolID: serverID + "/" + name, ServerID: serverID, Name: name,
		Description: string(encoded), InputSchema: json.RawMessage(`{"type":"object","required":["name","arguments"],"properties":{"name":{"type":"string"},"arguments":{"type":"object"}}}`), Risk: RiskSafe, RiskSource: SourceAnnotation}
	tool.SchemaHash = remoteSchemaHash(tool)
	return tool
}

func (manager *MCPClientManager) remoteSubjectLocked(operation RemoteOperation, serverID, toolID string) (*MCPTool, int) {
	var subject *MCPTool
	matches := 0
	consider := func(candidate *MCPTool) {
		if candidate != nil && candidate.ToolID == toolID {
			subject = candidate
			matches++
		}
	}
	switch operation {
	case RemoteTool:
		if connection := manager.servers[serverID]; connection != nil {
			for _, candidate := range connection.Tools {
				consider(candidate)
			}
		}
	case RemoteResource:
		for _, resource := range manager.resources[serverID] {
			consider(ResourceAuthorityTool(serverID, resource))
		}
	case RemotePrompt:
		for _, prompt := range manager.prompts[serverID] {
			consider(PromptAuthorityTool(serverID, prompt))
		}
	}
	return subject, matches
}
