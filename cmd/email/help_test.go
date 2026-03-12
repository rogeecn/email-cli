package main

import (
	"strings"
	"testing"
)

func TestFlagSetUsageIncludesExamplesAndConfigPath(t *testing.T) {
	flagSet, _ := newFlagSet()

	var output strings.Builder
	flagSet.SetOutput(&output)
	flagSet.Usage()

	text := output.String()
	if !strings.Contains(text, "Fetch email from IMAP accounts") {
		t.Fatalf("usage should include summary, got %q", text)
	}
	if !strings.Contains(text, "Examples:") {
		t.Fatalf("usage should include examples section, got %q", text)
	}
	if !strings.Contains(text, "email -A personal --uid 12345") {
		t.Fatalf("usage should include detail example, got %q", text)
	}
	if !strings.Contains(text, "--debug") {
		t.Fatalf("usage should include debug flag, got %q", text)
	}
	if !strings.Contains(text, DefaultConfigPath()) {
		t.Fatalf("usage should mention default config path, got %q", text)
	}
}
