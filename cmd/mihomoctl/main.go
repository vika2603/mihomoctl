package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/the-super-company/mihomoctl/internal/cli"
)

func main() {
	signal.Ignore(syscall.SIGPIPE)
	if code := cli.Run(os.Args[1:], os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}
