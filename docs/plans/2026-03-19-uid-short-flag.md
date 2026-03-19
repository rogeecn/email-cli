# UID Short Flag Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `-u` as a short alias for `--uid` without changing existing detail-mode behavior.

**Architecture:** The CLI already maps multiple flag names to the same option field for `account` and `config`. This change follows the same pattern by registering `u` against the existing `UID` field, then updates tests and docs so parsing, help output, and examples stay aligned.

**Tech Stack:** Go, standard library `flag` package, Go test framework, Markdown docs

---

### Task 1: Add the short UID alias

**Files:**
- Modify: `internal/cli/cli.go`
- Test: `cmd/email/main_test.go`

**Step 1: Write the failing parse test**

Add a test case in `cmd/email/main_test.go` that calls `cli.ParseFlags([]string{"-u", "42"})` and asserts `options.UID == 42`.

**Step 2: Run the targeted parse test and verify it fails**

Run: `go test ./cmd/email -run TestParseFlagsReadsAliasUIDConfigPathAndDebug -v`
Expected: FAIL because `-u` is not registered yet.

**Step 3: Write the minimal implementation**

Register `flagSet.UintVar(&options.UID, "u", 0, "message UID for detail view")` in `internal/cli/cli.go` next to the existing `--uid` registration.

**Step 4: Run the targeted parse test and verify it passes**

Run: `go test ./cmd/email -run TestParseFlagsReadsAliasUIDConfigPathAndDebug -v`
Expected: PASS.

### Task 2: Cover flag registration explicitly

**Files:**
- Modify: `cmd/email/main_test.go`

**Step 1: Write the failing registration assertion**

Extend the existing flag registration test to assert `flagSet.Lookup("u") != nil`.

**Step 2: Run the targeted registration test and verify it fails**

Run: `go test ./cmd/email -run TestNewFlagSetSupportsShortAliasConfigAndDebugFlag -v`
Expected: FAIL before the alias is added, PASS after Task 1 Step 3.

**Step 3: Keep the minimal implementation only**

Do not refactor flag registration. Reuse the alias added in `internal/cli/cli.go`.

**Step 4: Re-run the registration test**

Run: `go test ./cmd/email -run TestNewFlagSetSupportsShortAliasConfigAndDebugFlag -v`
Expected: PASS.

### Task 3: Update help text and README examples

**Files:**
- Modify: `internal/cli/cli.go`
- Modify: `cmd/email/help_test.go`
- Modify: `README.md`

**Step 1: Write the failing help expectation**

Update `cmd/email/help_test.go` so at least one usage example expects `-u 12345` to appear in help output.

**Step 2: Run the targeted help test and verify it fails**

Run: `go test ./cmd/email -run TestFlagSetUsageIncludesExamplesAndConfigPath -v`
Expected: FAIL until the usage text is updated.

**Step 3: Write the minimal doc changes**

Update one CLI usage example in `internal/cli/cli.go` and one or more examples in `README.md` to show `-u` while still keeping at least one `--uid` example so both forms are documented.

**Step 4: Re-run the help test**

Run: `go test ./cmd/email -run TestFlagSetUsageIncludesExamplesAndConfigPath -v`
Expected: PASS.

### Task 4: Verify the full change

**Files:**
- No new files required

**Step 1: Run the package tests**

Run: `go test ./cmd/email -v`
Expected: PASS.

**Step 2: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 3: Check help output manually if desired**

Run: `go run . --help`
Expected: help text lists `-u` support and shows a short-form UID example.
