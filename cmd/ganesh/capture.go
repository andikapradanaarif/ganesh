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
	return captureInput(parent, input, output, *directory, func(_ context.Context, r io.Reader) (capture.Metadata, error) {
		body, err := io.ReadAll(io.LimitReader(r, capture.MaxInput+1))
		if err != nil {
			return capture.Metadata{}, errors.New("capture input read failed")
		}
		return capture.Decode(body)
	}, true)
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
	return captureInput(parent, input, io.Discard, *directory, crush.ReduceStream, false)
}

type metadataResult struct {
	metadata capture.Metadata
	err      error
}

// Reduction runs on the input worker so an unclosed stdin cannot hold the
// process past its deadline. Recover here as well as at the CLI boundary;
// panics cannot be recovered across goroutines or printed as Crush exit 2.
func reduceCaptureInput(ctx context.Context, input io.Reader, reduce func(context.Context, io.Reader) (capture.Metadata, error)) (result metadataResult) {
	result.err = errors.New("capture input reduction failed")
	defer func() { _ = recover() }()
	result.metadata, result.err = reduce(ctx, input)
	return result
}

func captureInput(parent context.Context, input io.Reader, output io.Writer, directory string, reduce func(context.Context, io.Reader) (capture.Metadata, error), emitReceipt bool) error {
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
	ready := make(chan metadataResult, 1)
	go func() {
		ready <- reduceCaptureInput(ctx, input, reduce)
	}()
	var metadata capture.Metadata
	select {
	case <-ctx.Done():
		return errors.New("capture input deadline exceeded")
	case result := <-ready:
		if result.err != nil {
			return result.err
		}
		metadata = result.metadata
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
