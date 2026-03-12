package main

import (
	"flag"
	"testing"
)

func TestParseFlagsReadsAliasAndUID(t *testing.T) {
	options, err := parseFlags([]string{"-A", "personal", "--uid", "42", "--mailbox", "Archive", "--limit", "10", "--format", "json"})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if options.Account != "personal" {
		t.Fatalf("Account = %q, want %q", options.Account, "personal")
	}
	if options.UID != 42 {
		t.Fatalf("UID = %d, want 42", options.UID)
	}
	if options.Mailbox != "Archive" {
		t.Fatalf("Mailbox = %q, want %q", options.Mailbox, "Archive")
	}
	if options.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", options.Limit)
	}
	if options.Format != "json" {
		t.Fatalf("Format = %q, want %q", options.Format, "json")
	}
}

func TestNewFlagSetSupportsShortAlias(t *testing.T) {
	flagSet, _ := newFlagSet()
	if flagSet.Lookup("A") == nil {
		t.Fatalf("short -A flag should be registered")
	}
	if flagSet.Lookup("account") == nil {
		t.Fatalf("--account flag should be registered")
	}
	if flagSet.ErrorHandling() != flag.ContinueOnError {
		t.Fatalf("ErrorHandling = %v, want %v", flagSet.ErrorHandling(), flag.ContinueOnError)
	}
}
