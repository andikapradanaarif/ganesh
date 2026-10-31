package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
	"github.com/andikapradanaarif/ganesh/internal/processclock"
)

func captureCommand(parent context.Context, args []string, input io.Reader, output io.Writer) error {
	f := flag.NewFlagSet("capture", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	directory := f.String("spool", "", "private local spool directory with an existing parent")
	if err := f.Parse(args); err != nil || f.NArg() != 0 {
		return errors.New("invalid capture command flags")
	}
	if *directory == "" {
		return errors.New("capture requires --spool")
	}
	start, err := processclock.Start()
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	if elapsed < 0 || elapsed >= 750*time.Millisecond {
		return errors.New("capture process-start deadline exceeded or clock invalid")
	}
	ctx, cancel := context.WithTimeout(parent, 750*time.Millisecond-elapsed)
	defer cancel()
	type readResult struct {
		body []byte
		err  error
	}
	ready := make(chan readResult, 1)
	go func() {
		body, err := io.ReadAll(io.LimitReader(input, capture.MaxInput+1))
		ready <- readResult{body, err}
	}()
	var body []byte
	select {
	case <-ctx.Done():
		return errors.New("capture input deadline exceeded")
	case result := <-ready:
		if result.err != nil {
			return errors.New("capture input read failed")
		}
		body = result.body
	}
	metadata, err := capture.Decode(body)
	if err != nil {
		return err
	}
	receipt, err := (capture.Spool{Directory: *directory}).Append(ctx, metadata)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	duration := time.Since(start)
	if duration < 0 || duration >= 750*time.Millisecond || ctx.Err() != nil {
		return errors.New("capture process-start deadline exceeded or clock invalid")
	}
	return emit(output, struct {
		capture.Receipt
		ProcessStartToReceiptMS float64 `json:"process_start_to_receipt_ms"`
	}{receipt, float64(duration) / float64(time.Millisecond)})
}

func captureFromStdin(ctx context.Context, args []string, output io.Writer) error {
	return captureCommand(ctx, args, os.Stdin, output)
}
