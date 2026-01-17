package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gerund/ralph/internal/db"
)

func TestNewLearningsModel(t *testing.T) {
	project := &db.Project{
		ID:   "proj-1",
		Name: "Test Project",
	}

	model := NewLearningsModel(project)

	if model.project != project {
		t.Error("project not set correctly")
	}
	if model.completed {
		t.Error("model should not be completed initially")
	}
	if model.err != nil {
		t.Error("model should not have an error initially")
	}
	if model.status != "Capturing learnings..." {
		t.Errorf("unexpected initial status: %s", model.status)
	}
}

func TestLearningsModel_Update_WindowSize(t *testing.T) {
	model := NewLearningsModel(nil)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := model.Update(msg)

	if updated.width != 100 {
		t.Errorf("width not updated: %d", updated.width)
	}
	if updated.height != 50 {
		t.Errorf("height not updated: %d", updated.height)
	}
}

func TestLearningsModel_Update_LearningsCaptured_Success(t *testing.T) {
	model := NewLearningsModel(nil)

	msg := LearningsCapturedMsg{Err: nil}
	updated, _ := model.Update(msg)

	if !updated.completed {
		t.Error("model should be completed")
	}
	if updated.err != nil {
		t.Error("model should not have an error")
	}
	if updated.status != "Learnings captured successfully!" {
		t.Errorf("unexpected status: %s", updated.status)
	}
}

func TestLearningsModel_Update_LearningsCaptured_Error(t *testing.T) {
	model := NewLearningsModel(nil)

	testErr := errors.New("test error")
	msg := LearningsCapturedMsg{Err: testErr}
	updated, _ := model.Update(msg)

	if !updated.completed {
		t.Error("model should be completed")
	}
	if updated.err != testErr {
		t.Error("model should have the error")
	}
	if updated.status != "Failed to capture learnings" {
		t.Errorf("unexpected status: %s", updated.status)
	}
}

func TestLearningsModel_View_InProgress(t *testing.T) {
	model := NewLearningsModel(nil)
	model.width = 80
	model.height = 24

	view := model.View()

	if !strings.Contains(view, "Capturing Learnings") {
		t.Error("view should contain title")
	}
	if !strings.Contains(view, "Capturing learnings...") {
		t.Error("view should show in-progress status")
	}
	if !strings.Contains(view, "AGENTS.md") {
		t.Error("view should mention AGENTS.md")
	}
	if !strings.Contains(view, "README.md") {
		t.Error("view should mention README.md")
	}
}

func TestLearningsModel_View_Completed(t *testing.T) {
	model := NewLearningsModel(nil)
	model.completed = true
	model.status = "Learnings captured successfully!"

	view := model.View()

	if !strings.Contains(view, "Learnings captured successfully!") {
		t.Error("view should show success status")
	}
}

func TestLearningsModel_View_Error(t *testing.T) {
	model := NewLearningsModel(nil)
	model.completed = true
	model.status = "Failed to capture learnings"
	model.err = errors.New("test error")

	view := model.View()

	if !strings.Contains(view, "Failed to capture learnings") {
		t.Error("view should show failed status")
	}
	if !strings.Contains(view, "test error") {
		t.Error("view should show error message")
	}
}

func TestLearningsModel_IsCompleted(t *testing.T) {
	model := NewLearningsModel(nil)

	if model.IsCompleted() {
		t.Error("model should not be completed initially")
	}

	model.completed = true
	if !model.IsCompleted() {
		t.Error("model should be completed")
	}
}

func TestLearningsModel_Error(t *testing.T) {
	model := NewLearningsModel(nil)

	if model.Error() != nil {
		t.Error("model should not have an error initially")
	}

	testErr := errors.New("test error")
	model.err = testErr
	if model.Error() != testErr {
		t.Error("model should return the error")
	}
}

func TestLearningsModel_Init(t *testing.T) {
	model := NewLearningsModel(nil)

	cmd := model.Init()
	if cmd != nil {
		t.Error("Init should return nil")
	}
}
