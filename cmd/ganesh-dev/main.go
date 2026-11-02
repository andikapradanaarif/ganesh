// ganesh-dev hosts optional Go validation tools, never hook execution.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

type options struct{ command, source, profile, directory, study string }

func parse(args []string) (options, error) {
	var opts options
	if len(args) == 0 {
		return opts, devtools.ErrCommand
	}
	opts.command = args[0]
	flags := flag.NewFlagSet(opts.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	switch opts.command {
	case "crush-test":
		flags.StringVar(&opts.source, "source", "", "pinned source checkout")
		flags.StringVar(&opts.profile, "build-profile", "development", "fixed build profile")
	default:
		return options{}, devtools.ErrCommand
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || strings.TrimSpace(opts.source) == "" {
		return options{}, devtools.ErrCommand
	}
	if opts.profile != "development" && opts.profile != "stripped" && opts.profile != "collector" {
		return options{}, devtools.ErrCommand
	}
	return opts, nil
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", devtools.ErrCommand
	}
	for {
		f, err := os.Open(filepath.Join(dir, "go.mod"))
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(f, 4097))
			closeErr := f.Close()
			if readErr != nil || closeErr != nil || len(body) > 4096 || !strings.HasPrefix(string(body), "module github.com/andikapradanaarif/ganesh\n") {
				return "", devtools.ErrCommand
			}
			return dir, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			return "", devtools.ErrCommand
		}
		dir = filepath.Dir(dir)
	}
}

func run(args []string, output io.Writer) error {
	opts, err := parse(args)
	if err != nil {
		return err
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	return devtools.RunCrush(context.Background(), root, opts.source, opts.profile, output)
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
