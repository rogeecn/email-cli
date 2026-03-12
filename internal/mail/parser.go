package mail

import (
	"bytes"
	"io"
	"strings"
	"time"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	message "github.com/emersion/go-message"
	mailmessage "github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
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

	detail.MarkdownBody, err = renderHTMLAsMarkdown(detail.HTMLBody)
	if err != nil {
		return Detail{}, err
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

func renderHTMLAsMarkdown(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}

	root, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return "", err
	}

	body := findBodyNode(root)
	if body == nil {
		body = root
	}

	sanitized := cloneSanitizedHTML(body)
	if sanitized == nil {
		return "", nil
	}

	var buffer bytes.Buffer
	if err := html.Render(&buffer, sanitized); err != nil {
		return "", err
	}

	markdown, err := htmltomarkdown.ConvertString(buffer.String())
	if err != nil {
		return "", err
	}
	markdown = strings.ReplaceAll(markdown, "\u00a0", " ")
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")
	for strings.Contains(markdown, "\n\n") {
		markdown = strings.ReplaceAll(markdown, "\n\n", "\n")
	}

	return strings.TrimSpace(markdown), nil
}

func findBodyNode(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && strings.EqualFold(node.Data, "body") {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findBodyNode(child); found != nil {
			return found
		}
	}
	return nil
}

func cloneSanitizedHTML(node *html.Node) *html.Node {
	if node == nil {
		return nil
	}

	switch node.Type {
	case html.DocumentNode:
		clone := &html.Node{Type: html.DocumentNode}
		appendSanitizedChildren(clone, node)
		return clone
	case html.ElementNode:
		if isRemovedHTMLTag(node.Data) {
			return nil
		}
		clone := &html.Node{
			Type:      html.ElementNode,
			Data:      node.Data,
			DataAtom:  node.DataAtom,
			Namespace: node.Namespace,
			Attr:      filterAllowedHTMLAttributes(node.Attr),
		}
		appendSanitizedChildren(clone, node)
		return clone
	case html.TextNode:
		return &html.Node{Type: html.TextNode, Data: node.Data}
	default:
		clone := &html.Node{Type: node.Type, Data: node.Data, DataAtom: node.DataAtom, Namespace: node.Namespace}
		appendSanitizedChildren(clone, node)
		return clone
	}
}

func appendSanitizedChildren(parent *html.Node, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		clone := cloneSanitizedHTML(child)
		if clone != nil {
			parent.AppendChild(clone)
		}
	}
}

func isRemovedHTMLTag(tag string) bool {
	return strings.EqualFold(tag, "script") || strings.EqualFold(tag, "style") || strings.EqualFold(tag, "link")
}

func filterAllowedHTMLAttributes(attributes []html.Attribute) []html.Attribute {
	if len(attributes) == 0 {
		return nil
	}

	filtered := make([]html.Attribute, 0, len(attributes))
	for _, attribute := range attributes {
		if isAllowedHTMLAttribute(attribute.Key) {
			filtered = append(filtered, attribute)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func isAllowedHTMLAttribute(key string) bool {
	return strings.EqualFold(key, "href") ||
		strings.EqualFold(key, "src") ||
		strings.EqualFold(key, "alt") ||
		strings.EqualFold(key, "title")
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
