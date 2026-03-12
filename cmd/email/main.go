package main

import (
	"flag"
	"fmt"
	"os"
)

type cliOptions struct {
	Account string
	Mailbox string
	Limit   int
	Format  string
	UID     uint
}

func newFlagSet() (*flag.FlagSet, *cliOptions) {
	flagSet := flag.NewFlagSet("email", flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)

	options := &cliOptions{}
	flagSet.StringVar(&options.Account, "account", "", "account alias from config")
	flagSet.StringVar(&options.Account, "A", "", "account alias from config")
	flagSet.StringVar(&options.Mailbox, "mailbox", "", "mailbox name")
	flagSet.IntVar(&options.Limit, "limit", 0, "max messages to fetch")
	flagSet.StringVar(&options.Format, "format", "", "output format")
	flagSet.UintVar(&options.UID, "uid", 0, "message UID for detail view")

	return flagSet, options
}

func parseFlags(args []string) (cliOptions, error) {
	flagSet, options := newFlagSet()

	if err := flagSet.Parse(args); err != nil {
		return cliOptions{}, err
	}

	return *options, nil
}

func main() {
	_, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
