//go:build darwin || linux

package capture

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestSpoolDurabilityIdentityAndPrivateRecords(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	s := Spool{Directory: dir}
	first, err := s.Append(context.Background(), fixtureMetadata())
	if err != nil || first.State != "captured_local" || first.Origin != "unknown" || first.Coverage != "incomplete" || first.Duplicate {
		t.Fatalf("capture: %+v %v", first, err)
	}
	path := filepath.Join(dir, fixtureMetadata().EventID+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Append(context.Background(), fixtureMetadata())
	if err != nil || !second.Duplicate || !second.CapturedAt.Equal(first.CapturedAt) {
		t.Fatalf("retry: %+v %v", second, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("retry changed record")
	}
	changed := fixtureMetadata()
	changed.Outcome = "succeeded"
	if receipt, err := s.Append(context.Background(), changed); !errors.Is(err, ErrConflict) || receipt.EventID != "" {
		t.Fatalf("conflict: %+v %v", receipt, err)
	}
	for _, path := range []string{dir, path, filepath.Join(dir, ".lock")} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("privacy: %s %v", path, err)
		}
	}
}

func TestConcurrentDuplicatesHaveOneEffect(t *testing.T) {
	s := Spool{Directory: filepath.Join(t.TempDir(), "spool")}
	var group sync.WaitGroup
	results := make(chan Receipt, 12)
	errs := make(chan error, 12)
	for range 12 {
		group.Go(func() { r, e := s.Append(context.Background(), fixtureMetadata()); results <- r; errs <- e })
	}
	group.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	created := 0
	for result := range results {
		if !result.Duplicate {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created %d effects", created)
	}
}

func TestCapacityAndCorruptionDoNotIssueReceipts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	for _, cap := range []int64{1, -1, MaxPendingBytes + 1} {
		if r, err := (Spool{Directory: dir, MaxBytes: cap}).Append(context.Background(), fixtureMetadata()); !errors.Is(err, ErrCapacity) || r.EventID != "" {
			t.Fatalf("capacity: %+v %v", r, err)
		}
	}
	s := Spool{Directory: dir}
	first, err := s.Append(context.Background(), fixtureMetadata())
	if err != nil {
		t.Fatal(err)
	}
	// Recovery of an accepted identity remains possible when capacity is full.
	if r, err := (Spool{Directory: dir, MaxBytes: 1}).Append(context.Background(), fixtureMetadata()); err != nil || !r.Duplicate || r.CapturedAt != first.CapturedAt {
		t.Fatalf("full-spool duplicate: %+v %v", r, err)
	}
	path := filepath.Join(dir, fixtureMetadata().EventID+".json")
	if err := os.WriteFile(path, []byte(`{"raw":"ghp_SYNTHETIC_SECRET"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Append(context.Background(), fixtureMetadata()); !errors.Is(err, ErrCorrupt) || r.EventID != "" {
		t.Fatalf("corrupt: %+v %v", r, err)
	}
}

func TestUnsafeStoresAndRecordsFailClosed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := (Spool{Directory: dir}).Append(context.Background(), fixtureMetadata()); !errors.Is(err, ErrUnsafe) {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := (Spool{Directory: alias}).Append(context.Background(), fixtureMetadata()); !errors.Is(err, ErrUnsafe) {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(external, []byte("fixture-do-not-read"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, fixtureMetadata().EventID+".json")
	if err := os.Symlink(external, path); err != nil {
		t.Fatal(err)
	}
	if _, err := (Spool{Directory: dir}).Append(context.Background(), fixtureMetadata()); !errors.Is(err, ErrUnsafe) {
		t.Fatal(err)
	}
}

func TestLockWaitHonorsCancellation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	unlock, err := lock(context.Background(), filepath.Join(dir, ".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	r, err := (Spool{Directory: dir}).Append(ctx, fixtureMetadata())
	if !errors.Is(err, context.DeadlineExceeded) || r.EventID != "" {
		t.Fatalf("lock wait claimed capture: %+v %v", r, err)
	}
}

func TestPartialTemporaryWriteIsNotACommittedRecord(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spool")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(dir, ".pending-interrupted")
	if err := os.WriteFile(orphan, []byte(`{"metadata":`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := (Spool{Directory: dir}).Append(context.Background(), fixtureMetadata())
	if err != nil || r.Duplicate {
		t.Fatalf("partial temporary record was treated as accepted: %+v %v", r, err)
	}
	if _, err := os.Stat(orphan); err != nil {
		t.Fatal("prototype silently removed an orphan")
	}
}

func TestCancellationAtEveryBoundaryNeverClaimsCapture(t *testing.T) {
	for _, stage := range []string{"before_open", "before_write", "before_file_sync", "before_publish", "before_directory_sync", "before_parent_sync", "before_receipt"} {
		t.Run(stage, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "spool")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := Spool{Directory: dir, boundary: func(at string) error {
				if at == stage {
					cancel()
					return ctx.Err()
				}
				return nil
			}}
			r, err := s.Append(ctx, fixtureMetadata())
			if !errors.Is(err, context.Canceled) || r.EventID != "" {
				t.Fatalf("false receipt: %+v %v", r, err)
			}
			if _, err := (Spool{Directory: dir}).Append(context.Background(), fixtureMetadata()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRestartAfterProcessExitAtCommitBoundaries(t *testing.T) {
	for _, stage := range []string{"before_file_sync", "before_publish", "before_directory_sync", "before_parent_sync", "before_receipt"} {
		t.Run(stage, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "spool")
			cmd := exec.Command(os.Args[0], "-test.run=^TestCaptureCrashHelper$")
			cmd.Env = append(os.Environ(), "GANESH_CAPTURE_TEST_CRASH="+stage, "GANESH_CAPTURE_TEST_DIRECTORY="+dir)
			var exit *exec.ExitError
			if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 17 {
				t.Fatalf("did not reach crash boundary: %v", err)
			}
			r, err := (Spool{Directory: dir}).Append(context.Background(), fixtureMetadata())
			if err != nil {
				t.Fatal(err)
			}
			published := stage == "before_directory_sync" || stage == "before_parent_sync" || stage == "before_receipt"
			if r.Duplicate != published {
				t.Fatalf("restart identity: %+v", r)
			}
		})
	}
}

func TestCaptureCrashHelper(t *testing.T) {
	stage := os.Getenv("GANESH_CAPTURE_TEST_CRASH")
	if stage == "" {
		return
	}
	s := Spool{Directory: os.Getenv("GANESH_CAPTURE_TEST_DIRECTORY"), boundary: func(at string) error {
		if at == stage {
			os.Exit(17)
		}
		return nil
	}}
	_, _ = s.Append(context.Background(), fixtureMetadata())
	t.Fatal("crash boundary was not reached")
}
