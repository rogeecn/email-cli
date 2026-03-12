package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/rogeecn/email-cli/internal/app"
	"github.com/rogeecn/email-cli/internal/mail"
)

func TestParseFlagsReadsAliasUIDConfigPathAndDebug(t *testing.T) {
	options, err := parseFlags([]string{"-A", "personal", "--uid", "42", "--mailbox", "Archive", "--limit", "10", "--offset", "5", "--format", "json", "-c", "./custom.toml", "--debug"})
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}

	if options.Account != "personal" {
		t.Fatalf("Account = %q, want %q", options.Account, "personal")
	}
	if options.UID != 42 {
		t.Fatalf("UID = %d, want 42", options.UID)
	}
	if options.Mailbox != "Archive" {
		t.Fatalf("Mailbox = %q, want %q", options.Mailbox, "Archive")
	}
	if options.Limit != 10 {
		t.Fatalf("Limit = %d, want 10", options.Limit)
	}
	if options.Offset != 5 {
		t.Fatalf("Offset = %d, want 5", options.Offset)
	}
	if options.Format != "json" {
		t.Fatalf("Format = %q, want %q", options.Format, "json")
	}
	if options.ConfigPath != "./custom.toml" {
		t.Fatalf("ConfigPath = %q, want %q", options.ConfigPath, "./custom.toml")
	}
	if !options.Debug {
		t.Fatalf("Debug = false, want true")
	}
}

func TestNewFlagSetSupportsShortAliasConfigAndDebugFlag(t *testing.T) {
	flagSet, _ := newFlagSet()
	if flagSet.Lookup("A") == nil {
		t.Fatalf("short -A flag should be registered")
	}
	if flagSet.Lookup("account") == nil {
		t.Fatalf("--account flag should be registered")
	}
	if flagSet.Lookup("c") == nil {
		t.Fatalf("short -c flag should be registered")
	}
	if flagSet.Lookup("config") == nil {
		t.Fatalf("--config flag should be registered")
	}
	if flagSet.Lookup("debug") == nil {
		t.Fatalf("--debug flag should be registered")
	}
	if flagSet.ErrorHandling() != flag.ContinueOnError {
		t.Fatalf("ErrorHandling = %v, want %v", flagSet.ErrorHandling(), flag.ContinueOnError)
	}
}

type fakeRunner struct {
	result app.Result
	err    error
	seen   app.Options
}

func (f *fakeRunner) Run(_ context.Context, options app.Options) (app.Result, error) {
	f.seen = options
	return f.result, f.err
}

func TestExecuteWritesListOutput(t *testing.T) {
	runner := &fakeRunner{result: app.Result{
		Mode:   app.ModeList,
		Format: "plain",
		Summaries: []mail.Summary{{
			UID:     7,
			Date:    "2026-03-12T10:00:00Z",
			From:    "Alice <alice@example.com>",
			To:      []string{"Bob <bob@example.com>"},
			Subject: "Hello",
			Seen:    true,
		}},
	}}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := execute(context.Background(), runner, []string{"-A", "personal"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if runner.seen.Account != "personal" {
		t.Fatalf("runner account = %q, want %q", runner.seen.Account, "personal")
	}
	if !strings.Contains(stdout.String(), "Hello") {
		t.Fatalf("stdout should contain rendered summary, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", stderr.String())
	}
}

func TestExecuteWritesDetailWithoutHeadersByDefault(t *testing.T) {
	runner := &fakeRunner{result: app.Result{
		Mode:   app.ModeDetail,
		Format: "plain",
		Detail: mail.Detail{
			Summary: mail.Summary{
				UID:     9,
				Date:    "2026-03-12T10:00:00Z",
				From:    "Alice <alice@example.com>",
				To:      []string{"Bob <bob@example.com>"},
				Subject: "Hello",
				Seen:    true,
			},
			TextBody: "Plain body",
			Headers:  map[string]string{"Message-ID": "<123@example.com>"},
		},
	}}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := execute(context.Background(), runner, []string{"--uid", "9"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if strings.Contains(stdout.String(), "Headers:") {
		t.Fatalf("stdout should hide headers by default, got %q", stdout.String())
	}
}

func TestExecuteWritesDetailHeadersInDebugMode(t *testing.T) {
	runner := &fakeRunner{result: app.Result{
		Mode:   app.ModeDetail,
		Format: "plain",
		Detail: mail.Detail{
			Summary: mail.Summary{
				UID:     9,
				Date:    "2026-03-12T10:00:00Z",
				From:    "Alice <alice@example.com>",
				To:      []string{"Bob <bob@example.com>"},
				Subject: "Hello",
				Seen:    true,
			},
			TextBody: "Plain body",
			Headers:  map[string]string{"Message-ID": "<123@example.com>"},
		},
	}}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := execute(context.Background(), runner, []string{"--uid", "9", "--debug"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Headers:") {
		t.Fatalf("stdout should include headers in debug mode, got %q", stdout.String())
	}
}

func TestExecuteWritesErrorToStderr(t *testing.T) {
	runner := &fakeRunner{err: context.DeadlineExceeded}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := execute(context.Background(), runner, []string{"--uid", "9"}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected execute to return error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout should be empty, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), context.DeadlineExceeded.Error()) {
		t.Fatalf("stderr should contain error, got %q", stderr.String())
	}
}

func TestRunReturnsSuccessExitCode(t *testing.T) {
	fake := &fakeRunner{result: app.Result{Mode: app.ModeList, Format: "json", Summaries: []mail.Summary{}}}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(context.Background(), []string{}, &stdout, &stderr, func() runner { return fake })
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
}

func TestRunReturnsErrorExitCodeFromExecution(t *testing.T) {
	fake := &fakeRunner{err: errors.New("boom")}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(context.Background(), []string{}, &stdout, &stderr, func() runner { return fake })
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1", exitCode)
	}
}

func TestRunReturnsFlagErrorExitCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run(context.Background(), []string{"--uid", "not-a-number"}, &stdout, &stderr, func() runner {
		t.Fatalf("runner factory should not be called on flag parse error")
		return nil
	})
	if exitCode != 2 {
		t.Fatalf("exitCode = %d, want 2", exitCode)
	}
}
