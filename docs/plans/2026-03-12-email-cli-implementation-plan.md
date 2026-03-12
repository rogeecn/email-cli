# Email CLI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a read-only `email` CLI that fetches message lists and single-message details from configured `QQ`, `Gmail`, and `SelfHost` accounts over IMAP.

**Architecture:** The tool is account-centric. CLI input resolves a configured account, provider presets normalize IMAP settings, a single IMAP layer fetches mail data, and a renderer outputs `plain`, `json`, or `yaml`. The `--uid` flag switches from list mode to detail mode without introducing a second top-level command.

**Tech Stack:** Go, Cobra or stdlib flag parsing, TOML config parser, IMAP client library, JSON and YAML serializers, Go testing package

---

### Task 1: Initialize repository structure

**Files:**
- Create: `cmd/email/main.go`
- Create: `internal/app/app.go`
- Create: `internal/config/config.go`
- Create: `internal/provider/provider.go`
- Create: `internal/imap/client.go`
- Create: `internal/mail/model.go`
- Create: `internal/output/render.go`
- Create: `go.mod`

**Step 1: Create the folder layout**

Create the `cmd` and `internal` package structure so the CLI, config, provider, IMAP, mail, and output layers are separated from the start.

**Step 2: Initialize the Go module**

Run: `go mod init <module-name>`
Expected: a new `go.mod` file is created.

**Step 3: Add a minimal CLI entrypoint**

Write a `main.go` that parses flags and calls the app entrypoint.

**Step 4: Add a minimal app runner**

Implement a placeholder app function that returns `nil` so the binary can build before functionality is added.

**Step 5: Verify the binary builds**

Run: `go build ./...`
Expected: PASS with no compile errors.

### Task 2: Define config data structures

**Files:**
- Modify: `internal/config/config.go`
- Create: `internal/config/types.go`
- Test: `internal/config/config_test.go`

**Step 1: Write the failing config shape tests**

Add tests for:
- parsing `default_account`
- parsing `accounts.<name>`
- parsing nested `auth`, `imap`, and `defaults`

**Step 2: Run config tests and verify they fail**

Run: `go test ./internal/config -v`
Expected: FAIL because parsing and types are not implemented yet.

**Step 3: Define config structs**

Add structs for:
- `Config`
- `AccountConfig`
- `AuthConfig`
- `IMAPConfig`
- `DefaultOptions`

**Step 4: Implement TOML loading**

Load `~/.config/email-cli/config.toml` into the config structs.

**Step 5: Re-run config tests**

Run: `go test ./internal/config -v`
Expected: PASS.

### Task 3: Validate configuration and resolve accounts

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing validation tests**

Cover:
- missing config file
- missing `default_account`
- invalid `default_account`
- missing account when `-A` is provided
- missing IMAP fields for `selfhost`

**Step 2: Run validation tests**

Run: `go test ./internal/config -v`
Expected: FAIL with missing validation logic.

**Step 3: Implement validation and account resolution**

Add functions that:
- validate top-level config
- resolve a target account from CLI input or default
- return readable errors

**Step 4: Re-run validation tests**

Run: `go test ./internal/config -v`
Expected: PASS.

### Task 4: Implement provider presets

**Files:**
- Modify: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`

**Step 1: Write failing preset tests**

Cover:
- `qq` default host, port, and TLS
- `gmail` default host, port, and TLS
- `selfhost` preserving explicit settings

**Step 2: Run preset tests**

Run: `go test ./internal/provider -v`
Expected: FAIL.

**Step 3: Implement provider normalization**

Add logic that maps an account config into a complete IMAP connection config.

**Step 4: Re-run preset tests**

Run: `go test ./internal/provider -v`
Expected: PASS.

### Task 5: Implement CLI option model

**Files:**
- Modify: `cmd/email/main.go`
- Modify: `internal/app/app.go`
- Create: `internal/app/options.go`
- Test: `internal/app/options_test.go`

**Step 1: Write failing option resolution tests**

Cover:
- default list mode
- account override through `-A`
- detail mode through `--uid`
- CLI `--limit`, `--mailbox`, and `--format` overriding config defaults

**Step 2: Run option tests**

Run: `go test ./internal/app -v`
Expected: FAIL.

**Step 3: Implement option parsing and merge rules**

Add an options struct and merge precedence:
- CLI flags
- account defaults
- built-in defaults

**Step 4: Re-run option tests**

Run: `go test ./internal/app -v`
Expected: PASS.

### Task 6: Define mail models and parsing contract

**Files:**
- Modify: `internal/mail/model.go`
- Create: `internal/mail/parser.go`
- Create: `internal/mail/parser_test.go`

**Step 1: Write failing parser tests**

Cover:
- summary model fields
- detail model fields
- address header normalization
- body extraction placeholders
- attachment metadata placeholders

**Step 2: Run mail tests**

Run: `go test ./internal/mail -v`
Expected: FAIL.

**Step 3: Implement unified mail models**

Define:
- `MailSummary`
- `MailDetail`
- attachment metadata types

**Step 4: Implement parser scaffolding**

Add parsing helpers with fake or fixture inputs so downstream layers can be developed safely.

**Step 5: Re-run mail tests**

Run: `go test ./internal/mail -v`
Expected: PASS.

### Task 7: Implement IMAP client abstraction

**Files:**
- Modify: `internal/imap/client.go`
- Create: `internal/imap/types.go`
- Create: `internal/imap/client_test.go`

**Step 1: Write failing client abstraction tests**

Cover:
- connect/login/select mailbox flow
- fetch summary list contract
- fetch by UID contract
- error propagation

**Step 2: Run IMAP tests**

Run: `go test ./internal/imap -v`
Expected: FAIL.

**Step 3: Define a narrow IMAP interface**

Expose methods such as:
- `Connect`
- `ListRecent`
- `GetByUID`
- `Close`

**Step 4: Implement a concrete client using the chosen IMAP library**

Keep the library-specific logic isolated in this package.

**Step 5: Re-run IMAP tests**

Run: `go test ./internal/imap -v`
Expected: PASS.

### Task 8: Implement list mode orchestration

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/list.go`
- Create: `internal/app/list_test.go`

**Step 1: Write failing list orchestration tests**

Cover:
- resolving account config
- calling IMAP list fetch
- returning `MailSummary` results
- passing mailbox and limit correctly

**Step 2: Run list tests**

Run: `go test ./internal/app -v`
Expected: FAIL.

**Step 3: Implement list flow**

Add the orchestration that builds list requests and returns summary models.

**Step 4: Re-run list tests**

Run: `go test ./internal/app -v`
Expected: PASS.

### Task 9: Implement detail mode orchestration

**Files:**
- Modify: `internal/app/app.go`
- Create: `internal/app/detail.go`
- Create: `internal/app/detail_test.go`

**Step 1: Write failing detail orchestration tests**

Cover:
- switching to detail mode when `--uid` is set
- calling IMAP fetch by UID
- returning full `MailDetail`
- surfacing `UID not found` cleanly

**Step 2: Run detail tests**

Run: `go test ./internal/app -v`
Expected: FAIL.

**Step 3: Implement detail flow**

Add the orchestration that fetches, parses, and returns one detailed message.

**Step 4: Re-run detail tests**

Run: `go test ./internal/app -v`
Expected: PASS.

### Task 10: Implement output rendering

**Files:**
- Modify: `internal/output/render.go`
- Create: `internal/output/plain.go`
- Create: `internal/output/json.go`
- Create: `internal/output/yaml.go`
- Create: `internal/output/render_test.go`

**Step 1: Write failing render tests**

Cover:
- summary rendering in `plain`
- detail rendering in `plain`
- valid JSON serialization
- valid YAML serialization

**Step 2: Run output tests**

Run: `go test ./internal/output -v`
Expected: FAIL.

**Step 3: Implement renderers**

Add rendering paths for summary lists and detail objects.

**Step 4: Re-run output tests**

Run: `go test ./internal/output -v`
Expected: PASS.

### Task 11: Wire the full application

**Files:**
- Modify: `cmd/email/main.go`
- Modify: `internal/app/app.go`
- Modify: `internal/output/render.go`

**Step 1: Write a failing top-level flow test**

Cover:
- default command goes to list mode
- `--uid` goes to detail mode
- render output is written to stdout
- errors go to stderr

**Step 2: Run top-level tests**

Run: `go test ./internal/app -v`
Expected: FAIL.

**Step 3: Implement end-to-end wiring**

Connect CLI input, config loading, provider normalization, IMAP calls, parser, and renderer.

**Step 4: Re-run top-level tests**

Run: `go test ./internal/app -v`
Expected: PASS.

### Task 12: Add example config and usage help

**Files:**
- Create: `examples/config.toml`
- Modify: `cmd/email/main.go`
- Create: `README.md` only if explicitly requested later

**Step 1: Add a minimal config example**

Include one example each for:
- `qq`
- `gmail`
- `selfhost`

**Step 2: Improve CLI help text**

Show examples for:
- `email`
- `email -A personal`
- `email -A personal --uid 12345`
- `email -A personal --format yaml`

**Step 3: Build and smoke test the CLI**

Run: `go build ./...`
Expected: PASS.

### Task 13: Run the verification suite

**Files:**
- No new files required unless missing tests force small additions

**Step 1: Run targeted tests**

Run:
- `go test ./internal/config -v`
- `go test ./internal/provider -v`
- `go test ./internal/mail -v`
- `go test ./internal/imap -v`
- `go test ./internal/output -v`
- `go test ./internal/app -v`

Expected: PASS.

**Step 2: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 3: Run a final build**

Run: `go build ./...`
Expected: PASS.
