# Email CLI Design

## Goal

Build a read-only email CLI tool named `email` for fetching mail from configured accounts. The first version supports `QQ`, `Gmail`, and `SelfHost`, uses `IMAP` only, and provides both message list and single-message detail views through one default command.

## Product Scope

### In Scope

- Fetch email list from a configured account
- Fetch a single email by `IMAP UID`
- Support `QQ`, `Gmail`, and `SelfHost`
- Use `IMAP` as the only backend protocol in v1
- Read credentials and defaults from `~/.config/email-cli/config.toml`
- Support `plain`, `json`, and `yaml` output
- Use a default account when no account flag is provided

### Out of Scope

- Sending mail
- Deleting, moving, or updating mail state
- Attachment download
- Complex search filters beyond mailbox, limit, and uid
- Provider-specific APIs such as Gmail API
- OAuth in v1

## CLI Experience

The CLI has a single default action: list mail.

### Command Shape

- `email`
- `email -A personal`
- `email -A personal --uid 12345`
- `email -A personal --mailbox INBOX --limit 50 --format json`

### CLI Flags

- `-A, --account`: target account alias from config
- `--uid`: switch from list mode to detail mode
- `--mailbox`: override default mailbox, default is `INBOX`
- `--limit`: override page size, default is `20`
- `--format`: `plain`, `json`, or `yaml`

### Behavioral Rules

- No arguments uses `default_account`
- No `--uid` means list mode
- With `--uid` means detail mode
- `provider` is not exposed in CLI flags and is resolved from account config

## Configuration Design

### Config Path

- `~/.config/email-cli/config.toml`

### Structure

Configuration is account-centric rather than provider-centric. Users operate on accounts, and each account references one provider.

### Required Top-Level Fields

- `default_account`
- `accounts`

### Account Fields

Each account may include:

- `provider`
- `auth.username`
- `auth.password`
- `imap.host`
- `imap.port`
- `imap.tls`
- `defaults.mailbox`
- `defaults.page_size`
- `defaults.format`

### Provider Rules

- `qq`: preset can fill default `imap.host`, `imap.port`, `imap.tls`
- `gmail`: preset can fill default `imap.host`, `imap.port`, `imap.tls`
- `selfhost`: must explicitly provide `imap` connection settings

### Example Shape

```toml
default_account = "personal"

[accounts.personal]
provider = "qq"

[accounts.personal.auth]
username = "user@example.com"
password = "app-password"

[accounts.personal.defaults]
mailbox = "INBOX"
page_size = 20
format = "plain"
```

## Data Model

### MailSummary

Used for list mode:

- `uid`
- `date`
- `from`
- `to`
- `subject`
- `seen`

### MailDetail

Used for detail mode:

- `uid`
- `date`
- `from`
- `to`
- `cc`
- `bcc`
- `subject`
- `seen`
- `flags`
- `text_body`
- `html_body`
- `attachments`
- `headers`

## Architecture

The system uses a unified IMAP core with provider presets.

### Modules

- `cmd`: parse CLI flags and start the app
- `internal/app`: application orchestration
- `internal/config`: config loading and validation
- `internal/provider`: provider presets and normalization
- `internal/imap`: IMAP connection and fetch operations
- `internal/mail`: mail parsing and normalization
- `internal/output`: output rendering for `plain`, `json`, `yaml`

### Data Flow

1. Parse CLI arguments
2. Load config from `~/.config/email-cli/config.toml`
3. Resolve target account from `-A/--account` or `default_account`
4. Normalize account IMAP settings through provider presets
5. Connect to IMAP and select mailbox
6. If `--uid` is absent, fetch recent message summaries
7. If `--uid` is present, fetch one full message by UID
8. Parse message data into unified models
9. Render output in requested format

## Output Design

### plain

Human-oriented terminal layout.

- List mode shows aligned rows or a compact table
- Detail mode shows sections for metadata, flags, body, and attachments
- `plain` means terminal formatting, not MIME `text/plain`

### json

Machine-oriented stable structure for automation and scripts.

### yaml

Human-readable structured output for inspection and debugging.

## Error Handling

The CLI should return readable and specific errors.

### Expected Error Cases

- Config file missing
- `default_account` missing or invalid
- Requested account not found
- Required IMAP settings missing for `selfhost`
- Authentication failure
- Mailbox not found
- UID not found
- Malformed message content

### Error Principles

- Print errors to `stderr`
- Preserve actionable context in messages
- Avoid crashing on partially malformed messages when detail output can still be partially shown

## V1 Defaults

- Default mailbox: `INBOX`
- Default page size: `20`
- Default format: `plain`
- Default command behavior: list recent mail from `default_account`

## Future Extensions

- Search filters such as sender, subject, and date range
- Attachment download
- More provider presets such as Outlook and iCloud
- OAuth or token-based authentication
- Better pagination semantics
