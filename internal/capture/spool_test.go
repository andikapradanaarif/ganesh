package capture

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture() Metadata {
	return Metadata{1, "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "tool_result", "failed"}
}

func TestMetadataRejectsPayloadsAndSecrets(t *testing.T) {
	for _, input := range []string{
		`{"schema_version":1,"raw_tool_output":"ghp_SYNTHETIC_SECRET"}`,
		`{"schema_version":1,"event_id":"ghp_SYNTHETIC_SECRET"}`,
		`{"event":"session_end","outcome":"sk-SYNTHETIC_SECRET"}`,
		strings.Repeat("x", MaxInput+1),
	} {
		_, err := Decode([]byte(input))
		if err == nil || strings.Contains(err.Error(), "SYNTHETIC_SECRET") {
			t.Fatal("unsafe admission/error")
		}
	}
	valid, _ := json.Marshal(fixture())
	if _, err := Decode(append(valid, []byte(` {}`)...)); err == nil {
		t.Fatal("accepted second object")
	}
	if _, err := Decode(valid); err != nil {
		t.Fatal(err)
	}
}

func TestDurableRetryConflictAndPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	s := Spool{Directory: dir}
	r, err := s.Append(context.Background(), fixture())
	if err != nil || r.State != "captured_local" || r.Origin != "unknown" || r.Coverage != "incomplete" || r.Duplicate {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = s.Append(context.Background(), fixture())
	if err != nil || !r.Duplicate {
		t.Fatalf("retry: %+v %v", r, err)
	}
	changed := fixture()
	changed.Outcome = "succeeded"
	if _, err := s.Append(context.Background(), changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflict: %v", err)
	}
	for _, path := range []string{dir, filepath.Join(dir, fixture().EventID+".json"), filepath.Join(dir, ".lock")} {
		i, err := os.Stat(path)
		if err != nil || i.Mode().Perm()&0077 != 0 {
			t.Fatalf("permissions: %v %v", i, err)
		}
	}
}

func TestConcurrentDuplicateHasOneEffect(t *testing.T) {
	s := Spool{Directory: filepath.Join(t.TempDir(), "spool")}
	var wg sync.WaitGroup
	results := make(chan Receipt, 12)
	errs := make(chan error, 12)
	for range 12 {
		wg.Go(func() { r, e := s.Append(context.Background(), fixture()); results <- r; errs <- e })
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := 0
	for r := range results {
		if !r.Duplicate {
			first++
		}
	}
	if first != 1 {
		t.Fatalf("effects=%d", first)
	}
}

func TestCapacityDeadlineAndUnsafeStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	if _, err := (Spool{Directory: dir, MaxBytes: 1}).Append(context.Background(), fixture()); !errors.Is(err, ErrCapacity) {
		t.Fatalf("capacity: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Spool{Directory: dir}).Append(ctx, fixture()); !errors.Is(err, context.Canceled) {
		t.Fatalf("deadline: %v", err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Spool{Directory: dir}).Append(context.Background(), fixture()); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("unsafe dir: %v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if _, err := (Spool{Directory: link}).Append(context.Background(), fixture()); !errors.Is(err, ErrUnsafe) {
		t.Fatalf("symlink: %v", err)
	}
}

func TestCancellationNeverProducesReceipt(t *testing.T) {
	for _, boundary := range []string{"before_write", "before_file_sync", "before_link", "before_directory_sync", "before_receipt"} {
		t.Run(boundary, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "spool")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := Spool{Directory: dir, checkpoint: func(b string) error {
				if b == boundary {
					cancel()
					return ctx.Err()
				}
				return nil
			}}
			r, err := s.Append(ctx, fixture())
			if err == nil || r.EventID != "" {
				t.Fatalf("false receipt: %+v %v", r, err)
			}
			if _, err := (Spool{Directory: dir}).Append(context.Background(), fixture()); err != nil {
				t.Fatalf("recovery: %v", err)
			}
		})
	}
}

func TestProcessExitAtDurabilityBoundaries(t *testing.T) {
	for _, boundary := range []string{"before_file_sync", "before_link", "before_directory_sync", "before_receipt"} {
		t.Run(boundary, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "spool")
			cmd := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			cmd.Env = append(os.Environ(), "GANESH_TEST_CRASH="+boundary, "GANESH_TEST_SPOOL="+dir)
			var exit *exec.ExitError
			if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 17 {
				t.Fatalf("helper did not exit at intended boundary: %v", err)
			}
			r, err := (Spool{Directory: dir}).Append(context.Background(), fixture())
			if err != nil {
				t.Fatal(err)
			}
			linked := boundary == "before_directory_sync" || boundary == "before_receipt"
			if r.Duplicate != linked {
				t.Fatalf("wrong recovered effect: %+v", r)
			}
		})
	}
}

func TestCrashHelper(t *testing.T) {
	boundary := os.Getenv("GANESH_TEST_CRASH")
	if boundary == "" {
		return
	}
	s := Spool{Directory: os.Getenv("GANESH_TEST_SPOOL"), checkpoint: func(b string) error {
		if b == boundary {
			os.Exit(17)
		}
		return nil
	}}
	_, _ = s.Append(context.Background(), fixture())
	t.Fatal("crash boundary not reached")
}
