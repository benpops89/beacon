package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeState(t *testing.T, dir, session string, status, agent string) {
	t.Helper()
	writeStateWithAge(t, dir, session, status, agent, time.Now())
}

func writeStateWithAge(t *testing.T, dir, session string, status, agent string, updatedAt time.Time) {
	t.Helper()
	data, err := json.Marshal(State{
		Status:    status,
		Agent:     agent,
		UpdatedAt: updatedAt,
	})
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	path := filepath.Join(dir, session+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}
}

func TestFormatBar_NoFiles(t *testing.T) {
	dir := t.TempDir()
	output := formatBar(dir, time.Now())
	if output != "" {
		t.Errorf("expected empty output, got: %q", output)
	}
}

func TestFormatBar_PlanAgent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "running", "plan")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🧠#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_BuildAgent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "running", "build")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🔨#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_UnknownAgent(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "running", "")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]⏳#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_PlanAndBuildRunning(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "running", "plan")
	writeState(t, dir, "Session2", "running", "build")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🧠#[default] #[fg=cyan]🔨#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_AlertOnly(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "input_required", "plan")

	output := formatBar(dir, now)
	expected := "#[fg=red,bold]🔔#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_PlanAndAlert(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "RunningSession", "running", "plan")
	writeState(t, dir, "AlertSession", "input_required", "plan")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🧠#[default] #[fg=red,bold]🔔#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_AllAgentsAndAlert(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "PlanSession", "running", "plan")
	writeState(t, dir, "BuildSession", "running", "build")
	writeState(t, dir, "UnknownSession", "running", "explore")
	writeState(t, dir, "AlertSession", "input_required", "plan")

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🧠#[default] #[fg=cyan]🔨#[default] #[fg=cyan]⏳#[default] #[fg=red,bold]🔔#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_FinishedIgnored(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeState(t, dir, "Session1", "finished", "plan")

	output := formatBar(dir, now)
	if output != "" {
		t.Errorf("expected empty output for finished status, got: %q", output)
	}
}

func TestFormatBar_StaleFiles(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeStateWithAge(t, dir, "OldSession", "running", "plan", now.Add(-5*time.Hour))
	writeState(t, dir, "NewSession", "input_required", "plan")

	output := formatBar(dir, now)
	expected := "#[fg=red,bold]🔔#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestFormatBar_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()

	writeState(t, dir, "ValidSession", "running", "plan")

	path := filepath.Join(dir, "InvalidSession.json")
	if err := os.WriteFile(path, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid json: %v", err)
	}

	output := formatBar(dir, now)
	expected := "#[fg=cyan]🧠#[default]"
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}
