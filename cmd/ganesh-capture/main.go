// ganesh-capture is an experimental capture-only entry point. It shares the
// existing collector and does not perform discovery, preparation or upload.
package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/andikapradanaarif/ganesh/internal/capturecli"
)

func main() {
	os.Exit(capturecli.ExitCode(os.Args[1:], func() error {
		return run(context.Background(), os.Args[1:], os.Stdin, os.Stdout)
	}, os.Stderr))
}

func run(ctx context.Context, args []string, input io.Reader, output io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "capture":
			return capturecli.Capture(ctx, args[1:], input, output)
		case "hook":
			return capturecli.Hook(ctx, args[1:], input)
		}
	}
	return errors.New("usage: ganesh-capture capture --spool DIR | hook crush --spool DIR")
}
