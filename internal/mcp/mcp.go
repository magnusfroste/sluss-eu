// Package mcp is a small, dependency-free implementation of the Model Context
// Protocol (MCP) over JSON-RPC 2.0. It provides just enough of the protocol to
// expose read-only "tools" to an agent: initialize, tools/list and tools/call,
// served over either stdio (for local dev agents like Claude Code) or Streamable
// HTTP (for the deployed instance). See docs/mcp.md.
//
// It deliberately avoids a heavyweight SDK so the static CGO_ENABLED=0 build
// stays lean; the tools it serves wrap tokenizer's existing internal logic and
// never mutate state.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// ProtocolVersion is the MCP revision this server advertises. Clients that
// request a different version are echoed their own value (best-effort
// compatibility) as long as we can speak it.
const ProtocolVersion = "2024-11-05"

// Tool is one callable, read-only tool. Handler receives the raw JSON arguments
// object and returns any JSON-serialisable value, which is surfaced to the agent
// as a text content block of pretty-printed JSON.
type Tool struct {
	Name        string
	Description string
	// InputSchema is a JSON Schema object describing the arguments. Keep it a
	// plain map so callers can build it inline without extra types.
	InputSchema map[string]any
	Handler     func(ctx context.Context, args json.RawMessage) (any, error)
}

// Server holds the registered tools and dispatches JSON-RPC messages. It is safe
// for concurrent use after registration completes (tools are only read at
// dispatch time); register all tools before serving.
type Server struct {
	name    string
	version string
	tools   map[string]Tool
	order   []string
}

// NewServer returns an empty server identifying itself with name/version in the
// MCP initialize handshake.
func NewServer(name, version string) *Server {
	return &Server{name: name, version: version, tools: map[string]Tool{}}
}

// Register adds a tool. A later tool with the same name replaces an earlier one
// but keeps its list position.
func (s *Server) Register(t Tool) {
	if _, exists := s.tools[t.Name]; !exists {
		s.order = append(s.order, t.Name)
	}
	s.tools[t.Name] = t
}

// Tools returns the registered tools in registration order.
func (s *Server) Tools() []Tool {
	out := make([]Tool, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.tools[name])
	}
	return out
}

// JSON-RPC 2.0 wire types.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Standard JSON-RPC / MCP error codes.
const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

// MCP result payloads.

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolCallResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type toolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// HandleMessage dispatches a single decoded JSON-RPC message and returns the
// response to send back, or ok=false for notifications (no id) which get no
// response per JSON-RPC. It never returns an error: protocol problems are
// encoded as JSON-RPC error responses, tool failures as isError results.
func (s *Server) HandleMessage(ctx context.Context, raw []byte) (resp []byte, ok bool) {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return s.marshal(errorResponse(nil, codeParseError, "parse error: "+err.Error())), true
	}
	// Notifications (no id) are fire-and-forget — e.g. notifications/initialized.
	isNotification := len(req.ID) == 0
	r := s.route(ctx, req)
	if isNotification {
		return nil, false
	}
	return s.marshal(r), true
}

func (s *Server) route(ctx context.Context, req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized", "notifications/cancelled":
		return nil // ignored; notifications get no response
	case "ping":
		return successResponse(req.ID, map[string]any{})
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return errorResponse(req.ID, codeMethodNotFound, "method not found: "+req.Method)
	}
}

func (s *Server) handleInitialize(req rpcRequest) *rpcResponse {
	version := ProtocolVersion
	if len(req.Params) > 0 {
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(req.Params, &p); err == nil && p.ProtocolVersion != "" {
			version = p.ProtocolVersion // echo the client's requested revision
		}
	}
	return successResponse(req.ID, map[string]any{
		"protocolVersion": version,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    s.name,
			"version": s.version,
		},
	})
}

func (s *Server) handleToolsList(req rpcRequest) *rpcResponse {
	descriptors := make([]toolDescriptor, 0, len(s.order))
	for _, t := range s.Tools() {
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object"}
		}
		descriptors = append(descriptors, toolDescriptor{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		})
	}
	return successResponse(req.ID, map[string]any{"tools": descriptors})
}

func (s *Server) handleToolsCall(ctx context.Context, req rpcRequest) *rpcResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errorResponse(req.ID, codeInvalidParams, "invalid params: "+err.Error())
		}
	}
	tool, found := s.tools[p.Name]
	if !found {
		return errorResponse(req.ID, codeInvalidParams, "unknown tool: "+p.Name)
	}
	args := p.Arguments
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	out, err := tool.Handler(ctx, args)
	if err != nil {
		// MCP convention: tool errors are normal results flagged isError so the
		// agent can read the message, not JSON-RPC protocol errors.
		return successResponse(req.ID, toolCallResult{
			Content: []toolContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}
	text, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return errorResponse(req.ID, codeInternalError, "could not encode tool result: "+err.Error())
	}
	return successResponse(req.ID, toolCallResult{
		Content: []toolContent{{Type: "text", Text: string(text)}},
	})
}

func (s *Server) marshal(r *rpcResponse) []byte {
	if r == nil {
		return nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		// Should be unreachable; encode a minimal internal error.
		b, _ = json.Marshal(errorResponse(nil, codeInternalError, fmt.Sprintf("marshal error: %v", err)))
	}
	return b
}

func successResponse(id json.RawMessage, result any) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func errorResponse(id json.RawMessage, code int, msg string) *rpcResponse {
	return &rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}
