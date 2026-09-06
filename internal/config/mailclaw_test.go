package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMailClawConfigReuseAndEnvironment(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("MAILCLAW_CONFIG", "")
	t.Setenv("MAILCLAW_HOST", "")
	t.Setenv("MAILCLAW_API_TOKEN", "")
	path, err := MailClawPath("")
	if err != nil || path != filepath.Join(os.Getenv("HOME"), ".mailclaw", "config.json") {
		t.Fatalf("default path=%s err=%v", path, err)
	}
	settings := MailClawSettings{Host: "https://mail.example.com", APIToken: "secret"}
	if err := SaveMailClaw(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := ResolveMailClaw(path)
	if err != nil || loaded != settings {
		t.Fatal("existing config not reused")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("config permissions are not 0600")
	}
	t.Setenv("MAILCLAW_HOST", "https://other.example.com")
	if _, err := ResolveMailClaw(path); err == nil {
		t.Fatal("host override reused saved token")
	}
	t.Setenv("MAILCLAW_API_TOKEN", "new-secret")
	loaded, err = ResolveMailClaw("nonexistent")
	if err != nil || loaded.Host != "https://other.example.com" || loaded.APIToken != "new-secret" {
		t.Fatal("paired env override failed")
	}
	t.Setenv("MAILCLAW_HOST", "")
	loaded, err = ResolveMailClaw(path)
	if err != nil || loaded.Host != settings.Host || loaded.APIToken != "new-secret" {
		t.Fatal("token-only rotation failed")
	}
	t.Setenv("MAILCLAW_CONFIG", "/tmp/env-config.json")
	if got, _ := MailClawPath(""); got != "/tmp/env-config.json" {
		t.Fatal("config env ignored")
	}
	if got, _ := MailClawPath("/tmp/explicit.json"); got != "/tmp/explicit.json" {
		t.Fatal("explicit path lost precedence")
	}
	if err := SaveMailClaw(path, MailClawSettings{Host: settings.Host, APIToken: "rotated"}); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadMailClaw(path)
	if err != nil || loaded.APIToken != "rotated" {
		t.Fatal("config replacement failed")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatal("temporary config files left behind")
	}
}

func TestMailClawConfigErrorsDoNotEchoValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, content := range []string{`{"api_token":BROKEN-secret}`, `{"host":"https://example.com","api_token":{"secret":"sensitive"}}`} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadMailClaw(path); err == nil || strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "sensitive") {
			t.Fatalf("unsafe error: %v", err)
		}
	}
}
