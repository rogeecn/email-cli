package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rogeecn/email-cli/internal/mail"
	"gopkg.in/yaml.v3"
)

func sampleSummary() []mail.Summary {
	return []mail.Summary{
		{
			UID:             123,
			Date:            "2026-03-12T10:00:00Z",
			From:            "Alice <alice@example.com>",
			To:              []string{"Bob <bob@example.com>"},
			Subject:         "Hello",
			Seen:            true,
			AttachmentCount: 1,
		},
	}
}

func sampleDetail() mail.Detail {
	return mail.Detail{
		Summary: mail.Summary{
			UID:     123,
			Date:    "2026-03-12T10:00:00Z",
			From:    "Alice <alice@example.com>",
			To:      []string{"Bob <bob@example.com>"},
			Subject: "Hello",
			Seen:    true,
		},
		CC:       []string{"Carol <carol@example.com>"},
		Flags:    []string{"\\Seen"},
		TextBody: "Plain body",
		HTMLBody: "<p>Plain body</p>",
		Attachments: []mail.Attachment{
			{Name: "report.pdf", ContentType: "application/pdf", Size: 1024},
		},
		Headers: map[string]string{"Message-ID": "<123@example.com>"},
	}
}

func sampleHTMLOnlyDetail() mail.Detail {
	return mail.Detail{
		Summary: mail.Summary{
			UID:     456,
			Date:    "2026-03-12T11:00:00Z",
			From:    "Alice <alice@example.com>",
			To:      []string{"Bob <bob@example.com>"},
			Subject: "HTML Only",
			Seen:    false,
		},
		HTMLBody: "<div>Hello <strong>world</strong><br>Line&nbsp;2</div>",
	}
}

func sampleStyledHTMLDetail() mail.Detail {
	return mail.Detail{
		Summary: mail.Summary{
			UID:     789,
			Date:    "2026-03-12T12:00:00Z",
			From:    "Alice <alice@example.com>",
			To:      []string{"Bob <bob@example.com>"},
			Subject: "Styled HTML",
			Seen:    false,
		},
		HTMLBody: "<html><head><style>body{font-size:14px;} p{margin:0;}</style></head><body><p>First</p><p></p><p>Second</p><script>console.log('debug')</script></body></html>",
	}
}

func TestRenderSummaryPlain(t *testing.T) {
	out, err := RenderSummaries(sampleSummary(), "plain", ListMetadata{Total: 1, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("RenderSummaries returned error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "Showing 1 emails (total 1, limit 10, offset 0)") {
		t.Fatalf("plain output should include pagination header")
	}
	if !strings.Contains(text, "Hello\n") {
		t.Fatalf("plain output should include subject as headline")
	}
	if !strings.Contains(text, "  id: 123") {
		t.Fatalf("plain output should include message id")
	}
	if !strings.Contains(text, "  from: Alice <alice@example.com>") {
		t.Fatalf("plain output should include sender")
	}
	if !strings.Contains(text, "  to: Bob <bob@example.com>") {
		t.Fatalf("plain output should include recipients")
	}
	if !strings.Contains(text, "  received: 2026-03-12T10:00:00Z") {
		t.Fatalf("plain output should include receive time")
	}
	if !strings.Contains(text, "  attachments: yes (1)") {
		t.Fatalf("plain output should include attachment summary")
	}
	if strings.Contains(text, "UID\tDATE") {
		t.Fatalf("plain output should no longer render table headers")
	}
}

func TestRenderDetailPlainHidesHeadersByDefault(t *testing.T) {
	out, err := RenderDetail(sampleDetail(), "plain", false)
	if err != nil {
		t.Fatalf("RenderDetail returned error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "Subject: Hello") {
		t.Fatalf("plain detail should include subject line")
	}
	if !strings.Contains(text, "Plain body") {
		t.Fatalf("plain detail should include message body")
	}
	if !strings.Contains(text, "report.pdf") {
		t.Fatalf("plain detail should include attachment name")
	}
	if strings.Contains(text, "Headers:") {
		t.Fatalf("plain detail should hide headers by default")
	}
}

func TestRenderDetailPlainShowsHeadersInDebugMode(t *testing.T) {
	out, err := RenderDetail(sampleDetail(), "plain", true)
	if err != nil {
		t.Fatalf("RenderDetail returned error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "Headers:") {
		t.Fatalf("plain detail should include headers in debug mode")
	}
	if !strings.Contains(text, "Message-ID: <123@example.com>") {
		t.Fatalf("plain detail should include message header values in debug mode")
	}
}

func TestRenderDetailPlainConvertsHTMLBodyToText(t *testing.T) {
	out, err := RenderDetail(sampleHTMLOnlyDetail(), "plain", false)
	if err != nil {
		t.Fatalf("RenderDetail returned error: %v", err)
	}

	text := string(out)
	if !strings.Contains(text, "Hello world") {
		t.Fatalf("plain detail should include converted html text, got %q", text)
	}
	if !strings.Contains(text, "Line 2") {
		t.Fatalf("plain detail should preserve readable line content, got %q", text)
	}
	if strings.Contains(text, "<strong>") || strings.Contains(text, "<div>") || strings.Contains(text, "&nbsp;") {
		t.Fatalf("plain detail should not include raw html markup, got %q", text)
	}
}

func TestRenderDetailPlainRemovesCSSScriptsAndCollapsesBlankLines(t *testing.T) {
	out, err := RenderDetail(sampleStyledHTMLDetail(), "plain", false)
	if err != nil {
		t.Fatalf("RenderDetail returned error: %v", err)
	}

	text := string(out)
	if strings.Contains(text, "font-size") || strings.Contains(text, "console.log") {
		t.Fatalf("plain detail should remove style and script content, got %q", text)
	}
	if !strings.Contains(text, "First\n\nSecond") {
		t.Fatalf("plain detail should keep at most one blank line between paragraphs, got %q", text)
	}
	if strings.Contains(text, "\n\n\n") {
		t.Fatalf("plain detail should collapse repeated blank lines, got %q", text)
	}
}

func TestRenderJSONAndYAML(t *testing.T) {
	jsonOut, err := RenderDetail(sampleDetail(), "json", false)
	if err != nil {
		t.Fatalf("RenderDetail json returned error: %v", err)
	}

	var detailJSON mail.Detail
	if err := json.Unmarshal(jsonOut, &detailJSON); err != nil {
		t.Fatalf("json output is invalid: %v", err)
	}
	if detailJSON.Subject != "Hello" {
		t.Fatalf("json subject = %q, want %q", detailJSON.Subject, "Hello")
	}

	yamlOut, err := RenderDetail(sampleDetail(), "yaml", false)
	if err != nil {
		t.Fatalf("RenderDetail yaml returned error: %v", err)
	}

	var detailYAML mail.Detail
	if err := yaml.Unmarshal(yamlOut, &detailYAML); err != nil {
		t.Fatalf("yaml output is invalid: %v", err)
	}
	if detailYAML.Subject != "Hello" {
		t.Fatalf("yaml subject = %q, want %q", detailYAML.Subject, "Hello")
	}
}
