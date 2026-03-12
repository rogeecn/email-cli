# email-cli

A read-only email CLI for fetching messages from IMAP accounts.

Currently supported providers:
- `qq`
- `gmail`
- `selfhost`

Supported output formats:
- `plain`
- `json`
- `yaml`

## Features

- List recent messages from a configured account
- Show a full message by `IMAP UID`
- Use `default_account` when no account is specified
- Load configuration from a local config file
- Normalize provider presets for `qq` and `gmail`

## Scope

This tool only fetches email.

Not supported yet:
- Sending email
- Deleting or moving email
- Updating read state
- Downloading attachments
- Complex search filters
- OAuth-based authentication

## Quick Start

### 1. Prepare config

Copy `examples/config.toml` to your local config path:

```bash
mkdir -p ~/.config/email-cli
cp examples/config.toml ~/.config/email-cli/config.toml
```

You can also keep config anywhere and pass it explicitly with `-c, --config`.

Default config path:

```text
~/.config/email-cli/config.toml
```

You can also use `XDG_CONFIG_HOME`, which maps to:

```text
$XDG_CONFIG_HOME/email-cli/config.toml
```

### 2. Fill your credentials

Update the account entries in `~/.config/email-cli/config.toml`:
- `auth.username`
- `auth.password`
- for `selfhost`, also set `imap.host`, `imap.port`, `imap.tls`

For `qq` and `gmail`, use IMAP app passwords where required.

### 3. Run the CLI

Using the default account:

```bash
go run ./cmd/email
```

Using a named account:

```bash
go run ./cmd/email -A personal
```

Using a custom config path:

```bash
go run ./cmd/email -c ./config.toml -A personal
```

Show a single message by UID:

```bash
go run ./cmd/email -A personal --uid 12345
```

Render JSON:

```bash
go run ./cmd/email -A personal --format json
```

List the next page of messages:

```bash
go run ./cmd/email -A personal --offset 10 --limit 10
```

Print receive debug logs to `stderr`:

```bash
go run ./cmd/email -A personal --debug
```

## Command Reference

### List messages

```bash
email
email -A personal
email -A personal --mailbox INBOX --limit 20
email -A personal --offset 10 --limit 10
```

Default behavior:
- uses `default_account`
- uses default mailbox from config or `INBOX`
- uses default page size from config or `20`
- uses default offset `0`
- uses default output format from config or `plain`
- with `--debug`, writes receive diagnostics to `stderr`

### Show message detail

```bash
email --uid 12345
email -A personal --uid 12345
```

This prints message metadata and the full body.

## Output Formats

### `plain`

Human-friendly terminal output.

- list mode prints a summary header plus each email as a multi-line block with subject, id, sender, recipients, received time, and real attachment summary
- detail mode prints sections for metadata, body, attachments, and headers

### `json`

Structured output for scripts and automation.

### `yaml`

Structured output that is easier to inspect manually.

## Config Example

See `examples/config.toml` for a complete sample covering:
- `qq`
- `gmail`
- `selfhost`

## Development

Run tests:

```bash
go test ./...
```

Build:

```bash
go build ./...
```
