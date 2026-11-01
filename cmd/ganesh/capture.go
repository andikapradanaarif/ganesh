package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/adapter/crush"
	"github.com/andikapradanaarif/ganesh/internal/capture"
	"github.com/andikapradanaarif/ganesh/internal/processclock"
)

const captureBudget = 750 * time.Millisecond

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
	return captureInput(parent, input, output, *directory, capture.Decode, true)
}

func hookCommand(parent context.Context, args []string, input io.Reader) error {
	if len(args) == 0 || args[0] != "crush" {
		return errors.New("unsupported capture hook adapter")
	}
	f := flag.NewFlagSet("hook crush", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	directory := f.String("spool", "", "dedicated private local spool with an existing parent")
	if err := f.Parse(args[1:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid capture hook flags")
	}
	if *directory == "" {
		return errors.New("capture hook requires --spool")
	}
	return captureInput(parent, input, io.Discard, *directory, crush.Reduce, false)
}

func captureInput(parent context.Context, input io.Reader, output io.Writer, directory string, reduce func([]byte) (capture.Metadata, error), emitReceipt bool) error {
	start, err := processclock.Start()
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	if elapsed < 0 || elapsed >= captureBudget {
		return errors.New("capture process-start deadline exceeded or clock invalid")
	}
	ctx, cancel := context.WithTimeout(parent, captureBudget-elapsed)
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
	metadata, err := reduce(body)
	if err != nil {
		return err
	}
	receipt, err := (capture.Spool{Directory: directory}).Append(ctx, metadata)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	duration := time.Since(start)
	if duration < 0 || duration >= captureBudget || ctx.Err() != nil {
		return errors.New("capture process-start deadline exceeded or clock invalid")
	}
	if !emitReceipt {
		// The durable record is the local receipt. Hook stdout would enter
		// the client's control/context parser, so no development JSON is sent.
		return nil
	}
	return emitCaptureReceipt(ctx, output, struct {
		capture.Receipt
		ProcessStartToReceiptMS float64 `json:"process_start_to_receipt_ms"`
	}{receipt, float64(duration) / float64(time.Millisecond)})
}

func captureFromStdin(ctx context.Context, args []string, output io.Writer) error {
	return captureCommand(ctx, args, os.Stdin, output)
}
