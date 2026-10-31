package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"unicode/utf8"
)

func decode(body []byte, value any) error {
	if !utf8.Valid(body) {
		return errors.New("invalid local JSON record encoding")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid local JSON record")
	}
	return nil
}

func readRecord(path string, value any) error {
	_, err := readRecordInfo(path, value)
	return err
}

// Inspect the opened descriptor so a separate path lookup cannot inspect a
// different file from the one parsed. This checks mode bits, not ACLs.
func readRecordInfo(path string, value any) (os.FileInfo, error) {
	if path == "" {
		return nil, errors.New("--input or referenced record path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("cannot open local JSON record")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, errors.New("cannot inspect local JSON record metadata")
	}
	body, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return nil, errors.New("local JSON record unreadable or exceeds 1 MiB")
	}
	if err := decode(body, value); err != nil {
		return nil, err
	}
	return info, nil
}
