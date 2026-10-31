package capture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const MaxPendingBytes int64 = 64 * 1024 * 1024

var (
	ErrStorage  = errors.New("local capture storage failed")
	ErrUnsafe   = errors.New("local capture store must be private and regular")
	ErrConflict = errors.New("capture event identity conflicts with stored metadata")
	ErrCapacity = errors.New("local capture spool capacity reached")
	ErrCorrupt  = errors.New("stored capture record is invalid")
)

type Record struct {
	Metadata   Metadata  `json:"metadata"`
	CapturedAt time.Time `json:"captured_at"`
	State      string    `json:"state"`
	Origin     string    `json:"origin"`
	Coverage   string    `json:"coverage"`
}

type Receipt struct {
	EventID    string    `json:"event_id"`
	CapturedAt time.Time `json:"captured_at"`
	State      string    `json:"state"`
	Origin     string    `json:"origin"`
	Coverage   string    `json:"coverage"`
	Duplicate  bool      `json:"duplicate"`
}

type Spool struct {
	Directory string
	// Tests may set a smaller cap. Zero uses the declared 64 MiB bound.
	MaxBytes int64
	boundary func(string) error
}

func (s Spool) checkpoint(ctx context.Context, stage string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.boundary != nil {
		return s.boundary(stage)
	}
	return nil
}

// Append returns success only after a synced record is published without
// overwriting and its directory entries are synced. A lost receipt is resolved
// by the same metadata/identity. Durability assumes this local volume survives.
func (s Spool) Append(ctx context.Context, m Metadata) (Receipt, error) {
	var empty Receipt
	if err := m.Validate(); err != nil {
		return empty, err
	}
	if err := s.checkpoint(ctx, "before_open"); err != nil {
		return empty, err
	}
	if err := os.Mkdir(s.Directory, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return empty, ErrStorage
	}
	info, err := os.Lstat(s.Directory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return empty, ErrUnsafe
	}
	unlock, err := lock(ctx, filepath.Join(s.Directory, ".lock"))
	if err != nil {
		return empty, err
	}
	defer unlock()
	dir, err := os.Open(s.Directory)
	if err != nil {
		return empty, ErrStorage
	}
	defer dir.Close()
	parent, err := os.Open(filepath.Dir(s.Directory))
	if err != nil {
		return empty, ErrStorage
	}
	defer parent.Close()
	final := filepath.Join(s.Directory, m.EventID+".json")
	prior, err := readRecord(final)
	if err == nil {
		if prior.Metadata != m {
			return empty, ErrConflict
		}
		// Also repairs ambiguity after link but before a prior directory fsync.
		if err := s.syncPublication(ctx, dir, parent); err != nil {
			return empty, err
		}
		return receipt(prior, true), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return empty, err
	}
	stored := Record{m, time.Now().UTC(), "captured_local", "unknown", "incomplete"}
	body, err := json.Marshal(stored)
	if err != nil {
		return empty, ErrStorage
	}
	limit := s.MaxBytes
	if limit == 0 {
		limit = MaxPendingBytes
	}
	if limit < 0 || limit > MaxPendingBytes {
		return empty, ErrCapacity
	}
	used, err := pendingBytes(ctx, s.Directory)
	if err != nil {
		return empty, err
	}
	if int64(len(body)) > limit-used {
		return empty, ErrCapacity
	}
	if err := s.checkpoint(ctx, "before_write"); err != nil {
		return empty, err
	}
	f, err := os.CreateTemp(s.Directory, ".pending-*")
	if err != nil {
		return empty, ErrStorage
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(body); err != nil {
		return empty, ErrStorage
	}
	if err := s.checkpoint(ctx, "before_file_sync"); err != nil {
		return empty, err
	}
	if err := f.Sync(); err != nil {
		return empty, ErrStorage
	}
	if err := f.Close(); err != nil {
		return empty, ErrStorage
	}
	if err := s.checkpoint(ctx, "before_publish"); err != nil {
		return empty, err
	}
	if err := os.Link(f.Name(), final); err != nil {
		return empty, ErrStorage
	}
	if err := s.syncPublication(ctx, dir, parent); err != nil {
		return empty, err
	}
	return receipt(stored, false), nil
}

func (s Spool) syncPublication(ctx context.Context, dir, parent *os.File) error {
	if err := s.checkpoint(ctx, "before_directory_sync"); err != nil {
		return err
	}
	if err := dir.Sync(); err != nil {
		return ErrStorage
	}
	if err := s.checkpoint(ctx, "before_parent_sync"); err != nil {
		return err
	}
	if err := parent.Sync(); err != nil {
		return ErrStorage
	}
	return s.checkpoint(ctx, "before_receipt")
}

func receipt(r Record, duplicate bool) Receipt {
	return Receipt{r.Metadata.EventID, r.CapturedAt, r.State, r.Origin, r.Coverage, duplicate}
}

func pendingBytes(ctx context.Context, directory string) (int64, error) {
	dir, err := os.Open(directory)
	if err != nil {
		return 0, ErrStorage
	}
	defer dir.Close()
	var used int64
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		entries, err := dir.ReadDir(64)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			info, err := entry.Info()
			if err != nil {
				return 0, ErrStorage
			}
			if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
				return 0, ErrUnsafe
			}
			used += info.Size()
			if used > MaxPendingBytes {
				return used, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return used, nil
		}
		if err != nil {
			return 0, ErrStorage
		}
	}
}

func readRecord(path string) (Record, error) {
	var record Record
	f, err := openNoFollow(path, os.O_RDONLY)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return record, os.ErrNotExist
		}
		return record, ErrUnsafe
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return record, ErrStorage
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return record, ErrUnsafe
	}
	body, err := io.ReadAll(io.LimitReader(f, MaxInput+1))
	if err != nil {
		return record, ErrStorage
	}
	if len(body) > MaxInput {
		return record, ErrCorrupt
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&record) != nil || d.Decode(new(any)) != io.EOF || record.Metadata.Validate() != nil || record.CapturedAt.IsZero() || record.State != "captured_local" || record.Origin != "unknown" || record.Coverage != "incomplete" {
		return Record{}, ErrCorrupt
	}
	return record, nil
}
