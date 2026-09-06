package tmux

import (
	"strings"
	"testing"
)

func TestDetectInstallInfo(t *testing.T) {
	t.Parallel()
	info := DetectInstallInfo()
	if info.Platform == "" {
		t.Errorf("expected platform to be populated")
	}
	if info.Command == "" {
		t.Errorf("expected command to be populated")
	}
	if !strings.Contains(info.Command, "tmux") {
		t.Errorf("expected command to mention tmux, got %q", info.Command)
	}
}

func TestMissingTmuxPrompt(t *testing.T) {
	t.Parallel()
	prompt := MissingTmuxPrompt()
	if !strings.Contains(prompt, "Error: tmux binary not found in PATH") {
		t.Errorf("expected error title in prompt")
	}
	if !strings.Contains(prompt, "To install tmux on your system") {
		t.Errorf("expected install section in prompt")
	}
	if !strings.Contains(prompt, "After installing tmux, rerun 'operator'") {
		t.Errorf("expected closing instructions in prompt")
	}
}
