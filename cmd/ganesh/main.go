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
	os.Exit(commandExitCode(os.Args[1:], func() error {
		return run(context.Background(), os.Args[1:], os.Stdout)
	}, os.Stderr))
}

func commandExitCode(args []string, execute func() error, diagnostic io.Writer) int {
	if len(args) > 0 && args[0] == "hook" {
		return hookExitCode(execute)
	}
	if err := execute(); err != nil {
		fmt.Fprintln(diagnostic, err)
		return 1
	}
	return 0
}

// A recoverable panic in the hook handler must not escape as Go's exit 2,
// which Crush interprets as a tool denial. Never format the panic value.
// Recovery is limited to this goroutine; runtime fatal errors are not caught.
func hookExitCode(execute func() error) (status int) {
	status = 1
	defer func() { _ = recover() }()
	if execute() == nil {
		return 0
	}
	return status
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) > 0 && args[0] == "capture" {
		return captureFromStdin(ctx, args[1:], output)
	}
	if len(args) > 0 && args[0] == "hook" {
		return hookCommand(ctx, args[1:], os.Stdin)
	}
	if len(args) == 1 && args[0] == "doctor" {
		return emit(output, s0.Discover(ctx))
	}
	if len(args) < 2 || args[0] != "s0" {
		return errors.New("usage: ganesh doctor | capture --spool DIR | hook crush --spool DIR | s0 prepare|check|freeze-targets|screen [flags]")
	}
	f := flag.NewFlagSet(args[1], flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var directory, input, outputPath, baselinePath, phase string
	switch args[1] {
	case "prepare":
		f.StringVar(&directory, "dir", "", "new private workspace")
	case "check":
		f.StringVar(&input, "input", "", "private run.json")
		f.StringVar(&phase, "phase", "inventory", "input-check phase")
	case "freeze-targets":
		f.StringVar(&input, "input", "", "private target inventory")
		f.StringVar(&outputPath, "output", "", "new private need-definition snapshot")
	case "screen":
		f.StringVar(&input, "input", "", "private target inventory")
		f.StringVar(&baselinePath, "baseline", "", "frozen need-definition snapshot")
	default:
		return errors.New("unknown S0 command")
	}
	if err := f.Parse(args[2:]); err != nil || f.NArg() != 0 {
		return errors.New("invalid S0 command flags")
	}
	switch args[1] {
	case "prepare":
		if directory == "" {
			return errors.New("prepare requires --dir")
		}
		r, err := s0.NewRun(time.Now(), s0.Discover(ctx))
		if err != nil {
			return errors.New("cannot generate run identity")
		}
		if err := s0.Prepare(directory, r); err != nil {
			return err
		}
		return emit(output, struct {
			State         string `json:"state"`
			RunID         string `json:"run_id"`
			ScreenStarted bool   `json:"screen_started"`
		}{"prepared", r.ID, false})
	case "check":
		var r s0.Run
		info, err := readRecordInfo(input, &r)
		if err != nil {
			return err
		}
		readiness, err := s0.Check(r, phase)
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0044 != 0 {
			readiness.Warnings = append(readiness.Warnings, "run_record_group_or_other_readable")
		}
		if err := emit(output, readiness); err != nil {
			return err
		}
		if !readiness.InputsPresent {
			return errors.New("required phase inputs are missing; local preparation may continue")
		}
		return nil
	case "freeze-targets":
		if outputPath == "" {
			return errors.New("freeze-targets requires --output")
		}
		var inventory s0.TargetInventory
		if err := readRecord(input, &inventory); err != nil {
			return err
		}
		snapshot, err := s0.FreezeNeeds(inventory)
		if err != nil {
			return err
		}
		if err := s0.SaveNeedSnapshot(outputPath, snapshot); err != nil {
			return err
		}
		digest, err := snapshot.Digest()
		if err != nil {
			return err
		}
		return emit(output, struct {
			State  string `json:"state"`
			Digest string `json:"need_digest"`
			Count  int    `json:"target_count"`
		}{"needs_frozen", digest, len(snapshot.Targets)})
	case "screen":
		var inventory s0.TargetInventory
		if err := readRecord(input, &inventory); err != nil {
			return err
		}
		var baseline *s0.NeedSnapshot
		if baselinePath != "" {
			baseline = &s0.NeedSnapshot{}
			if err := readRecord(baselinePath, baseline); err != nil {
				return err
			}
		}
		report, err := s0.InitialScreen(inventory, baseline)
		if err != nil {
			return err
		}
		if err := emit(output, report); err != nil {
			return err
		}
		if report.Route == "inconclusive" {
			return errors.New("origin screen is inconclusive")
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
