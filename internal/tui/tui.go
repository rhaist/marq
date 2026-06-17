// Package tui is the Bubble Tea agent-host front-end (phase 4). It drives the
// shared agent loop against the local model runtime and renders the live
// transcript: operator prompts, model prose, tool calls and tool output. Agent
// events stream in over a channel consumed by a re-subscribing tea.Cmd.
package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pentest-mcp/internal/agent"
	"pentest-mcp/internal/config"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Padding(0, 1)
	helpStyle   = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	userStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	botStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	resultStyle = lipgloss.NewStyle().Faint(true)
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
)

type eventMsg agent.Event

// Run starts the TUI agent host.
func Run() error {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type model struct {
	loop    *agent.Loop
	input   textinput.Model
	vp      viewport.Model
	events  chan agent.Event
	lines   []string
	running bool
	ready   bool
	w, h    int
}

func newModel() model {
	ti := textinput.New()
	ti.Placeholder = "Describe the task (e.g. footprint example.com) — Enter to run, Ctrl+C to quit"
	ti.Focus()
	ti.CharLimit = 4000
	return model{
		loop:   agent.NewLoop(),
		input:  ti,
		events: make(chan agent.Event, 256),
	}
}

func (m model) Init() tea.Cmd { return textinput.Blink }

// waitForEvent reads the next agent event from the channel as a tea.Msg.
func waitForEvent(ch chan agent.Event) tea.Cmd {
	return func() tea.Msg { return eventMsg(<-ch) }
}

func (m *model) start(task string) tea.Cmd {
	m.append(userStyle.Render("❯ ") + task)
	m.loop.User(task)
	m.running = true
	go m.loop.Run(context.Background(), func(e agent.Event) { m.events <- e })
	return waitForEvent(m.events)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		bodyH := msg.Height - 4 // header + input + help + spacing
		if bodyH < 3 {
			bodyH = 3
		}
		if !m.ready {
			m.vp = viewport.New(msg.Width, bodyH)
			m.vp.SetContent(m.welcome())
			m.ready = true
		} else {
			m.vp.Width, m.vp.Height = msg.Width, bodyH
		}
		m.input.Width = msg.Width - 4

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if !m.running {
				task := strings.TrimSpace(m.input.Value())
				if task != "" {
					m.input.SetValue("")
					cmds = append(cmds, m.start(task))
				}
			}
		}

	case eventMsg:
		m.renderEvent(agent.Event(msg))
		if msg.Kind == agent.EventDone || msg.Kind == agent.EventError {
			m.running = false
		} else {
			cmds = append(cmds, waitForEvent(m.events)) // keep listening
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)
	m.vp, cmd = m.vp.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *model) renderEvent(e agent.Event) {
	switch e.Kind {
	case agent.EventAssistant:
		m.append(botStyle.Render("🤖 " + e.Text))
	case agent.EventToolCall:
		m.append(toolStyle.Render("→ " + e.Tool + " " + e.Args))
	case agent.EventToolResult:
		m.append(resultStyle.Render(indent(clip(e.Text, 40))))
	case agent.EventError:
		m.append(errStyle.Render("✖ " + e.Text))
	case agent.EventDone:
		m.append(botStyle.Render("✓ done"))
	}
}

func (m *model) append(s string) {
	m.lines = append(m.lines, s)
	if m.ready {
		m.vp.SetContent(strings.Join(m.lines, "\n"))
		m.vp.GotoBottom()
	}
}

func (m model) View() string {
	if !m.ready {
		return "starting…"
	}
	status := "ready"
	if m.running {
		status = "running…"
	}
	header := headerStyle.Render(fmt.Sprintf("pentest-mcp · %s/%s · model %s · %s",
		config.C.Operator, config.C.Engagement, config.C.ModelName, status))
	help := helpStyle.Render("Enter: run · ↑/↓ or PgUp/PgDn: scroll · Ctrl+C: quit")
	return strings.Join([]string{header, m.vp.View(), m.input.View(), help}, "\n")
}

func (m model) welcome() string {
	return botStyle.Render("pentest-mcp agent host") + "\n\n" +
		config.C.Banner() + "\n\n" +
		helpStyle.Render("Type a task below and press Enter. The agent calls server_info first, "+
			"works the methodology, records findings, then renders the report.")
}

// clip keeps at most n lines of a tool result for display.
func clip(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = append(lines[:n], fmt.Sprintf("…[+%d more lines]", len(lines)-n))
	}
	return strings.Join(lines, "\n")
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}
