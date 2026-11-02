package main

import (
	"context"
	"io"
	"os"

	"github.com/andikapradanaarif/ganesh/internal/capturecli"
)

func captureFromStdin(ctx context.Context, args []string, output io.Writer) error {
	return capturecli.Capture(ctx, args, os.Stdin, output)
}

func hookCommand(ctx context.Context, args []string, input io.Reader) error {
	return capturecli.Hook(ctx, args, input)
}
