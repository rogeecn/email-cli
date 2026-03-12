package app

import "testing"

func TestBuildRequestUsesCLIOverridesAndDefaults(t *testing.T) {
	accountDefaults := AccountDefaults{
		Mailbox:  "INBOX",
		PageSize: 20,
		Format:   "plain",
	}

	request := BuildRequest(Options{}, accountDefaults)
	if request.Mailbox != "INBOX" {
		t.Fatalf("Mailbox = %q, want %q", request.Mailbox, "INBOX")
	}
	if request.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", request.Limit)
	}
	if request.Offset != 0 {
		t.Fatalf("Offset = %d, want 0", request.Offset)
	}
	if request.Format != "plain" {
		t.Fatalf("Format = %q, want %q", request.Format, "plain")
	}
	if request.DetailUID != 0 {
		t.Fatalf("DetailUID = %d, want 0", request.DetailUID)
	}

	request = BuildRequest(Options{
		Mailbox: "Archive",
		Limit:   50,
		Offset:  5,
		Format:  "json",
		UID:     12345,
	}, accountDefaults)

	if request.Mailbox != "Archive" {
		t.Fatalf("Mailbox = %q, want %q", request.Mailbox, "Archive")
	}
	if request.Limit != 50 {
		t.Fatalf("Limit = %d, want 50", request.Limit)
	}
	if request.Offset != 5 {
		t.Fatalf("Offset = %d, want 5", request.Offset)
	}
	if request.Format != "json" {
		t.Fatalf("Format = %q, want %q", request.Format, "json")
	}
	if request.DetailUID != 12345 {
		t.Fatalf("DetailUID = %d, want 12345", request.DetailUID)
	}
}
