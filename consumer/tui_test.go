package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeStateFile(t *testing.T, dir, session string, status, agent string, updatedAt time.Time) {
	t.Helper()
	data := fmt.Sprintf(`{
  "status": "%s",
  "agent": "%s",
  "updated_at": "%s"
}`, status, agent, updatedAt.Format(time.RFC3339))
	path := filepath.Join(dir, session+".json")
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}
}

func TestLoadSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	sessions := loadSessions(dir, now)

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestLoadSessions_ParsesStatus(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateFile(t, dir, "test-session", "running", "plan", now.Add(-1*time.Hour))

	sessions := loadSessions(dir, now)

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "test-session" {
		t.Errorf("expected session name 'test-session', got '%s'", sessions[0].Name)
	}
	if sessions[0].Status != StatusRunning {
		t.Errorf("expected status 'running', got '%s'", sessions[0].Status)
	}
	if sessions[0].Agent != "plan" {
		t.Errorf("expected agent 'plan', got '%s'", sessions[0].Agent)
	}
}

func TestLoadSessions_IdleDetection(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateFile(t, dir, "stale-session", "running", "plan", now.Add(-5*time.Hour))

	sessions := loadSessions(dir, now)

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Status != StatusIdle {
		t.Errorf("expected status 'idle' for stale session, got '%s'", sessions[0].Status)
	}
}

func TestLoadSessions_AllStatuses(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateFile(t, dir, "running", "running", "build", now.Add(-1*time.Hour))
	writeStateFile(t, dir, "input", "input_required", "plan", now.Add(-30*time.Minute))
	writeStateFile(t, dir, "finished", "finished", "build", now.Add(-2*time.Hour))
	writeStateFile(t, dir, "idle", "running", "plan", now.Add(-6*time.Hour))

	sessions := loadSessions(dir, now)

	if len(sessions) != 4 {
		t.Fatalf("expected 4 sessions, got %d", len(sessions))
	}
}

func TestSortSessions_AlertsFirst(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateFile(t, dir, "running1", "running", "plan", now.Add(-1*time.Hour))
	writeStateFile(t, dir, "input1", "input_required", "plan", now.Add(-30*time.Minute))

	sessions := loadSessions(dir, now)
	sessions = sortSessions(sessions)

	if sessions[0].Status != StatusInputRequired {
		t.Errorf("expected first session to be input_required, got '%s'", sessions[0].Status)
	}
}

func TestSortSessions_WithinPriorityByTime(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateFile(t, dir, "running-newer", "running", "plan", now.Add(-10*time.Minute))
	writeStateFile(t, dir, "running-older", "running", "build", now.Add(-1*time.Hour))

	sessions := loadSessions(dir, now)
	sessions = sortSessions(sessions)

	if sessions[0].Name != "running-newer" {
		t.Errorf("expected first session to be 'running-newer', got '%s'", sessions[0].Name)
	}
}

func TestGetSessionIcon_Running(t *testing.T) {
	now := time.Now()
	session := Session{Status: StatusRunning, Agent: "plan", UpdatedAt: now.Add(-1 * time.Hour)}
	icon := getSessionIcon(session)
	if icon != "🧠" {
		t.Errorf("expected '🧠' for plan, got '%s'", icon)
	}

	session = Session{Status: StatusRunning, Agent: "build", UpdatedAt: now.Add(-1 * time.Hour)}
	icon = getSessionIcon(session)
	if icon != "🔨" {
		t.Errorf("expected '🔨' for build, got '%s'", icon)
	}

	session = Session{Status: StatusRunning, Agent: "unknown", UpdatedAt: now.Add(-1 * time.Hour)}
	icon = getSessionIcon(session)
	if icon != "⏳" {
		t.Errorf("expected '⏳' for unknown, got '%s'", icon)
	}
}

func TestGetSessionIcon_Alert(t *testing.T) {
	session := Session{Status: StatusInputRequired}
	icon := getSessionIcon(session)
	if icon != "🔔" {
		t.Errorf("expected '🔔' for input_required, got '%s'", icon)
	}
}

func TestGetSessionIcon_Finished(t *testing.T) {
	session := Session{Status: StatusFinished}
	icon := getSessionIcon(session)
	if icon != "✅" {
		t.Errorf("expected '✅' for finished, got '%s'", icon)
	}
}

func TestGetSessionIcon_Idle(t *testing.T) {
	session := Session{Status: StatusIdle}
	icon := getSessionIcon(session)
	if icon != "💤" {
		t.Errorf("expected '💤' for idle, got '%s'", icon)
	}
}

func TestGetSessionIcon_RunningBecomesIdle(t *testing.T) {
	session := Session{
		Status:    StatusRunning,
		Agent:     "plan",
		UpdatedAt: time.Now().Add(-5 * time.Hour),
	}
	icon := getSessionIcon(session)
	if icon != "💤" {
		t.Errorf("expected '💤' for stale running session, got '%s'", icon)
	}
}

func TestGetAgentIcon(t *testing.T) {
	if getAgentIcon("plan") != "🧠" {
		t.Errorf("expected '🧠' for plan")
	}
	if getAgentIcon("build") != "🔨" {
		t.Errorf("expected '🔨' for build")
	}
	if getAgentIcon("") != "⏳" {
		t.Errorf("expected '⏳' for empty/unknown")
	}
}

func TestGetStatusAbbr(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected string
	}{
		{StatusRunning, "run"},
		{StatusInputRequired, "input"},
		{StatusFinished, "done"},
		{StatusIdle, "idle"},
	}

	for _, tt := range tests {
		if got := getStatusAbbr(tt.status); got != tt.expected {
			t.Errorf("getStatusAbbr(%s) = %s, want %s", tt.status, got, tt.expected)
		}
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		diff     time.Duration
		expected string
	}{
		{30 * time.Second, "now"},
		{2 * time.Minute, "2m ago"},
		{30 * time.Minute, "30m ago"},
		{1 * time.Hour, "1h ago"},
		{5 * time.Hour, "5h ago"},
		{1 * 24 * time.Hour, "1d ago"},
		{2 * 24 * time.Hour, "2d ago"},
	}

	for _, tt := range tests {
		timestamp := now.Add(-tt.diff)
		got := formatRelativeTime(timestamp)
		if got != tt.expected {
			t.Errorf("formatRelativeTime(%v) = %s, want %s", tt.diff, got, tt.expected)
		}
	}
}

func TestFormatSessionItem(t *testing.T) {
	session := Session{
		Name:      "github_test",
		Agent:     "plan",
		Status:    StatusRunning,
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	}

	item := formatSessionItem(session)
	expected := "🧠  github_test      🧠      run    5m ago"

	if item != expected {
		t.Errorf("formatSessionItem() = %s, want %s", item, expected)
	}
}

func TestGetSessionPriority(t *testing.T) {
	tests := []struct {
		status   SessionStatus
		expected int
	}{
		{StatusInputRequired, 1},
		{StatusRunning, 2},
		{StatusFinished, 3},
		{StatusIdle, 4},
	}

	for _, tt := range tests {
		session := Session{Status: tt.status}
		if got := getSessionPriority(session); got != tt.expected {
			t.Errorf("getSessionPriority(%s) = %d, want %d", tt.status, got, tt.expected)
		}
	}
}

func TestParseJSON(t *testing.T) {
	data := `{
  "status": "running",
  "agent": "plan",
  "updated_at": "2026-03-20T12:00:00Z"
}`

	var state State
	err := parseJSON(data, &state)
	if err != nil {
		t.Fatalf("parseJSON failed: %v", err)
	}

	if state.Status != "running" {
		t.Errorf("expected status 'running', got '%s'", state.Status)
	}
	if state.Agent != "plan" {
		t.Errorf("expected agent 'plan', got '%s'", state.Agent)
	}
	if state.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}
