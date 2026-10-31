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
	if path == "" {
		return errors.New("--input or referenced record path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.New("cannot open local JSON record")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil || len(body) > 1024*1024 {
		return errors.New("local JSON record unreadable or exceeds 1 MiB")
	}
	return decode(body, value)
}
