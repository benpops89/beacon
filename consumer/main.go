package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	staleDuration = 4 * time.Hour
	beaconDir     = ".local/share/beacon"
)

type Icons struct {
	Plan          string
	Build         string
	Unknown       string
	InputRequired string
	Finished      string
	Idle          string
}

type Colors struct {
	Running string
	Alert   string
}

var icon = Icons{
	Plan:          "🧠",
	Build:         "🔨",
	Unknown:       "⏳",
	InputRequired: "🔔",
	Finished:      "✅",
	Idle:          "💤",
}

var color = Colors{
	Running: "#[fg=cyan]",
	Alert:   "#[fg=red,bold]",
}

type State struct {
	Status      string    `json:"status"`
	Agent       string    `json:"agent"`
	SessionName string    `json:"session_name"`
	UpdatedAt   time.Time `json:"updated_at"`
}

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

func main() {
	barMode := flag.Bool("bar", false, "Output tmux status bar format")
	listMode := flag.Bool("list", false, "List sessions for Television integration")
	switchMode := flag.Bool("switch", false, "Switch to a session and mark it as idle")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, beaconDir)

	if *switchMode {
		if len(flag.Args()) < 1 {
			fmt.Fprintf(os.Stderr, "Usage: beacon --switch <session_name>\n")
			return
		}
		targetSession := flag.Args()[0]
		handleSwitch(dir, targetSession)
		return
	}

	if *listMode {
		listSessions(dir, time.Now())
		return
	}

	if *barMode {
		output := formatBar(dir, time.Now())
		if output != "" {
			fmt.Print(output)
		}
	}
}

func listSessions(dir string, now time.Time) {
	sessions := loadSessions(dir, now)
	for _, s := range sessions {
		iconStr := getSessionIcon(s)
		fmt.Println(iconStr + " " + s.Name)
	}
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
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		sessionName := state.SessionName
		if sessionName == "" {
			sessionName = strings.TrimSuffix(filepath.Base(file), ".json")
		}

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

	return sortSessions(sessions)
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

func getSessionIcon(session Session) string {
	if session.Status == StatusRunning && time.Since(session.UpdatedAt) > staleDuration {
		return icon.Idle
	}

	switch session.Status {
	case StatusRunning:
		switch session.Agent {
		case "plan":
			return icon.Plan
		case "build":
			return icon.Build
		default:
			return icon.Unknown
		}
	case StatusInputRequired:
		return icon.InputRequired
	case StatusFinished:
		return icon.Finished
	case StatusIdle:
		return icon.Idle
	default:
		return "❓"
	}
}

func formatBar(dir string, now time.Time) string {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		return ""
	}

	var hasPlan, hasBuild, hasUnknown bool
	var hasAlert bool

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		if now.Sub(state.UpdatedAt) > staleDuration {
			continue
		}

		switch state.Status {
		case "running":
			switch state.Agent {
			case "plan":
				hasPlan = true
			case "build":
				hasBuild = true
			default:
				hasUnknown = true
			}
		case "input_required":
			hasAlert = true
		}
	}

	var parts []string

	if hasPlan {
		parts = append(parts, fmt.Sprintf("%s%s#[default]", color.Running, icon.Plan))
	}
	if hasBuild {
		parts = append(parts, fmt.Sprintf("%s%s#[default]", color.Running, icon.Build))
	}
	if hasUnknown {
		parts = append(parts, fmt.Sprintf("%s%s#[default]", color.Running, icon.Unknown))
	}
	if hasAlert {
		parts = append(parts, fmt.Sprintf("%s%s#[default]", color.Alert, icon.InputRequired))
	}

	return strings.Join(parts, " ")
}

func handleSwitch(dir string, targetSession string) {
	// Switch to target session
	switchCmd := exec.Command("tmux", "switch-client", "-t", targetSession)
	switchCmd.Stdout = os.Stdout
	switchCmd.Stderr = os.Stderr
	switchCmd.Run()

	// Update target session status to idle (acknowledged)
	updateSessionStatus(dir, targetSession, StatusIdle)
}

func updateSessionStatus(dir string, sessionName string, status SessionStatus) {
	// Find the file for this session (check session_name field first, then filename)
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return
	}

	var targetFile string
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var state State
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		// Check if session_name matches, or if filename matches (sanitized)
		sessionFromFile := state.SessionName
		if sessionFromFile == "" {
			sessionFromFile = strings.TrimSuffix(filepath.Base(file), ".json")
		}

		if sessionFromFile == sessionName {
			targetFile = file
			break
		}
	}

	if targetFile == "" {
		return
	}

	// Read existing file
	data, err := os.ReadFile(targetFile)
	if err != nil {
		return
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	// Update status
	state.Status = string(status)
	state.UpdatedAt = time.Now()

	// Write back
	updatedData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(targetFile, updatedData, 0644)
}
