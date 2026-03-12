package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rogeecn/email-cli/internal/mail"
	"gopkg.in/yaml.v3"
)

type ListMetadata struct {
	Total  int
	Limit  int
	Offset int
}

func RenderSummaries(summaries []mail.Summary, format string, metadata ListMetadata) ([]byte, error) {
	switch normalizedFormat(format) {
	case "plain":
		return renderSummariesPlain(summaries, metadata), nil
	case "json":
		return json.MarshalIndent(summaries, "", "  ")
	case "yaml":
		return yaml.Marshal(summaries)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func RenderDetail(detail mail.Detail, format string, showHeaders bool) ([]byte, error) {
	switch normalizedFormat(format) {
	case "plain":
		return renderDetailPlain(detail, showHeaders), nil
	case "json":
		return json.MarshalIndent(detail, "", "  ")
	case "yaml":
		return yaml.Marshal(detail)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}

func normalizedFormat(format string) string {
	if format == "" {
		return "plain"
	}
	return strings.ToLower(format)
}

func renderSummariesPlain(summaries []mail.Summary, metadata ListMetadata) []byte {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "Showing %d emails (total %d, limit %d, offset %d)\n\n", len(summaries), metadata.Total, metadata.Limit, metadata.Offset)
	for index, summary := range summaries {
		buffer.WriteString(summary.Subject)
		buffer.WriteByte('\n')
		fmt.Fprintf(&buffer, "  id: %s\n", strconv.FormatUint(uint64(summary.UID), 10))
		fmt.Fprintf(&buffer, "  from: %s\n", summary.From)
		fmt.Fprintf(&buffer, "  to: %s\n", strings.Join(summary.To, ", "))
		fmt.Fprintf(&buffer, "  received: %s\n", summary.Date)
		fmt.Fprintf(&buffer, "  attachments: %s (%d)\n", attachmentLabel(summary.AttachmentCount), summary.AttachmentCount)
		if index < len(summaries)-1 {
			buffer.WriteByte('\n')
		}
	}

	return buffer.Bytes()
}

func attachmentLabel(count int) string {
	if count > 0 {
		return "yes"
	}
	return "no"
}

func renderDetailPlain(detail mail.Detail, showHeaders bool) []byte {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "UID: %d\n", detail.UID)
	fmt.Fprintf(&buffer, "Date: %s\n", detail.Date)
	fmt.Fprintf(&buffer, "From: %s\n", detail.From)
	fmt.Fprintf(&buffer, "To: %s\n", strings.Join(detail.To, ", "))
	if len(detail.CC) > 0 {
		fmt.Fprintf(&buffer, "CC: %s\n", strings.Join(detail.CC, ", "))
	}
	if len(detail.BCC) > 0 {
		fmt.Fprintf(&buffer, "BCC: %s\n", strings.Join(detail.BCC, ", "))
	}
	fmt.Fprintf(&buffer, "Subject: %s\n", detail.Subject)
	fmt.Fprintf(&buffer, "Seen: %t\n", detail.Seen)
	if len(detail.Flags) > 0 {
		fmt.Fprintf(&buffer, "Flags: %s\n", strings.Join(detail.Flags, ", "))
	}

	buffer.WriteString("\nBody:\n")
	if detail.TextBody != "" {
		buffer.WriteString(detail.TextBody)
		buffer.WriteByte('\n')
	} else if detail.HTMLBody != "" {
		buffer.WriteString(detail.HTMLBody)
		buffer.WriteByte('\n')
	}

	if len(detail.Attachments) > 0 {
		buffer.WriteString("\nAttachments:\n")
		for _, attachment := range detail.Attachments {
			fmt.Fprintf(&buffer, "- %s (%s, %d bytes)\n", attachment.Name, attachment.ContentType, attachment.Size)
		}
	}

	if showHeaders && len(detail.Headers) > 0 {
		buffer.WriteString("\nHeaders:\n")
		for key, value := range detail.Headers {
			fmt.Fprintf(&buffer, "%s: %s\n", key, value)
		}
	}

	return buffer.Bytes()
}
