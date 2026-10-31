package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

func decode(body []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return errors.New("invalid run record JSON")
	}
	return nil
}
