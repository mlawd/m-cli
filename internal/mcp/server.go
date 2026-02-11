package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const protocolVersion = "2024-11-05"

type requestEnvelope struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type responseEnvelope struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type resourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

type resourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

type toolInfo struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type promptInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type promptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type promptMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func ServeStdio(ctx context.Context, in io.Reader, out io.Writer, version string) error {
	server := &stdioServer{
		ctx:     ctx,
		in:      bufio.NewReader(in),
		out:     out,
		version: strings.TrimSpace(version),
	}

	if server.version == "" {
		server.version = "dev"
	}

	return server.serve()
}

type stdioServer struct {
	ctx     context.Context
	in      *bufio.Reader
	out     io.Writer
	version string
}

func (s *stdioServer) serve() error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
		}

		msg, err := readRPCMessage(s.in)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var req requestEnvelope
		if err := json.Unmarshal(msg, &req); err != nil {
			if err := s.writeError(nil, -32700, "invalid JSON payload"); err != nil {
				return err
			}
			continue
		}

		if strings.TrimSpace(req.Method) == "" {
			continue
		}

		var id interface{}
		if len(req.ID) > 0 {
			if err := json.Unmarshal(req.ID, &id); err != nil {
				if err := s.writeError(nil, -32600, "invalid request id"); err != nil {
					return err
				}
				continue
			}
		}

		result, rpcErr := s.handleRequest(req.Method, req.Params)
		if id == nil {
			continue
		}

		if rpcErr != nil {
			if err := s.writeResponse(responseEnvelope{JSONRPC: "2.0", ID: id, Error: rpcErr}); err != nil {
				return err
			}
			continue
		}

		if err := s.writeResponse(responseEnvelope{JSONRPC: "2.0", ID: id, Result: result}); err != nil {
			return err
		}
	}
}

func (s *stdioServer) handleRequest(method string, rawParams json.RawMessage) (interface{}, *rpcError) {
	switch method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": protocolVersion,
			"capabilities": map[string]interface{}{
				"resources": map[string]interface{}{},
				"tools":     map[string]interface{}{},
				"prompts":   map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "m-cli-mcp",
				"version": s.version,
			},
			"instructions": "Use resources for m workflow guidance and tools to inspect current stack/stage context.",
		}, nil

	case "ping":
		return map[string]interface{}{}, nil

	case "resources/list":
		return map[string]interface{}{
			"resources": []resourceInfo{
				{
					URI:         "m://plan/format",
					Name:        "m Plan File Format",
					Description: "YAML schema and validation rules for `m stack new --plan-file`",
					MimeType:    "text/markdown",
				},
				{
					URI:         "m://guide/workflow",
					Name:        "m Workflow Guide",
					Description: "How to plan and execute work using m stack and stage commands",
					MimeType:    "text/markdown",
				},
				{
					URI:         "m://commands/reference",
					Name:        "m Command Reference",
					Description: "Quick command reference for m init/stack/stage workflows",
					MimeType:    "text/markdown",
				},
				{
					URI:         "m://state/context",
					Name:        "Current m Context",
					Description: "JSON snapshot of repo-local m state, including selected stack and stage",
					MimeType:    "application/json",
				},
			},
		}, nil

	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params for resources/read"}
		}

		return s.readResource(strings.TrimSpace(params.URI))

	case "tools/list":
		return map[string]interface{}{
			"tools": []toolInfo{
				{
					Name:        "get_m_context",
					Description: "Return current repository context for m, including selected stack and stage",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"include_stacks": map[string]interface{}{
								"type":        "boolean",
								"description": "When true, include full stack/stage objects in response",
							},
						},
					},
				},
				{
					Name:        "suggest_m_plan",
					Description: "Generate an m-aware execution plan for an agent goal",
					InputSchema: map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"goal": map[string]interface{}{
								"type":        "string",
								"description": "What the agent is trying to accomplish",
							},
						},
						"required": []string{"goal"},
					},
				},
			},
		}, nil

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params for tools/call"}
		}
		return s.callTool(strings.TrimSpace(params.Name), params.Arguments)

	case "prompts/list":
		return map[string]interface{}{
			"prompts": []promptInfo{
				{Name: "plan_with_m", Description: "Prompt template for stage-aware planning with m"},
			},
		}, nil

	case "prompts/get":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params for prompts/get"}
		}
		if strings.TrimSpace(params.Name) != "plan_with_m" {
			return nil, &rpcError{Code: -32602, Message: "unknown prompt"}
		}
		goal, _ := params.Arguments["goal"].(string)
		if strings.TrimSpace(goal) == "" {
			goal = "Complete the next stage in this repository"
		}

		promptText := fmt.Sprintf("Goal: %s\n\nUse the m context and workflow resources before planning. Propose a stage-aligned plan and identify the exact m commands needed to confirm or update stack/stage context.", strings.TrimSpace(goal))
		return map[string]interface{}{
			"description": "Stage-aware planning prompt for m",
			"arguments": []promptArgument{
				{Name: "goal", Description: "Goal to plan for", Required: false},
			},
			"messages": []promptMessage{
				{
					Role: "user",
					Content: map[string]interface{}{
						"type": "text",
						"text": promptText,
					},
				},
			},
		}, nil

	case "notifications/initialized", "initialized":
		return map[string]interface{}{}, nil

	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func (s *stdioServer) readResource(uri string) (interface{}, *rpcError) {
	switch uri {
	case "m://plan/format":
		return map[string]interface{}{
			"contents": []resourceContent{{
				URI:      uri,
				MimeType: "text/markdown",
				Text:     planFormatGuide(),
			}},
		}, nil

	case "m://guide/workflow":
		return map[string]interface{}{
			"contents": []resourceContent{{
				URI:      uri,
				MimeType: "text/markdown",
				Text:     planningGuide(),
			}},
		}, nil

	case "m://commands/reference":
		return map[string]interface{}{
			"contents": []resourceContent{{
				URI:      uri,
				MimeType: "text/markdown",
				Text:     commandReference(),
			}},
		}, nil

	case "m://state/context":
		snapshot, err := BuildContextSnapshot(".", true)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}
		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "failed to encode context"}
		}
		return map[string]interface{}{
			"contents": []resourceContent{{
				URI:      uri,
				MimeType: "application/json",
				Text:     string(data),
			}},
		}, nil

	default:
		return nil, &rpcError{Code: -32602, Message: "unknown resource uri"}
	}
}

func (s *stdioServer) callTool(name string, args map[string]interface{}) (interface{}, *rpcError) {
	switch name {
	case "get_m_context":
		includeStacks := true
		if raw, ok := args["include_stacks"]; ok {
			if b, ok := raw.(bool); ok {
				includeStacks = b
			}
		}

		snapshot, err := BuildContextSnapshot(".", includeStacks)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}

		data, err := json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: "failed to encode context"}
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": string(data)},
			},
			"structuredContent": snapshot,
		}, nil

	case "suggest_m_plan":
		goal, _ := args["goal"].(string)
		goal = strings.TrimSpace(goal)
		if goal == "" {
			return nil, &rpcError{Code: -32602, Message: "goal is required"}
		}

		snapshot, err := BuildContextSnapshot(".", false)
		if err != nil {
			return nil, &rpcError{Code: -32000, Message: err.Error()}
		}

		steps := []string{
			"Read `m://state/context` or call `get_m_context`.",
			"If no stack is selected, run `m stack list` then `m stack select <stack-name>`.",
			"If no stage is selected, run `m stage list` then `m stage select <stage-id>`.",
			"Execute implementation tasks for the selected stage only.",
			"Before handoff, re-check with `m stage current` and summarize stage-scoped changes.",
		}

		headline := fmt.Sprintf("Goal: %s", goal)
		if snapshot.CurrentStack != "" {
			headline += fmt.Sprintf(" | current stack: %s", snapshot.CurrentStack)
		}
		if snapshot.CurrentStage != "" {
			headline += fmt.Sprintf(" | current stage: %s", snapshot.CurrentStage)
		}

		var textBuilder strings.Builder
		textBuilder.WriteString(headline)
		textBuilder.WriteString("\n\nSuggested plan:\n")
		for i, step := range steps {
			textBuilder.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": strings.TrimSpace(textBuilder.String())},
			},
			"structuredContent": map[string]interface{}{
				"goal":  goal,
				"steps": steps,
				"context": map[string]interface{}{
					"current_stack": snapshot.CurrentStack,
					"current_stage": snapshot.CurrentStage,
				},
			},
		}, nil

	default:
		return nil, &rpcError{Code: -32602, Message: "unknown tool"}
	}
}

func readRPCMessage(r *bufio.Reader) ([]byte, error) {
	length := 0

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if err == io.EOF && length == 0 {
				return nil, io.EOF
			}
			return nil, err
		}

		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length header: %w", err)
			}
			length = parsed
		}
	}

	if length <= 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	return body, nil
}

func (s *stdioServer) writeResponse(resp responseEnvelope) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	_, _ = fmt.Fprintf(&b, "Content-Length: %d\r\n\r\n", len(data))
	_, _ = b.Write(data)

	_, err = s.out.Write(b.Bytes())
	return err
}

func (s *stdioServer) writeError(id interface{}, code int, message string) error {
	return s.writeResponse(responseEnvelope{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}
