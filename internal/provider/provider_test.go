package provider

import (
	"testing"

	"github.com/rogeecn/email-cli/internal/config"
)

func TestNormalizeAppliesProviderPresets(t *testing.T) {
	account := config.AccountConfig{Provider: "qq"}
	resolved, err := Normalize(account)
	if err != nil {
		t.Fatalf("Normalize qq returned error: %v", err)
	}
	if resolved.IMAP.Host != "imap.qq.com" {
		t.Fatalf("Host = %q, want %q", resolved.IMAP.Host, "imap.qq.com")
	}
	if resolved.IMAP.Port != 993 {
		t.Fatalf("Port = %d, want 993", resolved.IMAP.Port)
	}
	if !resolved.IMAP.TLS {
		t.Fatalf("TLS = false, want true")
	}

	account = config.AccountConfig{Provider: "gmail"}
	resolved, err = Normalize(account)
	if err != nil {
		t.Fatalf("Normalize gmail returned error: %v", err)
	}
	if resolved.IMAP.Host != "imap.gmail.com" {
		t.Fatalf("Host = %q, want %q", resolved.IMAP.Host, "imap.gmail.com")
	}
	if resolved.IMAP.Port != 993 {
		t.Fatalf("Port = %d, want 993", resolved.IMAP.Port)
	}
	if !resolved.IMAP.TLS {
		t.Fatalf("TLS = false, want true")
	}
}

func TestNormalizeSelfhostRequiresExplicitSettings(t *testing.T) {
	_, err := Normalize(config.AccountConfig{Provider: "selfhost"})
	if err == nil {
		t.Fatalf("expected selfhost without IMAP settings to error")
	}

	resolved, err := Normalize(config.AccountConfig{
		Provider: "selfhost",
		IMAP: config.IMAPConfig{
			Host: "mail.example.com",
			Port: 993,
			TLS:  true,
		},
	})
	if err != nil {
		t.Fatalf("Normalize selfhost returned error: %v", err)
	}
	if resolved.IMAP.Host != "mail.example.com" {
		t.Fatalf("Host = %q, want %q", resolved.IMAP.Host, "mail.example.com")
	}
}
