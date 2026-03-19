# UID Short Flag Design

## Goal

Add `-u` as a short alias for `--uid` so users can request a single message detail view with a shorter CLI form while keeping existing behavior unchanged.

## Scope

### In Scope

- Support `-u <uid>` anywhere `--uid <uid>` works today
- Keep `--uid` fully supported
- Update CLI help text and README examples to show the short alias exists
- Add tests for flag registration and parsing through the short alias

### Out of Scope

- Renaming or removing `--uid`
- Changing how detail mode is selected
- Changing output, validation, or message-fetch behavior
- Refactoring flag registration beyond what this alias requires

## CLI Behavior

### Command Shape

- `email-cli -u 12345`
- `email-cli -A personal -u 12345`
- `email-cli -A personal --uid 12345`

### Behavioral Rules

- `-u` and `--uid` map to the same `UID` option field
- Both forms continue to require a numeric value
- Presence of either flag switches the app from list mode to detail mode
- Existing examples and workflows using `--uid` remain valid

## Implementation Approach

The CLI already uses Go's standard `flag.FlagSet` and registers short aliases separately for `-A` and `-c`. The smallest and most consistent change is to register a second flag name, `u`, against the existing `options.UID` field.

This keeps parsing behavior aligned with the current implementation style, avoids unnecessary refactoring, and limits the blast radius to CLI parsing, tests, and documentation.

## Files Affected

- `internal/cli/cli.go` for flag registration and usage examples
- `cmd/email/main_test.go` for parse and registration coverage
- `cmd/email/help_test.go` if examples are updated
- `README.md` for command examples and reference text

## Testing Strategy

- Add a parse test that passes `-u 42` and expects `options.UID == 42`
- Extend flag registration coverage to assert `flagSet.Lookup("u") != nil`
- Run the existing CLI test suite to verify no regressions in list/detail behavior

## Risks

- Very low risk because the alias reuses the same destination field as `--uid`
- The main failure mode is incomplete docs or tests rather than runtime behavior changes

## Recommendation

Implement `-u` as a direct alias for `--uid`, update one or more examples to demonstrate it, and keep the rest of the CLI behavior unchanged.
