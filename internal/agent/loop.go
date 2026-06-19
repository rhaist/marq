package agent

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go"

	"marq/internal/config"
	"marq/internal/registry"
	"marq/internal/skills"
)

// EventKind tags an agent-loop event for the UI/headless consumer.
type EventKind string

const (
	EventAssistant  EventKind = "assistant"   // model prose
	EventToolCall   EventKind = "tool_call"   // model invoked a tool
	EventToolResult EventKind = "tool_result" // tool output
	EventError      EventKind = "error"
	EventDone       EventKind = "done" // model finished (no more tool calls)
)

// Event is one step of the agent loop, streamed to the consumer.
type Event struct {
	Kind EventKind
	Text string // assistant prose / tool output / error
	Tool string // tool name (tool_call / tool_result)
	Args string // raw JSON arguments (tool_call)
}

// systemPrompt is the agent host's standing instruction: scope banner +
// methodology + operating rules.
func systemPrompt() string {
	return config.C.Banner() + "\n\n" + registry.Methodology + "\n\n" +
		"You are an autonomous penetration-testing assistant operating the tools above. " +
		"Call server_info first to confirm scope. Stay strictly within the authorized targets. " +
		"Work through the methodology, chaining tools as needed. When you start on a specific " +
		"technique or vulnerability class, call load_skill to pull its playbook first. Record each " +
		"validated issue with report_finding, then call render_report to produce the deliverable. " +
		"When the task is complete, call the finish tool with a concise summary.\n\n" +
		"Available skills (load_skill):\n" + skills.IndexText()
}

// Loop holds the conversation state for one agent session.
type Loop struct {
	model    Model
	byName   map[string]registry.Tool
	msgs     []openai.ChatCompletionMessageParamUnion
	MaxTurns int
}

// NewLoop builds an agent loop seeded with the system prompt.
func NewLoop() *Loop {
	return &Loop{
		model:    NewModel(),
		byName:   registry.ByName(),
		msgs:     []openai.ChatCompletionMessageParamUnion{openai.SystemMessage(systemPrompt())},
		MaxTurns: 50,
	}
}

// User appends an operator instruction to the conversation.
func (l *Loop) User(text string) {
	l.msgs = append(l.msgs, openai.UserMessage(text))
}

// Run drives turns until the model stops calling tools, an error occurs, the
// context is cancelled, or MaxTurns is hit. Each step is reported via emit.
func (l *Loop) Run(ctx context.Context, emit func(Event)) {
	for turn := 0; turn < l.MaxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			emit(Event{Kind: EventError, Text: err.Error()})
			return
		}
		msg, err := l.model.Complete(ctx, l.msgs)
		if err != nil {
			emit(Event{Kind: EventError, Text: err.Error()})
			return
		}
		l.msgs = append(l.msgs, msg.ToParam())
		if msg.Content != "" {
			emit(Event{Kind: EventAssistant, Text: msg.Content})
		}
		if len(msg.ToolCalls) == 0 {
			emit(Event{Kind: EventDone})
			return
		}
		finished := false
		for _, tc := range msg.ToolCalls {
			if tc.Function.Name == finishToolName {
				// Lifecycle tool: acknowledge, surface the summary, end the run.
				l.msgs = append(l.msgs, openai.ToolMessage("session finished", tc.ID))
				if s := finishSummary(tc.Function.Arguments); s != "" {
					emit(Event{Kind: EventAssistant, Text: s})
				}
				finished = true
				continue
			}
			emit(Event{Kind: EventToolCall, Tool: tc.Function.Name, Args: tc.Function.Arguments})
			out := l.dispatch(tc.Function.Name, tc.Function.Arguments)
			emit(Event{Kind: EventToolResult, Tool: tc.Function.Name, Text: out})
			l.msgs = append(l.msgs, openai.ToolMessage(out, tc.ID))
		}
		if finished {
			emit(Event{Kind: EventDone})
			return
		}
	}
	emit(Event{Kind: EventError, Text: "max turns reached"})
}

// finishSummary extracts the summary field from the finish tool's arguments.
func finishSummary(rawArgs string) string {
	var a struct {
		Summary string `json:"summary"`
	}
	_ = json.Unmarshal([]byte(rawArgs), &a)
	return a.Summary
}

// dispatch runs one tool call through the shared registry.
func (l *Loop) dispatch(name, rawArgs string) string {
	tool, ok := l.byName[name]
	if !ok {
		return "error: unknown tool " + name
	}
	var args map[string]any
	if rawArgs != "" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "error: invalid arguments: " + err.Error()
		}
	}
	return tool.Call(args)
}
