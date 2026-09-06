package output

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderAPIPreservesSchema(t *testing.T) {
	data := json.RawMessage(`{"id":"000123","text_body":"first\nsecond","size":9007199254740993,"attachment":null}`)
	for _, format := range []string{"plain", "json", "yaml"} {
		result, err := RenderAPI(data, format)
		if err != nil || !strings.Contains(string(result), "9007199254740993") {
			t.Fatalf("%s: %s %v", format, result, err)
		}
		var value map[string]any
		if format == "json" {
			err = json.Unmarshal(result, &value)
		} else {
			err = yaml.Unmarshal(result, &value)
		}
		if err != nil || value["id"] != "000123" || value["text_body"] != "first\nsecond" {
			t.Fatalf("%s changed schema: %v %v", format, value, err)
		}
	}
	if _, err := RenderAPI(data, "xml"); err == nil {
		t.Fatal("unknown format accepted")
	}
}
