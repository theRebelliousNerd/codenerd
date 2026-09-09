package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// MCP servers expose three primitive kinds: tools, resources, and prompts.
// Until now codeNERD only consumed tools and recorded the other two as boolean
// capability flags, so a server advertising `resources` gave the agent nothing
// it could actually read.
//
// Resources and prompts are added as optional transport interfaces rather than
// new MCPTransport methods: MCPTransport is implemented by callers' fakes as
// well as the three built-in transports, and widening it would break every one
// of them for a capability many servers do not have.

// MCPResource is a resource advertised by an MCP server.
type MCPResource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitzero"`
	Description string `json:"description,omitzero"`
	MimeType    string `json:"mimeType,omitzero"`
}

// MCPResourceContent is one content block returned by resources/read.
type MCPResourceContent struct {
	URI      string `json:"uri,omitzero"`
	MimeType string `json:"mimeType,omitzero"`
	Text     string `json:"text,omitzero"`
	Blob     string `json:"blob,omitzero"` // base64, per the MCP wire format
}

// MCPPromptArgument describes one templated argument of a server prompt.
type MCPPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitzero"`
	Required    bool   `json:"required,omitzero"`
}

// MCPPrompt is a prompt template advertised by an MCP server.
type MCPPrompt struct {
	Name        string              `json:"name"`
	Description string              `json:"description,omitzero"`
	Arguments   []MCPPromptArgument `json:"arguments,omitzero"`
}

// MCPPromptMessage is one message of a rendered prompt.
type MCPPromptMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ResourceCapableTransport is implemented by transports that can serve the
// resources/* methods.
type ResourceCapableTransport interface {
	ListResources(ctx context.Context) ([]MCPResource, error)
	ReadResource(ctx context.Context, uri string) ([]MCPResourceContent, error)
}

// PromptCapableTransport is implemented by transports that can serve the
// prompts/* methods.
type PromptCapableTransport interface {
	ListPrompts(ctx context.Context) ([]MCPPrompt, error)
	GetPrompt(ctx context.Context, name string, args map[string]string) ([]MCPPromptMessage, error)
}

// rpcCaller is the shared shape of the three built-in transports' private
// JSON-RPC helper.
type rpcCaller interface {
	call(ctx context.Context, method string, params any) (*mcpResponse, error)
}

func listResourcesVia(ctx context.Context, caller rpcCaller) ([]MCPResource, error) {
	resp, err := caller.call(ctx, "resources/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}
	var result struct {
		Resources []MCPResource `json:"resources"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}
	return result.Resources, nil
}

func readResourceVia(ctx context.Context, caller rpcCaller, uri string) ([]MCPResourceContent, error) {
	if uri == "" {
		return nil, fmt.Errorf("resource URI cannot be empty")
	}
	resp, err := caller.call(ctx, "resources/read", map[string]any{"uri": uri})
	if err != nil {
		return nil, fmt.Errorf("failed to read resource %s: %w", uri, err)
	}
	var result struct {
		Contents []MCPResourceContent `json:"contents"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse resource contents: %w", err)
	}
	return result.Contents, nil
}

func listPromptsVia(ctx context.Context, caller rpcCaller) ([]MCPPrompt, error) {
	resp, err := caller.call(ctx, "prompts/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}
	var result struct {
		Prompts []MCPPrompt `json:"prompts"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompts response: %w", err)
	}
	return result.Prompts, nil
}

func getPromptVia(ctx context.Context, caller rpcCaller, name string, args map[string]string) ([]MCPPromptMessage, error) {
	if name == "" {
		return nil, fmt.Errorf("prompt name cannot be empty")
	}
	params := map[string]any{"name": name}
	if len(args) > 0 {
		params["arguments"] = args
	}
	resp, err := caller.call(ctx, "prompts/get", params)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt %s: %w", name, err)
	}

	// The wire format allows content to be either a bare string or a typed
	// content block; normalize both to text so callers get one shape.
	var result struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse prompt response: %w", err)
	}

	messages := make([]MCPPromptMessage, 0, len(result.Messages))
	for _, m := range result.Messages {
		messages = append(messages, MCPPromptMessage{Role: m.Role, Content: promptContentText(m.Content)})
	}
	return messages, nil
}

func promptContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &block); err == nil && block.Text != "" {
		return block.Text
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var sb strings.Builder
		var length int
		for _, b := range blocks {
			length += len(b.Text)
		}
		sb.Grow(length)
		for _, b := range blocks {
			sb.WriteString(b.Text)
		}
		return sb.String()
	}
	return string(raw)
}

// --- HTTP ---

func (t *HTTPTransport) ListResources(ctx context.Context) ([]MCPResource, error) {
	return listResourcesVia(ctx, t)
}

func (t *HTTPTransport) ReadResource(ctx context.Context, uri string) ([]MCPResourceContent, error) {
	return readResourceVia(ctx, t, uri)
}

func (t *HTTPTransport) ListPrompts(ctx context.Context) ([]MCPPrompt, error) {
	return listPromptsVia(ctx, t)
}

func (t *HTTPTransport) GetPrompt(ctx context.Context, name string, args map[string]string) ([]MCPPromptMessage, error) {
	return getPromptVia(ctx, t, name, args)
}

// --- SSE ---

func (t *SSETransport) ListResources(ctx context.Context) ([]MCPResource, error) {
	return listResourcesVia(ctx, t)
}

func (t *SSETransport) ReadResource(ctx context.Context, uri string) ([]MCPResourceContent, error) {
	return readResourceVia(ctx, t, uri)
}

func (t *SSETransport) ListPrompts(ctx context.Context) ([]MCPPrompt, error) {
	return listPromptsVia(ctx, t)
}

func (t *SSETransport) GetPrompt(ctx context.Context, name string, args map[string]string) ([]MCPPromptMessage, error) {
	return getPromptVia(ctx, t, name, args)
}

// --- stdio ---

func (t *StdioTransport) ListResources(ctx context.Context) ([]MCPResource, error) {
	return listResourcesVia(ctx, t)
}

func (t *StdioTransport) ReadResource(ctx context.Context, uri string) ([]MCPResourceContent, error) {
	return readResourceVia(ctx, t, uri)
}

func (t *StdioTransport) ListPrompts(ctx context.Context) ([]MCPPrompt, error) {
	return listPromptsVia(ctx, t)
}

func (t *StdioTransport) GetPrompt(ctx context.Context, name string, args map[string]string) ([]MCPPromptMessage, error) {
	return getPromptVia(ctx, t, name, args)
}

var (
	_ ResourceCapableTransport = (*HTTPTransport)(nil)
	_ ResourceCapableTransport = (*SSETransport)(nil)
	_ ResourceCapableTransport = (*StdioTransport)(nil)
	_ PromptCapableTransport   = (*HTTPTransport)(nil)
	_ PromptCapableTransport   = (*SSETransport)(nil)
	_ PromptCapableTransport   = (*StdioTransport)(nil)
)
