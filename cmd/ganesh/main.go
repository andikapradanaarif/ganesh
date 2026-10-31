package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 1 && args[0] == "doctor" {
		return emit(output, s0.Discover(ctx))
	}
	if len(args) < 2 || args[0] != "s0" {
		return errors.New("usage: ganesh doctor | s0 prepare --dir PATH | s0 check --input PATH --phase inventory|heldout")
	}
	f := flag.NewFlagSet("s0", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	directory := f.String("dir", "", "new private workspace")
	input := f.String("input", "", "private run.json")
	phase := f.String("phase", "inventory", "input-check phase")
	if err := f.Parse(args[2:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid S0 command flags")
	}
	switch args[1] {
	case "prepare":
		if *directory == "" {
			return errors.New("prepare requires --dir")
		}
		r, err := s0.NewRun(time.Now(), s0.Discover(ctx))
		if err != nil {
			return errors.New("cannot generate run identity")
		}
		if err := s0.Prepare(*directory, r); err != nil {
			return err
		}
		return emit(output, struct {
			State         string `json:"state"`
			RunID         string `json:"run_id"`
			ScreenStarted bool   `json:"screen_started"`
		}{"prepared", r.ID, false})
	case "check":
		if *input == "" {
			return errors.New("check requires --input")
		}
		file, err := os.Open(*input)
		if err != nil {
			return errors.New("cannot open private run record")
		}
		defer file.Close()
		body, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
		if err != nil || len(body) > 1024*1024 {
			return errors.New("run record unreadable or exceeds 1 MiB")
		}
		var r s0.Run
		if err := decode(body, &r); err != nil {
			return err
		}
		readiness, err := s0.Check(r, *phase)
		if err != nil {
			return err
		}
		if err := emit(output, readiness); err != nil {
			return err
		}
		if !readiness.InputsPresent {
			return errors.New("required phase inputs are missing; local preparation may continue")
		}
		return nil
	default:
		return errors.New("unknown S0 command")
	}
}

func emit(output io.Writer, value any) error {
	e := json.NewEncoder(output)
	e.SetIndent("", "  ")
	return e.Encode(value)
}
