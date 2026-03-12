package config

import (
	"errors"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	DefaultAccount string                   `toml:"default_account"`
	Accounts       map[string]AccountConfig `toml:"accounts"`
}

type AccountConfig struct {
	Provider string         `toml:"provider"`
	Auth     AuthConfig     `toml:"auth"`
	IMAP     IMAPConfig     `toml:"imap"`
	Defaults DefaultOptions `toml:"defaults"`
}

type AuthConfig struct {
	Username string `toml:"username"`
	Password string `toml:"password"`
}

type IMAPConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
	TLS  bool   `toml:"tls"`
}

type DefaultOptions struct {
	Mailbox  string `toml:"mailbox"`
	PageSize int    `toml:"page_size"`
	Format   string `toml:"format"`
}

func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func ResolveAccount(cfg Config, alias string) (string, AccountConfig, error) {
	accountName := alias
	if accountName == "" {
		accountName = cfg.DefaultAccount
	}

	if accountName == "" {
		return "", AccountConfig{}, errors.New("default_account is not configured")
	}

	account, ok := cfg.Accounts[accountName]
	if !ok {
		return "", AccountConfig{}, fmt.Errorf("account %q not found", accountName)
	}

	return accountName, account, nil
}
