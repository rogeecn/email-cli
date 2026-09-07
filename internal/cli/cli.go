package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/rogeecn/email-cli/internal/app"
	"github.com/rogeecn/email-cli/internal/config"
	imapservice "github.com/rogeecn/email-cli/internal/imap"
	"github.com/rogeecn/email-cli/internal/output"
)

const BinaryName = "email-cli"

type Options struct {
	Account    string
	ConfigPath string
	Debug      bool
	Mailbox    string
	Limit      int
	Offset     int
	Format     string
	UID        uint
	ID         string
}

type Runner interface {
	Run(ctx context.Context, options app.Options) (app.Result, error)
}

type RunnerFactory func() Runner

type fileLoader struct {
	path string
}

func (l fileLoader) Load() (config.Config, error) {
	return config.LoadFile(l.path)
}

func DefaultConfigPath() string {
	return config.DefaultPath()
}

func NewFlagSet() (*flag.FlagSet, *Options) {
	flagSet := flag.NewFlagSet(BinaryName, flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)

	options := &Options{}
	flagSet.StringVar(&options.Account, "account", "", "account alias from config")
	flagSet.StringVar(&options.Account, "A", "", "account alias from config")
	flagSet.StringVar(&options.ConfigPath, "config", "", "custom config file path")
	flagSet.StringVar(&options.ConfigPath, "c", "", "custom config file path")
	flagSet.BoolVar(&options.Debug, "debug", false, "print receive debug logs to stderr")
	flagSet.StringVar(&options.Mailbox, "mailbox", "", "mailbox name")
	flagSet.IntVar(&options.Limit, "limit", 0, "max messages to fetch")
	flagSet.IntVar(&options.Offset, "offset", 0, "messages to skip before listing")
	flagSet.StringVar(&options.Format, "format", "", "output format")
	flagSet.UintVar(&options.UID, "u", 0, "message UID for detail view")
	flagSet.UintVar(&options.UID, "uid", 0, "message UID for detail view")
	flagSet.StringVar(&options.ID, "id", "", "MailClaw string ID for detail view")
	flagSet.Usage = func() {
		output := flagSet.Output()
		fmt.Fprintf(output, "Fetch email from IMAP accounts or MailClaw accounts.\n\n")
		fmt.Fprintf(output, "Usage:\n  %s [flags]\n  %s <command> [flags]\n\n", BinaryName, BinaryName)
		fmt.Fprintf(output, "Behavior:\n")
		fmt.Fprintf(output, "  - Without -u/--uid or --id, lists recent messages from the target account\n")
		fmt.Fprintf(output, "  - With -u/--uid (IMAP) or --id (MailClaw), shows one full message and body\n")
		fmt.Fprintf(output, "  - Without -A/--account, uses default_account from config\n\n")
		fmt.Fprintf(output, "Examples:\n")
		fmt.Fprintf(output, "  %s\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal\n", BinaryName)
		fmt.Fprintf(output, "  %s -c ./config.toml -A personal\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal -u 12345\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal --offset 10 --limit 10\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal --debug\n", BinaryName)
		fmt.Fprintf(output, "  %s -A work --format json\n\n", BinaryName)
		fmt.Fprintf(output, "Commands:\n")
		fmt.Fprintf(output, "  list, get, export, send, delete, attachments, download, health\n")
		fmt.Fprintf(output, "  Backend support is determined by -A/--account or default_account.\n\n")
		fmt.Fprintf(output, "MailClaw examples (host/api_token in the same TOML account config):\n")
		fmt.Fprintf(output, "  %s list -A cloud --format json\n", BinaryName)
		fmt.Fprintf(output, "  %s send -A cloud --to user@example.com --subject Hello --text Hi\n\n", BinaryName)
		fmt.Fprintf(output, "Config:\n")
		fmt.Fprintf(output, "  default path: %s\n\n", DefaultConfigPath())
		fmt.Fprintf(output, "Flags:\n")
		flagSet.PrintDefaults()
	}

	return flagSet, options
}

func ParseFlags(args []string) (Options, error) {
	return parseFlags(args, os.Stderr)
}

func parseFlags(args []string, diagnostics io.Writer) (Options, error) {
	flagSet, options := NewFlagSet()
	flagSet.SetOutput(diagnostics)

	if err := flagSet.Parse(args); err != nil {
		return Options{}, err
	}

	if flagSet.NArg() != 0 {
		return Options{}, errors.New("unexpected positional arguments; use email-cli --help for commands")
	}
	if uint64(options.UID) > uint64(^uint32(0)) {
		return Options{}, errors.New("IMAP UID must be between 1 and 4294967295")
	}
	var invalid string
	flagSet.Visit(func(f *flag.Flag) {
		if (f.Name == "uid" || f.Name == "u") && options.UID == 0 {
			invalid = "IMAP UID must be between 1 and 4294967295"
		}
		if f.Name == "limit" && options.Limit < 1 {
			invalid = "limit must be positive"
		}
		if f.Name == "id" && options.ID == "" {
			invalid = "MailClaw ID must not be empty"
		}
	})
	if invalid != "" {
		return Options{}, errors.New(invalid)
	}
	if options.Offset < 0 {
		return Options{}, errors.New("offset must be non-negative")
	}
	if options.ID != "" && options.UID != 0 {
		return Options{}, errors.New("--id and --uid are mutually exclusive")
	}
	return *options, nil
}

func Execute(ctx context.Context, appRunner Runner, args []string, stdout io.Writer, stderr io.Writer) error {
	cliOptions, err := parseFlags(args, stderr)
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, err)
		}
		return err
	}

	result, err := appRunner.Run(ctx, app.Options{
		Account: cliOptions.Account,
		Mailbox: cliOptions.Mailbox,
		Limit:   cliOptions.Limit,
		Offset:  cliOptions.Offset,
		Format:  cliOptions.Format,
		UID:     uint32(cliOptions.UID),
		ID:      cliOptions.ID,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	if err := renderResult(stdout, result, cliOptions.Debug); err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}
	return nil
}

func renderResult(stdout io.Writer, result app.Result, debug bool) error {
	var rendered []byte
	var err error
	if result.APIData != nil {
		rendered, err = output.RenderAPI(result.APIData, result.Format)
	} else if result.Mode == app.ModeDetail {
		rendered, err = output.RenderDetail(result.Detail, result.Format, debug)
	} else {
		rendered, err = output.RenderSummaries(result.Summaries, result.Format, result.ListMetadata)
	}
	if err != nil {
		return err
	}
	_, err = stdout.Write(rendered)
	return err
}

func isCommand(command string) bool {
	switch command {
	case "list", "get", "export", "send", "delete", "attachments", "download", "health":
		return true
	default:
		return false
	}
}

func runCommand(ctx context.Context, args []string, stdout, stderr io.Writer, factory RunnerFactory) int {
	if err := executeMailClaw(ctx, args, stdout, stderr, factory); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, factory RunnerFactory) int {
	if len(args) > 0 && isCommand(args[0]) {
		return runCommand(ctx, args, stdout, stderr, factory)
	}
	if len(args) > 0 && args[0] == "mailclaw" {
		fmt.Fprintln(stderr, "the mailclaw prefix was removed; use email-cli <command> -A <account>")
		return 2
	}
	if _, err := parseFlags(args, stderr); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	if err := Execute(ctx, factory(), args, stdout, stderr); err != nil {
		var numError *strconv.NumError
		if errors.As(err, &numError) {
			return 2
		}
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}

	return 0
}

func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) > 0 && isCommand(args[0]) {
		return runCommand(context.Background(), args, stdout, stderr, nil)
	}
	if len(args) > 0 && args[0] == "mailclaw" {
		fmt.Fprintln(stderr, "the mailclaw prefix was removed; use email-cli <command> -A <account>")
		return 2
	}
	cliOptions, err := parseFlags(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(stderr, err)
		return 2
	}

	configPath := DefaultConfigPath()
	if cliOptions.ConfigPath != "" {
		configPath = cliOptions.ConfigPath
	}

	loader := fileLoader{path: configPath}
	mailRuntime := imapservice.NewDefaultRuntimeClient()
	if cliOptions.Debug {
		mailRuntime = mailRuntime.WithDebugOutput(stderr)
		fmt.Fprintf(stderr, "[debug] config path: %s\n", configPath)
	}
	application := app.New(loader, mailRuntime)

	return Run(context.Background(), args, stdout, stderr, func() Runner {
		return application
	})
}
