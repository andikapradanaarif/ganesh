package capture

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrConflict = errors.New("event identity conflicts with stored metadata")
	ErrCapacity = errors.New("local spool capacity reached")
	ErrUnsafe   = errors.New("local spool is not a private regular store")
)

type Receipt struct {
	EventID   string `json:"event_id"`
	State     string `json:"state"`
	Origin    string `json:"origin"`
	Coverage  string `json:"coverage"`
	Duplicate bool   `json:"duplicate"`
}

type record struct {
	Metadata   Metadata  `json:"metadata"`
	CapturedAt time.Time `json:"captured_at"`
	State      string    `json:"state"`
	Origin     string    `json:"origin"`
	Coverage   string    `json:"coverage"`
}

type Spool struct {
	Directory string
	// Zero selects the initial 64 MiB pending limit.
	MaxBytes int64
	// checkpoint is a test-only fault boundary, never an external call.
	checkpoint func(string) error
}

func (s Spool) check(ctx context.Context, boundary string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.checkpoint != nil {
		return s.checkpoint(boundary)
	}
	return nil
}

// Append promises process-restart durability with this volume intact, not
// remote durability. A missing receipt may leave a committed event; retry the
// same normalized envelope to recover it. It never claims clean coverage.
func (s Spool) Append(ctx context.Context, m Metadata) (Receipt, error) {
	var empty Receipt
	// Validate even internal callers; filenames cannot be caller paths.
	encoded, _ := json.Marshal(m)
	if _, err := Decode(encoded); err != nil {
		return empty, err
	}
	if err := s.check(ctx, "before_open"); err != nil {
		return empty, err
	}
	if err := os.MkdirAll(s.Directory, 0700); err != nil {
		return empty, errors.New("cannot create local spool")
	}
	info, err := os.Lstat(s.Directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return empty, ErrUnsafe
	}
	unlock, err := lock(ctx, filepath.Join(s.Directory, ".lock"))
	if err != nil {
		return empty, err
	}
	defer unlock()
	dir, err := os.Open(s.Directory)
	if err != nil {
		return empty, errors.New("cannot open spool directory")
	}
	defer dir.Close()
	// Persist newly created directory entries up to the first existing parent.
	// The helper also covers retry after a prior ambiguous creation attempt.
	if err := syncAncestors(s.Directory); err != nil {
		return empty, err
	}
	final := filepath.Join(s.Directory, m.EventID+".json")
	receipt := Receipt{EventID: m.EventID, State: "captured_local", Origin: "unknown", Coverage: "incomplete"}
	if old, err := os.Lstat(final); err == nil {
		if !old.Mode().IsRegular() || old.Mode().Perm()&0077 != 0 {
			return empty, ErrUnsafe
		}
		if old.Size() > MaxInput {
			return empty, errors.New("stored spool record exceeds limit")
		}
		body, readErr := os.ReadFile(final)
		if readErr != nil || len(body) > MaxInput {
			return empty, errors.New("cannot recover spool record")
		}
		var prior record
		if json.Unmarshal(body, &prior) != nil || prior.Metadata != m {
			return empty, ErrConflict
		}
		// Recover an acknowledgment lost after link but before directory fsync.
		if err := dir.Sync(); err != nil {
			return empty, errors.New("cannot sync spool directory")
		}
		if err := s.check(ctx, "before_receipt"); err != nil {
			return empty, err
		}
		receipt.Duplicate = true
		return receipt, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return empty, errors.New("cannot inspect spool record")
	}
	entries, err := os.ReadDir(s.Directory)
	if err != nil {
		return empty, errors.New("cannot inspect spool capacity")
	}
	var used int64
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		i, err := e.Info()
		if err != nil {
			return empty, errors.New("cannot inspect spool capacity")
		}
		used += i.Size()
	}
	max := s.MaxBytes
	if max == 0 {
		max = 64 * 1024 * 1024
	}
	body, err := json.Marshal(record{m, time.Now().UTC(), "captured_local", "unknown", "incomplete"})
	if err != nil {
		return empty, errors.New("cannot encode local metadata")
	}
	if max < 0 || used+int64(len(body)) > max {
		return empty, ErrCapacity
	}
	if err := s.check(ctx, "before_write"); err != nil {
		return empty, err
	}
	f, err := os.CreateTemp(s.Directory, ".pending-*")
	if err != nil {
		return empty, errors.New("cannot create spool record")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(body); err != nil {
		return empty, errors.New("cannot write spool record")
	}
	if err := s.check(ctx, "before_file_sync"); err != nil {
		return empty, err
	}
	if err := f.Sync(); err != nil {
		return empty, errors.New("cannot sync spool record")
	}
	if err := f.Close(); err != nil {
		return empty, errors.New("cannot close spool record")
	}
	if err := s.check(ctx, "before_link"); err != nil {
		return empty, err
	}
	// A hard link publishes the already fsynced inode without overwriting.
	if err := os.Link(f.Name(), final); err != nil {
		return empty, errors.New("cannot publish spool record")
	}
	if err := s.check(ctx, "before_directory_sync"); err != nil {
		return empty, err
	}
	if err := dir.Sync(); err != nil {
		return empty, errors.New("cannot sync spool directory")
	}
	if err := s.check(ctx, "before_receipt"); err != nil {
		return empty, err
	}
	return receipt, nil
}

func syncAncestors(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return errors.New("cannot resolve spool directory")
	}
	for current := filepath.Dir(abs); ; current = filepath.Dir(current) {
		f, err := os.Open(current)
		if err != nil {
			return errors.New("cannot open spool parent")
		}
		err = f.Sync()
		_ = f.Close()
		if err != nil {
			return errors.New("cannot sync spool parent")
		}
		if current == filepath.Dir(current) {
			break
		}
	}
	return nil
}
