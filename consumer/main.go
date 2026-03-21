package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
}

var color = Colors{
	Running: "#[fg=cyan]",
	Alert:   "#[fg=red,bold]",
}

type State struct {
	Status    string    `json:"status"`
	Agent     string    `json:"agent"`
	UpdatedAt time.Time `json:"updated_at"`
}

func main() {
	barMode := flag.Bool("bar", false, "Output tmux status bar format")
	tuiMode := flag.Bool("tui", false, "Launch TUI mode")
	flag.Parse()

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	dir := filepath.Join(home, beaconDir)

	if *tuiMode {
		if err := RunTUI(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		}
		return
	}

	if *barMode {
		output := formatBar(dir, time.Now())
		if output != "" {
			fmt.Print(output)
		}
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

		// Stale check: skip if older than 4 hours
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

	// Order: Plan → Build → Unknown → Alert
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
