package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileParsesDefaultAccountAndNestedFields(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.toml")

	content := `default_account = "personal"

[accounts.personal]
provider = "qq"

[accounts.personal.auth]
username = "user@example.com"
password = "secret"

[accounts.personal.defaults]
mailbox = "INBOX"
page_size = 20
format = "plain"
`

	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	if cfg.DefaultAccount != "personal" {
		t.Fatalf("DefaultAccount = %q, want %q", cfg.DefaultAccount, "personal")
	}

	account, ok := cfg.Accounts["personal"]
	if !ok {
		t.Fatalf("expected personal account to be loaded")
	}

	if account.Provider != "qq" {
		t.Fatalf("Provider = %q, want %q", account.Provider, "qq")
	}

	if account.Auth.Username != "user@example.com" {
		t.Fatalf("Auth.Username = %q, want %q", account.Auth.Username, "user@example.com")
	}

	if account.Auth.Password != "secret" {
		t.Fatalf("Auth.Password = %q, want %q", account.Auth.Password, "secret")
	}

	if account.Defaults.Mailbox != "INBOX" {
		t.Fatalf("Defaults.Mailbox = %q, want %q", account.Defaults.Mailbox, "INBOX")
	}

	if account.Defaults.PageSize != 20 {
		t.Fatalf("Defaults.PageSize = %d, want 20", account.Defaults.PageSize)
	}

	if account.Defaults.Format != "plain" {
		t.Fatalf("Defaults.Format = %q, want %q", account.Defaults.Format, "plain")
	}
}

func TestResolveAccountUsesDefaultAndAlias(t *testing.T) {
	cfg := Config{
		DefaultAccount: "personal",
		Accounts: map[string]AccountConfig{
			"personal": {Provider: "qq"},
			"work":     {Provider: "gmail"},
		},
	}

	accountName, account, err := ResolveAccount(cfg, "")
	if err != nil {
		t.Fatalf("ResolveAccount default returned error: %v", err)
	}

	if accountName != "personal" {
		t.Fatalf("accountName = %q, want %q", accountName, "personal")
	}

	if account.Provider != "qq" {
		t.Fatalf("Provider = %q, want %q", account.Provider, "qq")
	}

	accountName, account, err = ResolveAccount(cfg, "work")
	if err != nil {
		t.Fatalf("ResolveAccount alias returned error: %v", err)
	}

	if accountName != "work" {
		t.Fatalf("accountName = %q, want %q", accountName, "work")
	}

	if account.Provider != "gmail" {
		t.Fatalf("Provider = %q, want %q", account.Provider, "gmail")
	}
}

func TestResolveAccountErrorsForMissingDefault(t *testing.T) {
	cfg := Config{}

	_, _, err := ResolveAccount(cfg, "")
	if err == nil {
		t.Fatalf("expected error for missing default account")
	}
}
