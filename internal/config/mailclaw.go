package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// MailClawConfig references the existing CLI configuration, without copying its secrets.
type MailClawConfig struct {
	Config string `toml:"config"`
}

type MailClawSettings struct {
	Host     string `json:"host"`
	APIToken string `json:"api_token"`
}

func MailClawPath(path string) (string, error) {
	if path == "" {
		path = os.Getenv("MAILCLAW_CONFIG")
	}
	if path == "" {
		path = "~/.mailclaw/config.json"
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return path, nil
}

func LoadMailClaw(path string) (MailClawSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MailClawSettings{}, err
	}
	var settings MailClawSettings
	if json.Unmarshal(data, &settings) != nil {
		// JSON type errors can quote the offending value, including a secret.
		return MailClawSettings{}, errors.New("mailclaw: invalid config JSON (expected host and api_token strings)")
	}
	return settings, nil
}

// ResolveMailClaw uses an explicit environment host only with its own token.
// This prevents an accidental host override from leaking a saved production token.
func ResolveMailClaw(path string) (MailClawSettings, error) {
	host, token := os.Getenv("MAILCLAW_HOST"), os.Getenv("MAILCLAW_API_TOKEN")
	if host != "" {
		if token == "" {
			return MailClawSettings{}, errors.New("mailclaw: MAILCLAW_HOST requires MAILCLAW_API_TOKEN")
		}
		return MailClawSettings{Host: host, APIToken: token}, nil
	}
	settings, err := LoadMailClaw(path)
	if err != nil {
		return MailClawSettings{}, err
	}
	if token != "" {
		settings.APIToken = token
	}
	return settings, nil
}

// SaveMailClaw atomically replaces the settings file with owner-only permissions.
func SaveMailClaw(path string, settings MailClawSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".mailclaw-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
