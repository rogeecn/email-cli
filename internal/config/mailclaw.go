package config

// MailClawConfig is stored directly in the account's TOML table.
type MailClawConfig struct {
	Host     string `toml:"host"`
	APIToken string `toml:"api_token"`
}
