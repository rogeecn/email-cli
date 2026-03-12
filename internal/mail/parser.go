package mail

import (
	"io"
	"strings"
	"time"

	message "github.com/emersion/go-message"
	mailmessage "github.com/emersion/go-message/mail"
)

func ParseMessage(uid uint32, flags []string, reader io.Reader) (Detail, error) {
	entity, err := message.Read(reader)
	if err != nil {
		return Detail{}, err
	}

	header := mailmessage.Header{Header: entity.Header}
	detail := Detail{
		Summary: Summary{
			UID:  uid,
			Seen: containsFlag(flags, "\\Seen"),
		},
		Flags:   append([]string(nil), flags...),
		Headers: map[string]string{},
	}

	if date, err := header.Date(); err == nil {
		detail.Date = date.UTC().Format(time.RFC3339)
	}
	if from, err := header.AddressList("From"); err == nil {
		detail.From = formatAddressList(from)
	}
	if to, err := header.AddressList("To"); err == nil {
		detail.To = formatAddresses(to)
	}
	if cc, err := header.AddressList("Cc"); err == nil {
		detail.CC = formatAddresses(cc)
	}
	if bcc, err := header.AddressList("Bcc"); err == nil {
		detail.BCC = formatAddresses(bcc)
	}
	if subject, err := header.Subject(); err == nil {
		detail.Subject = subject
	}

	fields := entity.Header.Fields()
	for fields.Next() {
		key := fields.Key()
		value := fields.Value()
		detail.Headers[key] = value
		if strings.EqualFold(key, "Message-ID") {
			detail.Headers["Message-ID"] = value
		}
	}

	mediaType, _, _ := entity.Header.ContentType()
	if strings.HasPrefix(mediaType, "multipart/") {
		if err := parseMultipartInto(&detail, mailmessage.NewReader(entity)); err != nil {
			return Detail{}, err
		}
	} else {
		body, err := io.ReadAll(entity.Body)
		if err != nil {
			return Detail{}, err
		}
		content := strings.TrimSpace(string(body))
		if strings.Contains(mediaType, "html") {
			detail.HTMLBody = content
		} else {
			detail.TextBody = content
		}
	}

	return detail, nil
}

func parseMultipartInto(detail *Detail, reader *mailmessage.Reader) error {
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch header := part.Header.(type) {
		case *mailmessage.InlineHeader:
			contentType, _, _ := header.ContentType()
			data, err := io.ReadAll(part.Body)
			if err != nil {
				return err
			}
			content := strings.TrimSpace(string(data))
			if strings.Contains(contentType, "html") {
				detail.HTMLBody = content
			} else if detail.TextBody == "" {
				detail.TextBody = content
			}
		case *mailmessage.AttachmentHeader:
			filename, _ := header.Filename()
			contentType, _, _ := header.ContentType()
			data, err := io.ReadAll(part.Body)
			if err != nil {
				return err
			}
			detail.Attachments = append(detail.Attachments, Attachment{
				Name:        filename,
				ContentType: contentType,
				Size:        int64(len(data)),
			})
			detail.AttachmentCount = len(detail.Attachments)
		}
	}
}

func formatAddressList(addresses []*mailmessage.Address) string {
	formatted := formatAddresses(addresses)
	if len(formatted) == 0 {
		return ""
	}
	return formatted[0]
}

func formatAddresses(addresses []*mailmessage.Address) []string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if address == nil {
			continue
		}
		if address.Name != "" {
			formatted = append(formatted, address.Name+" <"+address.Address+">")
			continue
		}
		formatted = append(formatted, address.Address)
	}
	return formatted
}

func containsFlag(flags []string, target string) bool {
	for _, flag := range flags {
		if flag == target {
			return true
		}
	}
	return false
}
