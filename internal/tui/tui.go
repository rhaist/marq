// Package tui is the Bubble Tea agent-host front-end (phase 4). It drives the
// shared agent loop against the local model runtime and renders the live
// transcript: operator prompts, model prose, tool calls and tool output. Agent
// events stream in over a channel consumed by a re-subscribing tea.Cmd.
//
// On launch it first shows a setup screen where the operator picks the model
// backend (Ollama / LM Studio / custom), edits the endpoint + model +
// engagement fields, and verifies the endpoint with a /v1/models probe. The
// choices persist to config.C.TUIConfigPath so subsequent launches skip setup
// (Ctrl+S re-opens it).
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"marq/internal/agent"
	"marq/internal/config"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).Padding(0, 1)
	helpStyle   = lipgloss.NewStyle().Faint(true).Padding(0, 1)
	userStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	botStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	resultStyle = lipgloss.NewStyle().Faint(true)
	errStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
	okStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	labelStyle  = lipgloss.NewStyle().Faint(true)
	focusStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63"))
)

type eventMsg agent.Event

// Run starts the TUI agent host.
func Run() error {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ---- views / states --------------------------------------------------------

type view int

const (
	viewSetup view = iota
	viewChat
)

type setupState int

const (
	setupEditing setupState = iota
	setupChecking
	setupChecked
)

// ---- setup form fields -----------------------------------------------------

type fieldKind int

const (
	fieldCycle fieldKind = iota // backend selector (Left/Right to change)
	fieldText                   // free text
)

type field struct {
	label  string
	kind   fieldKind
	input  textinput.Model
	opts   []string
	idx    int
	hint   string
	width  int
	isLong bool // scope field: give it more room
}

// backend presets fill Base URL + API key when the backend selector changes.
type backend struct {
	name    string
	baseURL string
	apiKey  string
	model   string // suggested model (only a placeholder hint)
}

var backends = []backend{
	{name: "Ollama", baseURL: "http://host.docker.internal:11434/v1", apiKey: "ollama", model: "huihui_ai/Qwen3.6-abliterated:27b"},
	{name: "LM Studio", baseURL: "http://host.docker.internal:1234/v1", apiKey: "lm-studio", model: ""},
	{name: "Custom", baseURL: "", apiKey: "", model: ""},
}

const (
	fiBackend = iota
	fiBaseURL
	fiModel
	fiKey
	fiOperator
	fiEngagement
	fiScope
	fieldCount
)

// ---- top-level model -------------------------------------------------------

type model struct {
	view  view
	ready bool
	w, h  int

	// setup
	fields     [fieldCount]field
	focus      int
	setupState setupState
	spinner    spinner.Model
	connOK     bool
	connMsg    string // status line under the form
	savedHint  string // shown briefly after saving

	// chat
	loop    *agent.Loop
	input   textinput.Model
	vp      viewport.Model
	events  chan agent.Event
	lines   []string
	running bool
}

func newModel() model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("63"))

	ti := textinput.New()
	ti.Placeholder = "Describe the task (e.g. footprint example.com) — Enter to run, Ctrl+C to quit"
	ti.Focus()
	ti.CharLimit = 4000

	m := model{
		view:    viewSetup,
		spinner: sp,
		input:   ti,
		events:  make(chan agent.Event, 256),
	}
	m.buildSetupForm()

	// If a persisted config exists, skip setup and go straight to chat — the
	// values were already merged into config.C at Load() time.
	if config.TUIConfigExists(config.C.TUIConfigPath) {
		m.view = viewChat
		m.loop = agent.NewLoop()
	}
	return m
}

// buildSetupForm populates the setup fields from the current config.C, and
// picks the backend preset that matches the configured Base URL (or Custom).
func (m *model) buildSetupForm() {
	c := config.C
	// Resolve the active backend from the URL.
	bIdx := 2 // Custom
	for i, b := range backends {
		if b.baseURL != "" && b.baseURL == c.ModelBaseURL {
			bIdx = i
			break
		}
	}
	m.fields[fiBackend] = newCycleField("Backend", bIdx, backendNames(), "←/→ to change")
	m.fields[fiBaseURL] = newTextField("Base URL", c.ModelBaseURL, "OpenAI-compatible /v1 endpoint", 50)
	m.fields[fiModel] = newTextField("Model", c.ModelName, "model id (from /v1/models)", 40)
	m.fields[fiKey] = newTextField("API key", c.ModelAPIKey, "bearer token (Ollama ignores it)", 30)
	m.fields[fiOperator] = newTextField("Operator", c.Operator, "your name", 20)
	m.fields[fiEngagement] = newTextField("Engagement", c.Engagement, "engagement id", 20)
	m.fields[fiScope] = newLongField("Scope", c.ScopeNote, "authorized targets — set this!", 60)
	m.focus = 0
	m.setupState = setupEditing
}

func backendNames() []string {
	out := make([]string, len(backends))
	for i, b := range backends {
		out[i] = b.name
	}
	return out
}

func newTextField(label, value, hint string, width int) field {
	ti := textinput.New()
	ti.CharLimit = 4096
	ti.SetValue(value)
	ti.Prompt = ""
	return field{label: label, kind: fieldText, input: ti, hint: hint, width: width}
}

func newLongField(label, value, hint string, width int) field {
	f := newTextField(label, value, hint, width)
	f.isLong = true
	return f
}

func newCycleField(label string, idx int, opts []string, hint string) field {
	return field{label: label, kind: fieldCycle, opts: opts, idx: idx, hint: hint}
}

// applyBackendPreset overwrites Base URL + API key with the chosen backend's
// defaults (model is left untouched — it's user-specific). No-op for Custom.
func (m *model) applyBackendPreset() {
	if m.fields[fiBackend].idx >= len(backends) {
		return
	}
	b := backends[m.fields[fiBackend].idx]
	if b.baseURL == "" && b.name == "Custom" {
		return
	}
	m.fields[fiBaseURL].input.SetValue(b.baseURL)
	m.fields[fiKey].input.SetValue(b.apiKey)
}

// setupValues pulls the edited values out of the form.
func (m *model) setupValues() (baseURL, model_, key, op, eng, scope string) {
	return m.fields[fiBaseURL].input.Value(),
		m.fields[fiModel].input.Value(),
		m.fields[fiKey].input.Value(),
		m.fields[fiOperator].input.Value(),
		m.fields[fiEngagement].input.Value(),
		m.fields[fiScope].input.Value()
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if !m.ready {
			if m.view == viewChat {
				bodyH := max(msg.Height-4, 3)
				m.vp = viewport.New(msg.Width, bodyH)
				m.vp.SetContent(m.welcome())
			}
			m.ready = true
		} else if m.view == viewChat {
			m.vp.Width, m.vp.Height = msg.Width, max(msg.Height-4, 3)
		}
		m.input.Width = msg.Width - 4
		// Resize setup field widths to the terminal.
		for i := range m.fields {
			f := m.fields[i]
			if f.kind == fieldText {
				w := f.width
				if w > msg.Width-22 {
					w = msg.Width - 22
				}
				if w < 20 {
					w = 20
				}
				f.input.Width = w
				m.fields[i] = f
			}
		}

	case spinner.TickMsg:
		if m.view == viewSetup && m.setupState == setupChecking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case connCheckMsg:
		m.setupState = setupChecked
		m.connOK = msg.ok
		m.connMsg = msg.msg

	case eventMsg:
		if m.view == viewChat {
			m.renderEvent(agent.Event(msg))
			if msg.Kind == agent.EventDone || msg.Kind == agent.EventError {
				m.running = false
			} else {
				cmds = append(cmds, waitForEvent(m.events))
			}
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
		if m.view == viewSetup {
			cmds = append(cmds, m.handleSetupKey(msg))
		} else {
			cmds = append(cmds, m.handleChatKey(msg))
		}
		return m, tea.Batch(cmds...)
	}

	// Pass through to focused input + viewport (chat view only).
	if m.view == viewChat {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
		m.vp, cmd = m.vp.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// ---- setup view ------------------------------------------------------------

func (m *model) handleSetupKey(msg tea.KeyMsg) tea.Cmd {
	if m.setupState == setupChecking {
		return nil // ignore keys while probing
	}
	if m.setupState == setupChecked {
		switch msg.String() {
		case "enter":
			return m.proceed()
		case "e":
			m.setupState = setupEditing
			m.connMsg = ""
			return nil
		case "r":
			m.connMsg = ""
			return m.startCheck()
		}
		return nil
	}
	// setupEditing
	switch msg.String() {
	case "tab", "down", "ctrl+j":
		m.focus = (m.focus + 1) % fieldCount
		return nil
	case "shift+tab", "up", "ctrl+k":
		m.focus = (m.focus - 1 + fieldCount) % fieldCount
		return nil
	case "left":
		if m.focus == fiBackend && m.fields[fiBackend].idx > 0 {
			m.fields[fiBackend].idx--
			m.applyBackendPreset()
		}
		return nil
	case "right":
		if m.focus == fiBackend && m.fields[fiBackend].idx < len(backends)-1 {
			m.fields[fiBackend].idx++
			m.applyBackendPreset()
		}
		return nil
	case "enter":
		return m.startCheck()
	case "ctrl+s":
		return m.proceed() // save without checking
	}
	// Text input for the focused text field.
	f := m.fields[m.focus]
	if f.kind == fieldText {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		m.fields[m.focus] = f
		return cmd
	}
	return nil
}

// startCheck fires the /v1/models probe as a tea.Cmd and flips to the checking
// state. Requires a non-empty Base URL.
func (m *model) startCheck() tea.Cmd {
	baseURL, _, key, _, _, _ := m.setupValues()
	if strings.TrimSpace(baseURL) == "" {
		m.setupState = setupChecked
		m.connOK = false
		m.connMsg = "Base URL is empty — fill it in first"
		return nil
	}
	m.setupState = setupChecking
	m.connMsg = ""
	return tea.Batch(checkConn(baseURL, key), m.spinner.Tick)
}

// proceed commits the form to config.C, persists it, builds the agent loop and
// switches to the chat view. Used both after a successful check and via
// Ctrl+S (save without checking).
func (m *model) proceed() tea.Cmd {
	baseURL, mdl, key, op, eng, scope := m.setupValues()
	config.C.ModelBaseURL = baseURL
	config.C.ModelName = mdl
	config.C.ModelAPIKey = key
	config.C.Operator = op
	config.C.Engagement = eng
	config.C.ScopeNote = scope
	if err := config.SaveTUIConfig(config.C); err != nil {
		m.savedHint = fmt.Sprintf("could not save config: %v (continuing this session)", err)
	} else {
		m.savedHint = ""
	}
	m.loop = agent.NewLoop()
	m.view = viewChat
	// Seed the chat viewport now that we're leaving setup.
	bodyH := max(m.h-4, 3)
	m.vp = viewport.New(m.w, bodyH)
	m.vp.SetContent(m.welcome())
	return textinput.Blink
}

// connCheckMsg is the result of the /v1/models probe.
type connCheckMsg struct {
	ok  bool
	msg string
}

// checkConn hits the OpenAI-compatible /v1/models endpoint with the same
// client the agent loop uses, so the probe validates the real code path.
func checkConn(baseURL, apiKey string) tea.Cmd {
	return func() tea.Msg {
		client := openai.NewClient(
			option.WithBaseURL(baseURL),
			option.WithAPIKey(apiKey),
		)
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		resp, err := client.Models.List(ctx)
		if err != nil {
			return connCheckMsg{ok: false, msg: "connection failed: " + err.Error()}
		}
		n := len(resp.Data)
		if n == 0 {
			return connCheckMsg{ok: true, msg: "connected — no models listed (load a model in your runtime)"}
		}
		return connCheckMsg{ok: true, msg: fmt.Sprintf("connected — %d model(s) available", n)}
	}
}

func (m model) setupView() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("marq · setup") + "\n\n")
	if m.setupState == setupChecking {
		b.WriteString(helpStyle.Render(m.spinner.View() + " probing endpoint…") + "\n")
	}
	// Render each field.
	for i, f := range m.fields {
		focused := i == m.focus && m.setupState == setupEditing
		lbl := labelStyle.Render(padRight(f.label+":", 13))
		var val string
		switch f.kind {
		case fieldCycle:
			val = fmt.Sprintf("[ %s ]", f.opts[f.idx])
			if focused {
				val = focusStyle.Render(val)
			}
		case fieldText:
			val = f.input.View()
			if focused {
				val = focusStyle.Render(val)
			}
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", lbl, val))
		if focused && f.hint != "" {
			b.WriteString(helpStyle.Render("              " + f.hint) + "\n")
		}
	}
	b.WriteString("\n")
	switch m.setupState {
	case setupEditing:
		b.WriteString(helpStyle.Render("Enter: check & start · Tab/↑↓: move · ←/→: backend · Ctrl+S: save without check · Ctrl+C: quit") + "\n")
	case setupChecked:
		if m.connOK {
			b.WriteString(okStyle.Render("✓ "+m.connMsg) + "\n")
		} else {
			b.WriteString(errStyle.Render("✖ "+m.connMsg) + "\n")
		}
		b.WriteString(helpStyle.Render("Enter: proceed · e: edit · r: retry · Ctrl+C: quit") + "\n")
	}
	if m.savedHint != "" {
		b.WriteString(errStyle.Render(m.savedHint) + "\n")
	}
	return b.String()
}

// ---- chat view -------------------------------------------------------------

func (m *model) handleChatKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		return tea.Quit
	case "ctrl+s":
		// Re-open setup, preloaded with the current config.
		m.view = viewSetup
		m.buildSetupForm()
		m.setupState = setupEditing
		m.connMsg = ""
		m.savedHint = ""
		return nil
	case "enter":
		if !m.running {
			task := strings.TrimSpace(m.input.Value())
			if task != "" {
				m.input.SetValue("")
				return m.start(task)
			}
		}
	}
	return nil
}

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
	if m.view == viewSetup {
		return m.setupView()
	}
	status := "ready"
	if m.running {
		status = "running…"
	}
	header := headerStyle.Render(fmt.Sprintf("marq · %s/%s · model %s · %s",
		config.C.Operator, config.C.Engagement, config.C.ModelName, status))
	help := helpStyle.Render("Enter: run · ↑/↓ or PgUp/PgDn: scroll · Ctrl+S: setup · Ctrl+C: quit")
	return strings.Join([]string{header, m.vp.View(), m.input.View(), help}, "\n")
}

func (m model) welcome() string {
	return botStyle.Render("marq agent host") + "\n\n" +
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

// padRight pads s with spaces to width n (ANSI-aware via lipgloss.Width).
func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
