package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	sessions    []Session
	selectedIdx int
	quitting    bool
	width       int
	height      int
}

var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("white")).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("white"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("cyan")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

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

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "enter", "o":
			if len(m.sessions) > 0 && m.selectedIdx < len(m.sessions) {
				selected := m.sessions[m.selectedIdx]
				executeSwitch(selected.Name)
				m.quitting = true
				return m, tea.Quit
			}
		case "j", "down", "right":
			if m.selectedIdx < len(m.sessions)-1 {
				m.selectedIdx++
			}
		case "k", "up", "left":
			if m.selectedIdx > 0 {
				m.selectedIdx--
			}
		case "g", "home":
			m.selectedIdx = 0
		case "G", "end":
			if len(m.sessions) > 0 {
				m.selectedIdx = len(m.sessions) - 1
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var lines []string

	lines = append(lines, titleStyle.Render("Beacon Sessions"))
	lines = append(lines, "")

	if len(m.sessions) == 0 {
		lines = append(lines, dimStyle.Render("No active sessions"))
	} else {
		maxItems := m.height - 5
		if maxItems < 1 {
			maxItems = len(m.sessions)
		}

		startIdx := 0
		endIdx := len(m.sessions)
		if len(m.sessions) > maxItems {
			// Center selection in viewport
			startIdx = m.selectedIdx - maxItems/2
			endIdx = startIdx + maxItems
			if startIdx < 0 {
				startIdx = 0
				endIdx = maxItems
			}
			if endIdx > len(m.sessions) {
				endIdx = len(m.sessions)
				startIdx = endIdx - maxItems
			}
		}

		for i := startIdx; i < endIdx; i++ {
			session := m.sessions[i]
			icon := getSessionIcon(session)
			agentIcon := getAgentIcon(session.Agent)
			statusAbbr := getStatusAbbr(session.Status)
			timeAgo := formatRelativeTime(session.UpdatedAt)

			itemStr := fmt.Sprintf("%s  %-20s  %s  %-6s  %s",
				icon,
				session.Name,
				agentIcon,
				statusAbbr,
				timeAgo)

			if i == m.selectedIdx {
				lines = append(lines, selectedStyle.Render("▶ "+itemStr))
			} else {
				lines = append(lines, itemStyle.Render("  "+itemStr))
			}
		}
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("⏎ switch  ·  ↑↓ navigate  ·  q quit"))

	return lipgloss.NewStyle().Width(m.width).Render(strings.Join(lines, "\n"))
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

func formatSessionItem(session Session) string {
	icon := getSessionIcon(session)
	agentIcon := getAgentIcon(session.Agent)
	statusAbbr := getStatusAbbr(session.Status)
	timeAgo := formatRelativeTime(session.UpdatedAt)

	return fmt.Sprintf("%s  %-20s  %s  %-6s  %s",
		icon,
		session.Name,
		agentIcon,
		statusAbbr,
		timeAgo)
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

func RunTUI(dir string) error {
	now := time.Now()
	sessions := loadSessions(dir, now)
	sessions = sortSessions(sessions)

	m := model{
		sessions:    sessions,
		selectedIdx: 0,
	}

	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
