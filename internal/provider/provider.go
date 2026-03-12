package provider

import (
	"fmt"

	"github.com/rogeecn/email-cli/internal/config"
)

func Normalize(account config.AccountConfig) (config.AccountConfig, error) {
	resolved := account

	switch account.Provider {
	case "qq":
		if resolved.IMAP.Host == "" {
			resolved.IMAP.Host = "imap.qq.com"
		}
		if resolved.IMAP.Port == 0 {
			resolved.IMAP.Port = 993
		}
		resolved.IMAP.TLS = true
	case "gmail":
		if resolved.IMAP.Host == "" {
			resolved.IMAP.Host = "imap.gmail.com"
		}
		if resolved.IMAP.Port == 0 {
			resolved.IMAP.Port = 993
		}
		resolved.IMAP.TLS = true
	case "selfhost":
		if resolved.IMAP.Host == "" || resolved.IMAP.Port == 0 {
			return config.AccountConfig{}, fmt.Errorf("selfhost account requires explicit imap.host and imap.port")
		}
	case "":
		return config.AccountConfig{}, fmt.Errorf("provider is required")
	default:
		return config.AccountConfig{}, fmt.Errorf("unsupported provider %q", account.Provider)
	}

	return resolved, nil
}
