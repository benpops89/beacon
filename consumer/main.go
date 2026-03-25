package main

import (
	"encoding/json"
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
	if len(os.Args) < 2 {
		printHelp()
		return
	}

	dir := getDir()
	if dir == "" {
		return
	}

	switch os.Args[1] {
	case "status":
		runStatus(dir)
	case "switch":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: beacon switch <session_name>")
			return
		}
		runSwitch(dir, os.Args[2])
	case "list":
		runList(dir, os.Args[2:])
	default:
		printHelp()
	}
}

func printHelp() {
	fmt.Println("Beacon - opencode session tracker")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  beacon status              Output tmux status bar format")
	fmt.Println("  beacon switch <session>   Switch to session and mark as idle")
	fmt.Println("  beacon list               List sessions (plain names)")
	fmt.Println("  beacon list --icons       List sessions with icons")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  beacon status")
	fmt.Println("  beacon switch github/beacon")
	fmt.Println("  beacon list --icons")
}

func getDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, beaconDir)
}

func runStatus(dir string) {
	output := formatBar(dir, time.Now())
	if output != "" {
		fmt.Print(output)
	}
}

func runSwitch(dir string, targetSession string) {
	if os.Getenv("TMUX") == "" {
		return
	}

	cmd := exec.Command("tmux", "switch-client", "-t", targetSession)
	if err := cmd.Run(); err != nil {
		return
	}

	updateSessionStatus(dir, targetSession, StatusIdle)
}

func runList(dir string, args []string) {
	showIcons := false
	for _, arg := range args {
		if arg == "--icons" {
			showIcons = true
		}
	}

	now := time.Now()
	sessions := loadSessions(dir, now)

	if showIcons {
		for _, s := range sessions {
			iconStr := getSessionIcon(s)
			fmt.Println(iconStr + " " + s.Name)
		}
	} else {
		for _, s := range sessions {
			fmt.Println(s.Name)
		}
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

func updateSessionStatus(dir string, sessionName string, status SessionStatus) {
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

	data, err := os.ReadFile(targetFile)
	if err != nil {
		return
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}

	state.Status = string(status)
	state.UpdatedAt = time.Now()

	updatedData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(targetFile, updatedData, 0644)
}
