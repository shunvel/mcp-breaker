package dashboard

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shunvel/mcp-breaker/pkg/config"
	"github.com/shunvel/mcp-breaker/pkg/telemetry"
)

const maxStreamLines = 20

type tickMsg time.Time

type eventMsg telemetry.Event

// Model is the Bubble Tea dashboard model.
type Model struct {
	cfg       config.Config
	events    []string
	metrics   telemetry.Metrics
	lastSim   float64
	paused    bool
	pattern   string
	sessionID string
	eventLn   net.Listener
	stop      chan struct{}
}

// Run starts the TUI dashboard.
func Run(cfg config.Config) error {
	if err := os.MkdirAll(cfg.RunDir, 0o755); err != nil {
		return err
	}
	ln, err := telemetry.StartEventServer(cfg.EventSocket)
	if err != nil {
		return err
	}
	m := Model{
		cfg:       cfg,
		eventLn:   ln,
		stop:      make(chan struct{}),
		sessionID: cfg.SessionID,
	}
	m.appendLine("dashboard listening on " + cfg.EventSocket)
	p := tea.NewProgram(m, tea.WithAltScreen())
	return p.Start()
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(tick(), waitForEvent(m.eventLn, m.stop))
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func waitForEvent(ln net.Listener, stop <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return nil
				default:
					time.Sleep(50 * time.Millisecond)
					continue
				}
			}
			dec := json.NewDecoder(conn)
			for dec.More() {
				var ev telemetry.Event
				if err := dec.Decode(&ev); err != nil {
					break
				}
				_ = conn.Close()
				return eventMsg(ev)
			}
			_ = conn.Close()
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			close(m.stop)
			if m.eventLn != nil {
				_ = m.eventLn.Close()
			}
			return m, tea.Quit
		case "r", "R":
			_ = telemetry.SendResume(m.cfg.ControlSocket, "")
			m.appendLine("resume sent to control socket")
			return m, waitForEvent(m.eventLn, m.stop)
		}
	case tea.WindowSizeMsg:
		// reserved for future layout
	case tickMsg:
		return m, tick()
	case eventMsg:
		m.applyEvent(telemetry.Event(msg))
		return m, waitForEvent(m.eventLn, m.stop)
	}
	return m, nil
}

func (m *Model) applyEvent(ev telemetry.Event) {
	line := fmt.Sprintf("[%s] %s", ev.Type, ev.Method)
	if ev.Tool != "" {
		line += " " + ev.Tool
	}
	if ev.Pattern != "" {
		line += " " + ev.Pattern
	}
	if ev.Similarity > 0 {
		m.lastSim = ev.Similarity
		line += fmt.Sprintf(" sim=%.3f", ev.Similarity)
	}
	m.appendLine(line)
	if ev.Metrics.TotalFrames > 0 {
		m.metrics = ev.Metrics
	}
	switch ev.Type {
	case telemetry.EventPaused, telemetry.EventGraphTrip:
		m.paused = true
		m.pattern = ev.Pattern
	case telemetry.EventResumed:
		m.paused = false
		m.pattern = ""
	}
	if ev.SessionID != "" {
		m.sessionID = ev.SessionID
	}
}

func (m *Model) appendLine(s string) {
	m.events = append(m.events, s)
	if len(m.events) > maxStreamLines {
		m.events = m.events[len(m.events)-maxStreamLines:]
	}
}

func (m Model) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	alertStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("203"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("mcp-breaker dashboard"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("Metrics"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Frames: %d  Tools/call: %d  Echo: %d  Semantic: %d  Graph: %d  Tokens saved: ~%d\n",
		m.metrics.TotalFrames, m.metrics.ToolsCallCount, m.metrics.EchoTrips,
		m.metrics.SemanticTrips, m.metrics.GraphTrips, m.metrics.TokensSaved))
	b.WriteString(fmt.Sprintf("  Last cosine (N vs N-2): %.3f\n\n", m.lastSim))

	if m.paused {
		b.WriteString(alertStyle.Render("PAUSED — loop detected: " + m.pattern))
		b.WriteString("\n\n")
	}

	b.WriteString(labelStyle.Render("Live stream"))
	b.WriteString("\n")
	for _, line := range m.events {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("[R] Resume  [Q] Quit"))
	return b.String()
}
