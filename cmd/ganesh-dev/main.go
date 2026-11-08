// ganesh-dev hosts optional Go validation tools, never hook execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
)

type options struct{ command, source, profile, directory, study, raw, transcriptRoot, grantMapDirectory string }

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
	case "study-summary":
		flags.StringVar(&opts.directory, "dir", "", "private completed study directory")
		flags.StringVar(&opts.study, "study", "", "completed study: 34 or 36")
	case "corpus-audit":
		flags.StringVar(&opts.directory, "dir", "", "private corpus directory")
		flags.StringVar(&opts.raw, "raw", "", "comma-separated private raw snapshot basenames")
	case "devin-audit":
		flags.StringVar(&opts.directory, "dir", "", "private Devin evidence directory")
		flags.StringVar(&opts.transcriptRoot, "transcript-root", "", "explicit Devin transcript directory")
		flags.StringVar(&opts.grantMapDirectory, "grant-map-dir", "", "private owner-grant project-root map directory")
	default:
		return options{}, devtools.ErrCommand
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "crush-test" && (strings.TrimSpace(opts.source) == "" || (opts.profile != "development" && opts.profile != "stripped" && opts.profile != "collector")) {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "study-summary" && (strings.TrimSpace(opts.directory) == "" || (opts.study != "34" && opts.study != "36")) {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "corpus-audit" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.raw) == "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "devin-audit" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.transcriptRoot) == "" || strings.TrimSpace(opts.grantMapDirectory) == "") {
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

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parse(args)
	if err != nil {
		return err
	}
	if opts.command == "study-summary" {
		study := 34
		if opts.study == "36" {
			study = 36
		}
		report, err := devtools.SummarizeStudy(study, opts.directory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrStudy
		}
		return nil
	}
	if opts.command == "corpus-audit" {
		report, err := devtools.AuditCorpus(opts.directory, strings.Split(opts.raw, ","))
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrCorpus
		}
		return nil
	}
	if opts.command == "devin-audit" {
		report, err := devtools.AuditDevin(opts.directory, opts.transcriptRoot, opts.grantMapDirectory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrDevinAudit
		}
		return nil
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	return devtools.RunCrush(context.Background(), root, opts.source, opts.profile, stdout, stderr)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
