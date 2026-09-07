package tmux

import (
	"testing"
	"time"
)

func TestSanitizeSessionName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean alphanumeric",
			input:    "my-session_123",
			expected: "my-session_123",
		},
		{
			name:     "spaces to dashes",
			input:    "my session name",
			expected: "my-session-name",
		},
		{
			name:     "consecutive and surrounding whitespace",
			input:    "  spaced  out  ",
			expected: "--spaced--out--",
		},
		{
			name:     "special characters stripped",
			input:    "sess:ion.1!@#$%^&*()+={}[]|\\;:'\",<>/?",
			expected: "session1",
		},
		{
			name:     "mixed unicode accents stripped",
			input:    "séssîon ñamë",
			expected: "ssson-am",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSessionName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeSessionName(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSessionTarget(t *testing.T) {
	t.Parallel()
	if got := sessionTarget("work"); got != "=work" {
		t.Errorf("sessionTarget(%q) = %q; want %q", "work", got, "=work")
	}
	if got := paneTarget("work"); got != "=work:" {
		t.Errorf("paneTarget(%q) = %q; want %q", "work", got, "=work:")
	}
}

func TestParseSessionList(t *testing.T) {
	t.Parallel()
	t.Run("empty output", func(t *testing.T) {
		got := ParseSessionList("")
		if len(got) != 0 {
			t.Errorf("expected empty slice, got %d items", len(got))
		}
	})

	t.Run("single session", func(t *testing.T) {
		// Epoch 1700000000 -> 2023-11-14T22:13:20Z
		input := "worker-1\t2\t1700000000\t1\t/home/user/project\n"
		got := ParseSessionList(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 session, got %d", len(got))
		}
		s := got[0]
		if s.Name != "worker-1" {
			t.Errorf("expected Name 'worker-1', got %q", s.Name)
		}
		if s.Windows != 2 {
			t.Errorf("expected Windows 2, got %d", s.Windows)
		}
		expectedTime := time.Unix(1700000000, 0).UTC().Format(time.RFC3339)
		if s.CreatedAt != expectedTime {
			t.Errorf("expected CreatedAt %q, got %q", expectedTime, s.CreatedAt)
		}
		if !s.IsAttached {
			t.Errorf("expected IsAttached true, got false")
		}
		if s.Path != "/home/user/project" {
			t.Errorf("expected Path '/home/user/project', got %q", s.Path)
		}
	})

	t.Run("multiple sessions with pipe in path", func(t *testing.T) {
		input := "s1\t1\t1700000000\t0\t/tmp/dir|with|pipe\ns2\t3\t1700000100\t1\t/home/esteban\n"
		got := ParseSessionList(input)
		if len(got) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(got))
		}
		if got[0].Path != "/tmp/dir|with|pipe" {
			t.Errorf("expected path to preserve pipe characters, got %q", got[0].Path)
		}
		if got[0].IsAttached {
			t.Errorf("expected s1 IsAttached false")
		}
		if !got[1].IsAttached {
			t.Errorf("expected s2 IsAttached true")
		}
	})

	t.Run("malformed lines skipped", func(t *testing.T) {
		input := "incomplete\tline\ns3\t1\t1700000000\t0\t/tmp\n"
		got := ParseSessionList(input)
		if len(got) != 1 {
			t.Fatalf("expected 1 session, got %d", len(got))
		}
		if got[0].Name != "s3" {
			t.Errorf("expected session s3, got %q", got[0].Name)
		}
	})
}
