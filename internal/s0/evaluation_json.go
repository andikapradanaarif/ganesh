package s0

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"syscall"
	"unicode/utf8"
)

// Evaluation input is metadata only. Reject duplicate/aliased/missing keys
// before typed decoding so no second parser can interpret a different plan.
func decodeEvaluation(body []byte, target any) error {
	if !utf8.Valid(body) {
		return ErrEvaluation
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	var unique func(int) error
	unique = func(depth int) error {
		if depth > 32 {
			return ErrEvaluation
		}
		token, err := d.Token()
		if err != nil {
			return ErrEvaluation
		}
		delim, container := token.(json.Delim)
		if !container {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return ErrEvaluation
				}
				seen[name] = true
				if err := unique(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := unique(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrEvaluation
		}
		end, err := d.Token()
		if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
			return ErrEvaluation
		}
		return nil
	}
	if unique(0) != nil {
		return ErrEvaluation
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrEvaluation
	}
	if evaluationShape(body, reflect.TypeOf(target)) != nil {
		return ErrEvaluation
	}
	d = json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(target) != nil {
		return ErrEvaluation
	}
	return nil
}

func evaluationShape(body []byte, typ reflect.Type) error {
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		// Unknown optional pointers are admitted for preparation, but the
		// definition validator rejects an unselected seed/access classification.
		if typ.Kind() == reflect.Pointer {
			return nil
		}
		return ErrEvaluation
	}
	switch typ.Kind() {
	case reflect.Pointer:
		return evaluationShape(body, typ.Elem())
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil || len(object) != typ.NumField() {
			return ErrEvaluation
		}
		for i := range typ.NumField() {
			field := typ.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			raw, exists := object[name]
			if !exists || evaluationShape(raw, field.Type) != nil {
				return ErrEvaluation
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if json.Unmarshal(body, &values) != nil {
			return ErrEvaluation
		}
		for _, raw := range values {
			if evaluationShape(raw, typ.Elem()) != nil {
				return ErrEvaluation
			}
		}
	}
	return nil
}

func readEvaluation(path string, target any) error {
	// O_NONBLOCK also prevents a substituted FIFO from stalling admission.
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return ErrEvaluation
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > 1<<20 {
		return ErrEvaluation
	}
	body, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		return ErrEvaluation
	}
	return decodeEvaluation(body, target)
}

func ReadEvaluationDefinition(path string) (EvaluationDefinition, error) {
	var d EvaluationDefinition
	err := readEvaluation(path, &d)
	return d, err
}

func ReadEvaluationSnapshot(path string) (EvaluationSnapshot, error) {
	var s EvaluationSnapshot
	err := readEvaluation(path, &s)
	return s, err
}
