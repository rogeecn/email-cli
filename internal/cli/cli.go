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
	flagSet.Usage = func() {
		output := flagSet.Output()
		fmt.Fprintf(output, "Fetch email from IMAP accounts.\n\n")
		fmt.Fprintf(output, "Usage:\n  %s [flags]\n\n", BinaryName)
		fmt.Fprintf(output, "Behavior:\n")
		fmt.Fprintf(output, "  - Without -u/--uid, lists recent messages from the target account\n")
		fmt.Fprintf(output, "  - With -u/--uid, shows one full message and body\n")
		fmt.Fprintf(output, "  - Without -A/--account, uses default_account from config\n\n")
		fmt.Fprintf(output, "Examples:\n")
		fmt.Fprintf(output, "  %s\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal\n", BinaryName)
		fmt.Fprintf(output, "  %s -c ./config.toml -A personal\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal -u 12345\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal --offset 10 --limit 10\n", BinaryName)
		fmt.Fprintf(output, "  %s -A personal --debug\n", BinaryName)
		fmt.Fprintf(output, "  %s -A work --format json\n\n", BinaryName)
		fmt.Fprintf(output, "Config:\n")
		fmt.Fprintf(output, "  default path: %s\n\n", DefaultConfigPath())
		fmt.Fprintf(output, "Flags:\n")
		flagSet.PrintDefaults()
	}

	return flagSet, options
}

func ParseFlags(args []string) (Options, error) {
	flagSet, options := NewFlagSet()

	if err := flagSet.Parse(args); err != nil {
		return Options{}, err
	}

	return *options, nil
}

func Execute(ctx context.Context, appRunner Runner, args []string, stdout io.Writer, stderr io.Writer) error {
	cliOptions, err := ParseFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	result, err := appRunner.Run(ctx, app.Options{
		Account: cliOptions.Account,
		Mailbox: cliOptions.Mailbox,
		Limit:   cliOptions.Limit,
		Offset:  cliOptions.Offset,
		Format:  cliOptions.Format,
		UID:     uint32(cliOptions.UID),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	var rendered []byte
	if result.Mode == app.ModeDetail {
		rendered, err = output.RenderDetail(result.Detail, result.Format, cliOptions.Debug)
	} else {
		rendered, err = output.RenderSummaries(result.Summaries, result.Format, result.ListMetadata)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	_, err = stdout.Write(rendered)
	return err
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, factory RunnerFactory) int {
	if _, err := ParseFlags(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if err := Execute(ctx, factory(), args, stdout, stderr); err != nil {
		var numError *strconv.NumError
		if errors.As(err, &numError) {
			return 2
		}
		if errors.Is(err, flag.ErrHelp) {
			return 2
		}
		return 1
	}

	return 0
}

func Main(args []string, stdout io.Writer, stderr io.Writer) int {
	cliOptions, err := ParseFlags(args)
	if err != nil {
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
