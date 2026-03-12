package main

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

type cliOptions struct {
	Account    string
	ConfigPath string
	Mailbox    string
	Limit      int
	Format     string
	UID        uint
}

type runner interface {
	Run(ctx context.Context, options app.Options) (app.Result, error)
}

type runnerFactory func() runner

type fileLoader struct {
	path string
}

func (l fileLoader) Load() (config.Config, error) {
	return config.LoadFile(l.path)
}

func DefaultConfigPath() string {
	return config.DefaultPath()
}

func newFlagSet() (*flag.FlagSet, *cliOptions) {
	flagSet := flag.NewFlagSet("email", flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)

	options := &cliOptions{}
	flagSet.StringVar(&options.Account, "account", "", "account alias from config")
	flagSet.StringVar(&options.Account, "A", "", "account alias from config")
	flagSet.StringVar(&options.ConfigPath, "config", "", "custom config file path")
	flagSet.StringVar(&options.ConfigPath, "c", "", "custom config file path")
	flagSet.StringVar(&options.Mailbox, "mailbox", "", "mailbox name")
	flagSet.IntVar(&options.Limit, "limit", 0, "max messages to fetch")
	flagSet.StringVar(&options.Format, "format", "", "output format")
	flagSet.UintVar(&options.UID, "uid", 0, "message UID for detail view")
	flagSet.Usage = func() {
		output := flagSet.Output()
		fmt.Fprintf(output, "Fetch email from IMAP accounts.\n\n")
		fmt.Fprintf(output, "Usage:\n  email [flags]\n\n")
		fmt.Fprintf(output, "Behavior:\n")
		fmt.Fprintf(output, "  - Without --uid, lists recent messages from the target account\n")
		fmt.Fprintf(output, "  - With --uid, shows one full message and body\n")
		fmt.Fprintf(output, "  - Without -A/--account, uses default_account from config\n\n")
		fmt.Fprintf(output, "Examples:\n")
		fmt.Fprintf(output, "  email\n")
		fmt.Fprintf(output, "  email -A personal\n")
		fmt.Fprintf(output, "  email -c ./config.toml -A personal\n")
		fmt.Fprintf(output, "  email -A personal --uid 12345\n")
		fmt.Fprintf(output, "  email -A work --format json\n\n")
		fmt.Fprintf(output, "Config:\n")
		fmt.Fprintf(output, "  default path: %s\n\n", DefaultConfigPath())
		fmt.Fprintf(output, "Flags:\n")
		flagSet.PrintDefaults()
	}

	return flagSet, options
}

func parseFlags(args []string) (cliOptions, error) {
	flagSet, options := newFlagSet()

	if err := flagSet.Parse(args); err != nil {
		return cliOptions{}, err
	}

	return *options, nil
}

func execute(ctx context.Context, appRunner runner, args []string, stdout io.Writer, stderr io.Writer) error {
	cliOptions, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	result, err := appRunner.Run(ctx, app.Options{
		Account: cliOptions.Account,
		Mailbox: cliOptions.Mailbox,
		Limit:   cliOptions.Limit,
		Format:  cliOptions.Format,
		UID:     uint32(cliOptions.UID),
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	var rendered []byte
	if result.Mode == app.ModeDetail {
		rendered, err = output.RenderDetail(result.Detail, result.Format)
	} else {
		rendered, err = output.RenderSummaries(result.Summaries, result.Format)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}

	_, err = stdout.Write(rendered)
	return err
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, factory runnerFactory) int {
	if _, err := parseFlags(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if err := execute(ctx, factory(), args, stdout, stderr); err != nil {
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

func main() {
	cliOptions, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	configPath := DefaultConfigPath()
	if cliOptions.ConfigPath != "" {
		configPath = cliOptions.ConfigPath
	}

	loader := fileLoader{path: configPath}
	mailRuntime := imapservice.NewDefaultRuntimeClient()
	application := app.New(loader, mailRuntime)

	exitCode := run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, func() runner {
		return application
	})
	os.Exit(exitCode)
}
