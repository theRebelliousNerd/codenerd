package core

import (
	"context"
	"fmt"
	"strings"
)

type mcpClientProxy struct {
	vs     *VirtualStore
	client IntegrationClient
}

func (p *mcpClientProxy) CallTool(ctx context.Context, tool string, args map[string]any) (res any, err error) {
	// Panic recovery (crashes prevention)
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("contract violation: MCP client panicked: %v", r)
		}
	}()

	// 1. Tool name validation
	if tool == "" {
		return nil, fmt.Errorf("invalid tool name: empty tool name is not permitted")
	}

	// 2. Argument sanitization and deep copy (AST leak prevention and race prevention)
	sanitizedArgs, err := p.sanitizeArgs(args)
	if err != nil {
		return nil, err
	}

	// 3. Delegation to the underlying client
	rawRes, err := p.client.CallTool(ctx, tool, sanitizedArgs)
	if err != nil {
		return nil, err
	}

	// 4. Result sanitization (malformed result handling and null byte escaping)
	sanitizedRes, err := p.sanitizeResult(rawRes)
	if err != nil {
		return nil, err
	}

	return sanitizedRes, nil
}

func (p *mcpClientProxy) sanitizeArgs(args map[string]any) (map[string]any, error) {
	if args == nil {
		return nil, nil
	}
	res := make(map[string]any)
	for k, v := range args {
		sV, err := p.sanitizeVal(v)
		if err != nil {
			return nil, err
		}
		res[k] = sV
	}
	return res, nil
}

func (p *mcpClientProxy) sanitizeVal(val any) (any, error) {
	if val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case string, int, int32, int64, float32, float64, bool:
		return v, nil
	case map[string]any:
		res := make(map[string]any)
		for k, mv := range v {
			sV, err := p.sanitizeVal(mv)
			if err != nil {
				return nil, err
			}
			res[k] = sV
		}
		return res, nil
	case []any:
		res := make([]any, len(v))
		for i, lv := range v {
			sV, err := p.sanitizeVal(lv)
			if err != nil {
				return nil, err
			}
			res[i] = sV
		}
		return res, nil
	default:
		// Catch any Mangle AST structures or non-primitives
		return nil, fmt.Errorf("contract violation: non-primitive type %T cannot be serialized to JSON", v)
	}
}

func (p *mcpClientProxy) sanitizeResult(res any) (any, error) {
	if res == nil {
		return "", nil
	}
	switch r := res.(type) {
	case string:
		// Escape null bytes for Mangle safety
		return strings.ReplaceAll(r, "\x00", ""), nil
	case []byte:
		return strings.ReplaceAll(string(r), "\x00", ""), nil
	case int, int32, int64, float32, float64, bool:
		return r, nil
	case map[string]any:
		resMap := make(map[string]any)
		for k, mv := range r {
			sV, err := p.sanitizeResult(mv)
			if err != nil {
				return nil, err
			}
			resMap[k] = sV
		}
		return resMap, nil
	case []any:
		resSlice := make([]any, len(r))
		for i, lv := range r {
			sV, err := p.sanitizeResult(lv)
			if err != nil {
				return nil, err
			}
			resSlice[i] = sV
		}
		return resSlice, nil
	default:
		// Bad type (like chan int) - serialize to string
		return fmt.Sprintf("%v", r), nil
	}
}
