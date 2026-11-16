// ganesh-s0-preflight has only offline prepare/inspect operations. It cannot run
// inference, initialize an experiment ledger, install clients or approve a run.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/andikapradanaarif/ganesh/internal/devtools/s0package"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return s0package.ErrInput
	}
	f := flag.NewFlagSet(args[0], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var options s0package.Options
	var dir, hash string
	switch args[0] {
	case "prepare":
		f.StringVar(&options.Packet, "packet", "", "frozen bound packet")
		f.StringVar(&options.Calibration, "calibration", "", "reserved calibration manifest")
		f.StringVar(&options.Roots, "roots-map", "", "recorded project roots metadata")
		f.StringVar(&options.GuardReport, "guard-report", "", "frozen post-review guard report")
		f.StringVar(&options.Output, "out", "", "new private output directory")
	case "inspect":
		f.StringVar(&dir, "package", "", "prepared private package")
		f.StringVar(&hash, "sha256", "", "expected package manifest hash")
		f.StringVar(&options.Output, "out", "", "new private metadata report directory")
	default:
		return s0package.ErrInput
	}
	if f.Parse(args[1:]) != nil || f.NArg() != 0 || options.Output == "" {
		return s0package.ErrInput
	}
	if args[0] == "prepare" {
		if options.Packet == "" || options.Calibration == "" || options.Roots == "" || options.GuardReport == "" {
			return s0package.ErrInput
		}
		sha, err := s0package.Prepare(options)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "Prepared private owner-preflight proposal. Manifest SHA-256: %s\nNo model calls; execution remains unavailable.\n", sha)
		return err
	}
	if dir == "" || len(hash) != 64 {
		return s0package.ErrInput
	}
	r, err := s0package.Inspect(context.Background(), dir, hash, options.Output)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Local preflight recorded: %d selected files; reviewed snapshot matches=%t. Source bodies returned: 0; model calls: 0.\nExecution and complete tool/notes inventories remain unapproved/unverified.\n", len(r.Files), r.AllSelectedMatch)
	return err
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, s0package.ErrInput)
		os.Exit(1)
	}
}
