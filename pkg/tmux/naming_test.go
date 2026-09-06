package tmux

import (
	"context"
	"strings"
	"testing"
)

func TestDictionarySizeAndSanity(t *testing.T) {
	if len(EnglishWords) != 500 {
		t.Fatalf("expected 500 English words, got %d", len(EnglishWords))
	}
	if len(SpanishWords) != 500 {
		t.Fatalf("expected 500 Spanish words, got %d", len(SpanishWords))
	}

	enSeen := make(map[string]bool)
	for _, w := range EnglishWords {
		if enSeen[w] {
			t.Errorf("duplicate English word: %s", w)
		}
		enSeen[w] = true
		if SanitizeSessionName(w) != w {
			t.Errorf("English word %q contains invalid characters", w)
		}
	}

	esSeen := make(map[string]bool)
	for _, w := range SpanishWords {
		if esSeen[w] {
			t.Errorf("duplicate Spanish word: %s", w)
		}
		esSeen[w] = true
		if SanitizeSessionName(w) != w {
			t.Errorf("Spanish word %q contains invalid characters", w)
		}
	}
}

func TestGenerateSessionName(t *testing.T) {
	for range 20 {
		name := GenerateSessionName()
		parts := strings.Split(name, "-")
		if len(parts) != 3 {
			t.Fatalf("expected 3 words separated by dash, got %q (parts: %d)", name, len(parts))
		}
		for _, part := range parts {
			if strings.TrimSpace(part) == "" {
				t.Fatalf("empty word in generated name %q", name)
			}
		}
		// Must equal sanitized version
		sanitized := SanitizeSessionName(name)
		if sanitized != name {
			t.Fatalf("generated name %q not sanitized: got %q", name, sanitized)
		}
	}
}

type collisionMockClient struct {
	Client
	existing string
}

func (m *collisionMockClient) HasSession(_ context.Context, name string) (bool, error) {
	return name == m.existing, nil
}

func TestGenerateUniqueSessionName(t *testing.T) {
	first := GenerateSessionName()
	mock := &collisionMockClient{existing: first}

	unique := GenerateUniqueSessionName(t.Context(), mock)
	if unique == first {
		t.Fatalf("expected unique name different from %q, got %q", first, unique)
	}
}
