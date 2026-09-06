package output

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// RenderAPI preserves MailClaw's schema instead of coercing string IDs to IMAP UIDs.
// Plain output uses YAML's readable key/value notation, including multiline bodies.
func RenderAPI(data json.RawMessage, format string) ([]byte, error) {
	switch format {
	case "json":
		var result bytes.Buffer
		if err := json.Indent(&result, data, "", "  "); err != nil {
			return nil, err
		}
		result.WriteByte('\n')
		return result.Bytes(), nil
	case "plain", "yaml":
		var value any
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, err
		}
		return yaml.Marshal(value)
	default:
		return nil, fmt.Errorf("unsupported output format %q", format)
	}
}
