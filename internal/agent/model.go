// Package agent is the TUI/headless agent host: its own tool-calling loop
// driving a local model runtime over an OpenAI-compatible API (Ollama,
// llama.cpp). The runtime is reached via config.C.ModelBaseURL — swapping
// runtimes is a config change, not a code change.
package agent

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"

	"pentest-mcp/internal/config"
	"pentest-mcp/internal/registry"
)

// Model wraps the OpenAI-compatible client plus the tool schemas built from the
// shared registry.
type Model struct {
	client openai.Client
	name   string
	tools  []openai.ChatCompletionToolParam
}

// NewModel builds a client pointed at the configured runtime endpoint.
func NewModel() Model {
	client := openai.NewClient(
		option.WithBaseURL(config.C.ModelBaseURL),
		option.WithAPIKey(config.C.ModelAPIKey),
	)
	return Model{client: client, name: config.C.ModelName, tools: toolParams()}
}

// toolParams converts every registry tool into an OpenAI function-tool schema.
func toolParams() []openai.ChatCompletionToolParam {
	all := registry.All()
	out := make([]openai.ChatCompletionToolParam, 0, len(all))
	for _, t := range all {
		out = append(out, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name,
				Description: openai.String(t.Desc),
				Parameters:  shared.FunctionParameters(t.InputSchema()),
			},
		})
	}
	return out
}

// Complete runs one chat-completion turn and returns the assistant message
// (which may carry tool calls).
func (m Model) Complete(ctx context.Context, msgs []openai.ChatCompletionMessageParamUnion) (*openai.ChatCompletionMessage, error) {
	resp, err := m.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(m.name),
		Messages: msgs,
		Tools:    m.tools,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices")
	}
	return &resp.Choices[0].Message, nil
}
