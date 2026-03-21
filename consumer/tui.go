package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SessionStatus string

const (
	StatusRunning       SessionStatus = "running"
	StatusInputRequired SessionStatus = "input_required"
	StatusFinished      SessionStatus = "finished"
	StatusIdle          SessionStatus = "idle"
)

type Session struct {
	Name      string
	Agent     string
	Status    SessionStatus
	UpdatedAt time.Time
}

type model struct {
	table        table.Model
	sessions     []Session
	selectedName string
	quitting     bool
}

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("white")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

func getSessionIcon(session Session) string {
	if session.Status == StatusRunning && time.Since(session.UpdatedAt) > staleDuration {
		return "💤"
	}

	switch session.Status {
	case StatusRunning:
		switch session.Agent {
		case "plan":
			return "🧠"
		case "build":
			return "🔨"
		default:
			return "⏳"
		}
	case StatusInputRequired:
		return "🔔"
	case StatusFinished:
		return "✅"
	case StatusIdle:
		return "💤"
	default:
		return "❓"
	}
}

func getAgentIcon(agent string) string {
	switch agent {
	case "plan":
		return "🧠"
	case "build":
		return "🔨"
	default:
		return "⏳"
	}
}

func getStatusAbbr(status SessionStatus) string {
	switch status {
	case StatusRunning:
		return "run"
	case StatusInputRequired:
		return "input"
	case StatusFinished:
		return "done"
	case StatusIdle:
		return "idle"
	default:
		return "?"
	}
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours())/24)
}

func buildTableRows(sessions []Session) []table.Row {
	rows := make([]table.Row, len(sessions))
	for i, s := range sessions {
		rows[i] = table.Row{
			getSessionIcon(s),
			s.Name,
			getAgentIcon(s.Agent),
			getStatusAbbr(s.Status),
			formatRelativeTime(s.UpdatedAt),
		}
	}
	return rows
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter", "o":
			if len(m.sessions) > 0 {
				idx := m.table.Cursor()
				if idx < len(m.sessions) {
					m.selectedName = m.sessions[idx].Name
					m.quitting = true
					return m, tea.Quit
				}
			}
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		if m.selectedName != "" {
			executeSwitch(m.selectedName)
		}
		return ""
	}

	header := titleStyle.Render("Beacon Sessions") + "\n\n"
	return header + m.table.View() + "\n" + helpStyle.Render("⏎ switch  ·  ↑↓ navigate  ·  q quit")
}

func executeSwitch(sessionName string) {
	cmd := exec.Command("tmux", "switch-client", "-t", sessionName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func loadSessions(dir string, now time.Time) []Session {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		return []Session{}
	}

	var sessions []Session

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var state State
		if err := parseJSON(string(data), &state); err != nil {
			continue
		}

		sessionName := strings.TrimSuffix(filepath.Base(file), ".json")

		status := SessionStatus(state.Status)

		if status == StatusRunning && now.Sub(state.UpdatedAt) > staleDuration {
			status = StatusIdle
		}

		sessions = append(sessions, Session{
			Name:      sessionName,
			Agent:     state.Agent,
			Status:    status,
			UpdatedAt: state.UpdatedAt,
		})
	}

	return sessions
}

func parseJSON(data string, state *State) error {
	status := ""
	agent := ""

	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"status"`) {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				status = strings.Trim(strings.Join(parts[1:], ":"), `", `)
			}
		}
		if strings.HasPrefix(line, `"agent"`) {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				agent = strings.Trim(strings.Join(parts[1:], ":"), `", `)
			}
		}
		if strings.HasPrefix(line, `"updated_at"`) {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				ts := strings.Trim(strings.Join(parts[1:], ":"), `", `)
				t, err := time.Parse(time.RFC3339, ts)
				if err == nil {
					state.UpdatedAt = t
				}
			}
		}
	}

	state.Status = status
	state.Agent = agent
	return nil
}

func sortSessions(sessions []Session) []Session {
	sort.Slice(sessions, func(i, j int) bool {
		iPriority := getSessionPriority(sessions[i])
		jPriority := getSessionPriority(sessions[j])

		if iPriority != jPriority {
			return iPriority < jPriority
		}

		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions
}

func getSessionPriority(session Session) int {
	switch session.Status {
	case StatusInputRequired:
		return 1
	case StatusRunning:
		return 2
	case StatusFinished:
		return 3
	case StatusIdle:
		return 4
	default:
		return 5
	}
}

func formatSessionItem(session Session) string {
	icon := getSessionIcon(session)
	agentIcon := getAgentIcon(session.Agent)
	statusAbbr := getStatusAbbr(session.Status)
	timeAgo := formatRelativeTime(session.UpdatedAt)

	return fmt.Sprintf("%s  %s  %s  %s  %s",
		icon,
		session.Name,
		agentIcon,
		statusAbbr,
		timeAgo)
}

func RunTUI(dir string) error {
	now := time.Now()
	sessions := loadSessions(dir, now)
	sessions = sortSessions(sessions)

	columns := []table.Column{
		{Title: "", Width: 2},
		{Title: "SESSION", Width: 20},
		{Title: "AGENT", Width: 5},
		{Title: "STATUS", Width: 6},
		{Title: "SINCE", Width: 10},
	}

	rows := buildTableRows(sessions)

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		Foreground(lipgloss.Color("240"))
	s.Selected = s.Selected.
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false)
	t.SetStyles(s)

	m := model{
		table:        t,
		sessions:     sessions,
		selectedName: "",
	}

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
