package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mailclaw"
	"github.com/rogeecn/email-cli/internal/output"
)

type stringList []string

func (v *stringList) String() string { return strings.Join(*v, ",") }
func (v *stringList) Set(s string) error {
	*v = append(*v, s)
	return nil
}

// parseCommandFlags permits flags before or after IDs, unlike flag.Parse's default.
func parseCommandFlags(fs *flag.FlagSet, args []string) error {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name, _, assigned := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil || assigned {
			continue
		}
		boolFlag, ok := f.Value.(interface{ IsBoolFlag() bool })
		if ok && boolFlag.IsBoolFlag() {
			continue
		}
		if i+1 == len(args) {
			return fmt.Errorf("flag needs an argument: %s", arg)
		}
		i++
		flags = append(flags, args[i])
	}
	return fs.Parse(append(append(flags, "--"), positional...))
}

func mailClawHelp(w io.Writer) {
	fmt.Fprintf(w, `Usage: %s mailclaw <command> [flags]

Commands:
  list                         List metadata (one page)
  export                       Export full content (one page)
  get <id>                     Read an email by string ID
  send                         Send email (--from, --to, --subject, --text/--html)
  delete <id> --yes             Permanently delete an email and its attachments
  attachments <id>              List attachment metadata
  download <id> <attachment-id> --output <path>
  health                       Check the API
  config path|show              Locate configuration / show host and token presence
  config set --host <url>       Save host and MAILCLAW_API_TOKEN (never echoes token)

Common flags (after command, before or after IDs):
  --mailclaw-config <path>      Existing MailClaw JSON config (~/.mailclaw/config.json)
  -A, --account <alias>         MailClaw account from email-cli TOML config
  -c, --config <path>           TOML path (requires --account)
  --format plain|json|yaml      Output format (plain is readable key/value text)

list/export: --from --to --q --after --before --limit (1..100) --offset (>=0)
Dates: Unix seconds, YYYY-MM-DD, or RFC3339. Filters use inclusive bounds.
export: --output <path> writes one page without overwriting an existing file.
send: repeat --to/--cc/--bcc/--reply-to; --text-file/--html-file for UTF-8 bodies.
      --header key=value, --tag name=value, --scheduled-at <RFC3339> are optional.
Environment: MAILCLAW_CONFIG; MAILCLAW_HOST + MAILCLAW_API_TOKEN (must be paired).
No configuration or production mail is changed by read commands.
`, BinaryName)
}

func runMailClaw(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		mailClawHelp(stdout)
		return 0
	}
	if err := executeMailClaw(ctx, args, stdout, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func executeMailClaw(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	command := args[0]
	args = args[1:]
	if command == "config" {
		if len(args) == 0 {
			return errors.New("mailclaw config requires path, show, or set")
		}
		command += " " + args[0]
		args = args[1:]
	}
	fs := flag.NewFlagSet("mailclaw "+command, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { mailClawHelp(stderr); fs.PrintDefaults() }
	var account, tomlPath, jsonPath, format, from, toFilter, query, after, before, destination, host string
	var subject, text, html, textFile, htmlFile, scheduled string
	var recipients, cc, bcc, replyTo, headers, tags stringList
	var limit, offset int
	var yes bool
	fs.StringVar(&account, "account", "", "TOML account alias")
	fs.StringVar(&account, "A", "", "TOML account alias")
	fs.StringVar(&tomlPath, "config", "", "TOML configuration path")
	fs.StringVar(&tomlPath, "c", "", "TOML configuration path")
	fs.StringVar(&jsonPath, "mailclaw-config", "", "MailClaw JSON configuration path")
	fs.StringVar(&format, "format", "", "plain, json, or yaml")
	arity := 0
	switch command {
	case "list", "export":
		fs.StringVar(&from, "from", "", "sender filter")
		fs.StringVar(&toFilter, "to", "", "recipient filter")
		fs.StringVar(&query, "q", "", "full-text search")
		fs.StringVar(&after, "after", "", "inclusive lower timestamp")
		fs.StringVar(&before, "before", "", "inclusive upper timestamp")
		fs.IntVar(&limit, "limit", 0, "page size (1..100; default 20)")
		fs.IntVar(&offset, "offset", 0, "messages to skip")
		if command == "export" {
			fs.StringVar(&destination, "output", "", "new output file")
			fs.StringVar(&destination, "o", "", "new output file")
		}
	case "get", "attachments":
		arity = 1
	case "delete":
		arity = 1
		fs.BoolVar(&yes, "yes", false, "confirm permanent deletion")
	case "download":
		arity = 2
		fs.StringVar(&destination, "output", "", "required output file")
		fs.StringVar(&destination, "o", "", "required output file")
	case "send":
		fs.StringVar(&from, "from", "", "sender email address")
		fs.Var(&recipients, "to", "recipient (repeatable)")
		fs.Var(&cc, "cc", "CC recipient (repeatable)")
		fs.Var(&bcc, "bcc", "BCC recipient (repeatable)")
		fs.Var(&replyTo, "reply-to", "reply address (repeatable)")
		fs.StringVar(&subject, "subject", "", "subject")
		fs.StringVar(&text, "text", "", "plain text body")
		fs.StringVar(&html, "html", "", "HTML body")
		fs.StringVar(&textFile, "text-file", "", "plain text body file")
		fs.StringVar(&htmlFile, "html-file", "", "HTML body file")
		fs.Var(&headers, "header", "header key=value (repeatable)")
		fs.Var(&tags, "tag", "tag name=value (repeatable)")
		fs.StringVar(&scheduled, "scheduled-at", "", "scheduled send time (RFC3339)")
	case "config set":
		fs.StringVar(&host, "host", "", "HTTPS API base URL")
	case "health", "config path", "config show":
	default:
		return fmt.Errorf("unknown MailClaw command %q; use email-cli mailclaw --help", command)
	}
	if err := parseCommandFlags(fs, args); err != nil {
		return err
	}
	explicitLimit := false
	fs.Visit(func(f *flag.Flag) { explicitLimit = explicitLimit || f.Name == "limit" })
	if fs.NArg() != arity {
		return fmt.Errorf("mailclaw %s requires %d positional ID(s)", command, arity)
	}
	if account != "" && jsonPath != "" {
		return errors.New("choose --account or --mailclaw-config, not both")
	}
	if tomlPath != "" && account == "" {
		return errors.New("--config requires --account; use --mailclaw-config for JSON")
	}
	if account != "" {
		if tomlPath == "" {
			tomlPath = config.DefaultPath()
		}
		cfg, err := config.LoadFile(tomlPath)
		if err != nil {
			return err
		}
		_, selected, err := config.ResolveAccount(cfg, account)
		if err != nil {
			return err
		}
		if selected.Provider != "mailclaw" {
			return errors.New("mailclaw commands require a provider = \"mailclaw\" account; IMAP remains read-only")
		}
		jsonPath = selected.MailClaw.Config
		if format == "" {
			format = selected.Defaults.Format
		}
		if !explicitLimit {
			limit = selected.Defaults.PageSize
		}
	}
	if format == "" {
		format = "plain"
	}
	if _, err := output.RenderAPI(json.RawMessage(`{}`), format); err != nil {
		return err
	}
	path, err := config.MailClawPath(jsonPath)
	if err != nil {
		return err
	}
	if command == "config path" {
		return writeAPI(stdout, map[string]string{"path": path}, format)
	}
	if command == "config show" || command == "config set" {
		var settings config.MailClawSettings
		if command == "config show" {
			settings, err = config.ResolveMailClaw(path)
		} else {
			settings = config.MailClawSettings{Host: host, APIToken: os.Getenv("MAILCLAW_API_TOKEN")}
		}
		if err != nil {
			return err
		}
		if _, err := mailclaw.New(settings.Host, settings.APIToken); err != nil {
			return err
		}
		if command == "config set" {
			if err := config.SaveMailClaw(path, settings); err != nil {
				return err
			}
		}
		return writeAPI(stdout, map[string]any{"path": path, "host": settings.Host, "api_token_configured": true}, format)
	}
	method, endpoint := http.MethodGet, "/api/emails"
	params := url.Values{}
	var payload any
	if arity > 0 {
		endpoint, err = mailclaw.EmailPath(fs.Arg(0))
		if err != nil {
			return err
		}
	}
	switch command {
	case "list", "export":
		if limit == 0 && !explicitLimit {
			limit = 20
		}
		if limit < 1 || limit > 100 || offset < 0 {
			return errors.New("mailclaw: --limit must be 1..100 and --offset must be non-negative")
		}
		params.Set("limit", strconv.Itoa(limit))
		params.Set("offset", strconv.Itoa(offset))
		for key, value := range map[string]string{"from": from, "to": toFilter, "q": query} {
			if value != "" {
				params.Set(key, value)
			}
		}
		for key, value := range map[string]string{"after": after, "before": before} {
			if value != "" {
				timestamp, err := parseMailClawTime(value)
				if err != nil {
					return fmt.Errorf("mailclaw: --%s must be Unix seconds, YYYY-MM-DD, or RFC3339", key)
				}
				params.Set(key, strconv.FormatInt(timestamp, 10))
			}
		}
		if params.Has("after") && params.Has("before") {
			afterTime, _ := strconv.ParseInt(params.Get("after"), 10, 64)
			beforeTime, _ := strconv.ParseInt(params.Get("before"), 10, 64)
			if afterTime > beforeTime {
				return errors.New("mailclaw: --after must not exceed --before")
			}
		}
		if command == "export" {
			endpoint += "/export"
		}
	case "delete":
		if !yes {
			return errors.New("mailclaw: deletion is permanent; pass --yes to confirm")
		}
		method = http.MethodDelete
	case "attachments":
		endpoint += "/attachments"
	case "download":
		if destination == "" {
			return errors.New("mailclaw: download requires --output; remote filenames are not used")
		}
		if _, err := mailclaw.EmailPath(fs.Arg(1)); err != nil {
			return err
		}
	case "send":
		for _, addresses := range [][]string{{from}, recipients, cc, bcc, replyTo} {
			for _, address := range addresses {
				if _, err := mail.ParseAddress(address); err != nil || strings.ContainsAny(address, "\r\n") {
					return errors.New("mailclaw: invalid email address; repeat recipient flags for multiple addresses")
				}
			}
		}
		if len(recipients) == 0 || strings.TrimSpace(subject) == "" || strings.ContainsAny(subject, "\r\n") {
			return errors.New("mailclaw: send requires --to and a non-empty single-line --subject")
		}
		text, err = readBody(text, textFile)
		if err != nil {
			return err
		}
		html, err = readBody(html, htmlFile)
		if err != nil {
			return err
		}
		if text == "" && html == "" {
			return errors.New("mailclaw: send requires a non-empty text or HTML body")
		}
		body := map[string]any{"from": from, "to": recipients, "subject": subject}
		for key, value := range map[string]string{"text": text, "html": html, "scheduled_at": scheduled} {
			if value != "" {
				body[key] = value
			}
		}
		for key, value := range map[string]stringList{"cc": cc, "bcc": bcc, "reply_to": replyTo} {
			if len(value) > 0 {
				body[key] = value
			}
		}
		if scheduled != "" {
			if _, err := time.Parse(time.RFC3339, scheduled); err != nil {
				return errors.New("mailclaw: --scheduled-at must be RFC3339")
			}
		}
		if len(headers) > 0 {
			values := map[string]string{}
			for _, header := range headers {
				key, value, ok := strings.Cut(header, "=")
				if !ok || !validHeaderName(key) || strings.ContainsAny(value, "\r\n") {
					return errors.New("mailclaw: --header requires a valid key=value without newlines")
				}
				values[key] = value
			}
			body["headers"] = values
		}
		if len(tags) > 0 {
			values := []map[string]string{}
			for _, tag := range tags {
				key, value, ok := strings.Cut(tag, "=")
				if !ok || key == "" || value == "" || strings.ContainsAny(tag, "\r\n") {
					return errors.New("mailclaw: --tag requires non-empty name=value")
				}
				values = append(values, map[string]string{"name": key, "value": value})
			}
			body["tags"] = values
		}
		method, endpoint, payload = http.MethodPost, "/api/emails/send", body
	case "health":
		endpoint = "/api/health"
	}
	settings, err := config.ResolveMailClaw(path)
	if err != nil {
		return err
	}
	client, err := mailclaw.New(settings.Host, settings.APIToken)
	if err != nil {
		return err
	}
	if command == "download" {
		n, err := client.Download(ctx, fs.Arg(0), fs.Arg(1), destination)
		if err != nil {
			return err
		}
		return writeAPI(stdout, map[string]any{"path": destination, "bytes": n}, format)
	}
	data, err := client.Do(ctx, method, endpoint, params, payload)
	if err != nil {
		return err
	}
	rendered, err := output.RenderAPI(data, format)
	if err != nil {
		return err
	}
	if destination != "" {
		return writeNewFile(destination, rendered)
	}
	_, err = stdout.Write(rendered)
	return err
}

func writeAPI(w io.Writer, value any, format string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	rendered, err := output.RenderAPI(data, format)
	if err != nil {
		return err
	}
	_, err = w.Write(rendered)
	return err
}

func writeNewFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func parseMailClawTime(value string) (int64, error) {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds < 0 || seconds > 253402300799 {
			return 0, errors.New("timestamp out of range")
		}
		return seconds, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil && parsed.Unix() >= 0 {
			return parsed.Unix(), nil
		}
	}
	return 0, errors.New("invalid timestamp")
}

func readBody(value, path string) (string, error) {
	if path != "" {
		if value != "" {
			return "", errors.New("mailclaw: choose inline body or body file, not both")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		value = string(data)
	}
	if !utf8.ValidString(value) {
		return "", errors.New("mailclaw: message bodies must be valid UTF-8")
	}
	return value, nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, c := range name {
		if c <= 32 || c >= 127 || strings.ContainsRune("()<>@,;:\\\"/[]?={}", c) {
			return false
		}
	}
	return true
}
