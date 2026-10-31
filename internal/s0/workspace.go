package s0

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type TargetInventory struct {
	SchemaVersion int               `json:"schema_version"`
	State         string            `json:"state"`
	Targets       []json.RawMessage `json:"targets"`
}

// Prepare creates a new private directory. It never overwrites or reuses an
// existing workspace. A failure leaves any partial directory for inspection;
// it is not reported as a completed preparation.
func Prepare(directory string, r Run) error {
	if err := os.Mkdir(directory, 0700); err != nil {
		return errors.New("cannot create new workspace; its parent must exist and destination must not exist")
	}
	if err := writePrivate(filepath.Join(directory, "targets.json"), TargetInventory{1, "not_screened", []json.RawMessage{}}); err != nil {
		return err
	}
	if err := writePrivate(filepath.Join(directory, "run.json"), r); err != nil {
		return err
	}
	return nil
}

func writePrivate(path string, value any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("cannot create private run file")
	}
	defer f.Close()
	e := json.NewEncoder(f)
	e.SetIndent("", "  ")
	if err := e.Encode(value); err != nil {
		return errors.New("cannot write private run file")
	}
	if err := f.Sync(); err != nil {
		return errors.New("cannot sync private run file")
	}
	return f.Close()
}
