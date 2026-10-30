package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
	"github.com/andikapradanaarif/ganesh/internal/processclock"
	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: ganesh s0 init|check|route, or ganesh capture --spool DIRECTORY")
	}
	if args[0] == "capture" {
		return captureCommand(args[1:])
	}
	if len(args) < 2 || args[0] != "s0" {
		return errors.New("unknown command")
	}
	f := flag.NewFlagSet("s0", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	input := f.String("input", "", "local JSON input")
	output := f.String("output", "", "new run-input file")
	final := f.Bool("final", false, "evaluate the one final pre-S0b-2 eligibility recheck")
	if err := f.Parse(args[2:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid S0 command flags")
	}
	switch args[1] {
	case "init":
		if *output == "" {
			return errors.New("init requires --output; file must not already exist")
		}
		file, err := os.OpenFile(*output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("cannot create run-input file; create its parent directory and do not overwrite a frozen run")
		}
		defer file.Close()
		if err := writeJSON(file, s0.NewRunInputs()); err != nil {
			return err
		}
		return file.Sync()
	case "check":
		var in s0.RunInputs
		if err := readJSON(*input, &in); err != nil {
			return err
		}
		missing := in.Missing()
		if err := writeJSON(os.Stdout, struct {
			Missing            []string `json:"missing"`
			HeldOutRunsStarted bool     `json:"held_out_runs_started"`
		}{missing, false}); err != nil {
			return err
		}
		if len(missing) != 0 {
			return errors.New("run inputs incomplete; inventory and local tooling may continue")
		}
		return nil
	case "route":
		var in s0.Inventory
		if err := readJSON(*input, &in); err != nil {
			return err
		}
		decision, err := s0.Evaluate(in, *final)
		if err != nil {
			return err
		}
		return writeJSON(os.Stdout, decision)
	default:
		return errors.New("unknown S0 command")
	}
}

func captureCommand(args []string) error {
	start, err := processclock.Start()
	if err != nil {
		return err
	}
	remaining := 750*time.Millisecond - time.Since(start)
	if remaining <= 0 || remaining > 750*time.Millisecond {
		return errors.New("capture process-start deadline exceeded or clock invalid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), remaining)
	defer cancel()
	f := flag.NewFlagSet("capture", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	spool := f.String("spool", "", "private local spool directory")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || *spool == "" {
		return errors.New("capture requires --spool DIRECTORY and normalized metadata on stdin")
	}
	type readResult struct {
		body []byte
		err  error
	}
	ready := make(chan readResult, 1)
	go func() { b, e := io.ReadAll(io.LimitReader(os.Stdin, capture.MaxInput+1)); ready <- readResult{b, e} }()
	var body []byte
	select {
	case <-ctx.Done():
		return errors.New("capture process-start deadline exceeded")
	case result := <-ready:
		if result.err != nil {
			return errors.New("cannot read normalized metadata")
		}
		body = result.body
	}
	m, err := capture.Decode(body)
	if err != nil {
		return err
	}
	receipt, err := (capture.Spool{Directory: *spool}).Append(ctx, m)
	if err != nil {
		return err
	}
	return writeJSON(os.Stdout, struct {
		capture.Receipt
		ProcessStartToReceiptMS float64 `json:"process_start_to_receipt_ms"`
	}{receipt, float64(time.Since(start)) / float64(time.Millisecond)})
}

func readJSON(path string, destination any) error {
	if path == "" {
		return errors.New("--input is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("cannot open local input")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(b) > 1024*1024 {
		return errors.New("local input unreadable or exceeds 1 MiB limit")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(destination) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid local JSON input")
	}
	return nil
}

func writeJSON(w io.Writer, v any) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(v)
}
