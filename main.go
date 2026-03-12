package main

import (
	"os"

	"github.com/rogeecn/email-cli/internal/cli"
)

const binaryName = cli.BinaryName

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
