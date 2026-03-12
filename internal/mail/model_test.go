package mail

import (
	"strings"
	"testing"
)

func TestSummaryAndDetailModelsExposeExpectedFields(t *testing.T) {
	summary := Summary{
		UID:             123,
		Date:            "2026-03-12T10:00:00Z",
		From:            "Alice <alice@example.com>",
		To:              []string{"Bob <bob@example.com>"},
		Subject:         "Hello",
		Seen:            true,
		AttachmentCount: 1,
	}

	if summary.UID != 123 {
		t.Fatalf("UID = %d, want 123", summary.UID)
	}
	if summary.Date == "" {
		t.Fatalf("Date should not be empty")
	}
	if len(summary.To) != 1 {
		t.Fatalf("To length = %d, want 1", len(summary.To))
	}
	if !summary.Seen {
		t.Fatalf("Seen = false, want true")
	}
	if summary.AttachmentCount != 1 {
		t.Fatalf("AttachmentCount = %d, want 1", summary.AttachmentCount)
	}

	detail := Detail{
		Summary:  summary,
		CC:       []string{"Carol <carol@example.com>"},
		Flags:    []string{"\\Seen"},
		TextBody: "Plain body",
		HTMLBody: "<p>Plain body</p>",
		Attachments: []Attachment{
			{Name: "report.pdf", ContentType: "application/pdf", Size: 1024},
		},
		Headers: map[string]string{"Message-ID": "<123@example.com>"},
	}

	if detail.TextBody != "Plain body" {
		t.Fatalf("TextBody = %q, want %q", detail.TextBody, "Plain body")
	}
	if len(detail.CC) != 1 {
		t.Fatalf("CC length = %d, want 1", len(detail.CC))
	}
	if len(detail.Attachments) != 1 {
		t.Fatalf("Attachments length = %d, want 1", len(detail.Attachments))
	}
	if detail.Headers["Message-ID"] == "" {
		t.Fatalf("expected Message-ID header")
	}
}

func TestParseMessageBuildsDetailFromRawRFC822(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Cc: Carol <carol@example.com>",
		"Subject: Hello from IMAP",
		"Date: Thu, 12 Mar 2026 10:00:00 +0000",
		"Message-ID: <123@example.com>",
		"MIME-Version: 1.0",
		"Content-Type: multipart/mixed; boundary=boundary42",
		"",
		"--boundary42",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"Plain body",
		"--boundary42",
		"Content-Type: text/html; charset=UTF-8",
		"",
		"<p>Plain body</p>",
		"--boundary42",
		"Content-Type: application/pdf",
		"Content-Disposition: attachment; filename=report.pdf",
		"",
		"PDFDATA",
		"--boundary42--",
		"",
	}, "\r\n")

	detail, err := ParseMessage(777, []string{"\\Seen"}, strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseMessage returned error: %v", err)
	}

	if detail.UID != 777 {
		t.Fatalf("UID = %d, want 777", detail.UID)
	}
	if detail.Subject != "Hello from IMAP" {
		t.Fatalf("Subject = %q, want %q", detail.Subject, "Hello from IMAP")
	}
	if detail.From != "Alice <alice@example.com>" {
		t.Fatalf("From = %q", detail.From)
	}
	if len(detail.To) != 1 || detail.To[0] != "Bob <bob@example.com>" {
		t.Fatalf("unexpected To: %+v", detail.To)
	}
	if len(detail.CC) != 1 || detail.CC[0] != "Carol <carol@example.com>" {
		t.Fatalf("unexpected CC: %+v", detail.CC)
	}
	if detail.TextBody != "Plain body" {
		t.Fatalf("TextBody = %q, want %q", detail.TextBody, "Plain body")
	}
	if detail.HTMLBody != "<p>Plain body</p>" {
		t.Fatalf("HTMLBody = %q, want %q", detail.HTMLBody, "<p>Plain body</p>")
	}
	if len(detail.Attachments) != 1 || detail.Attachments[0].Name != "report.pdf" {
		t.Fatalf("unexpected attachments: %+v", detail.Attachments)
	}
	if detail.AttachmentCount != 1 {
		t.Fatalf("AttachmentCount = %d, want 1", detail.AttachmentCount)
	}
	if detail.Headers["Message-ID"] != "<123@example.com>" {
		t.Fatalf("Message-ID = %q", detail.Headers["Message-ID"])
	}
	if !detail.Seen {
		t.Fatalf("Seen = false, want true")
	}
}
