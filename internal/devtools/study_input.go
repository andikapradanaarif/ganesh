package devtools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"unicode/utf8"
)

var ErrStudy = errors.New("private bounded study input required")

type artifact struct {
	Bytes  int      `json:"bytes"`
	Flags  []string `json:"flags"`
	SHA256 string   `json:"sha256"`
}

type plan struct {
	Study                 int                 `json:"study"`
	Source                string              `json:"source"`
	Baseline              string              `json:"baseline"`
	AttemptCap            int                 `json:"attempt_cap"`
	Deadline              float64             `json:"deadline_utc"`
	Seed                  int                 `json:"seed"`
	Cache                 string              `json:"cache_state"`
	Platform              string              `json:"platform"`
	Toolchain             string              `json:"toolchain"`
	Artifacts             map[string]artifact `json:"artifacts"`
	CandidateDependencies []string            `json:"candidate_dependencies"`
	ProducerDigests       map[string]string   `json:"producer_digests"`
}

type attempt struct {
	Attempt       int        `json:"attempt"`
	Stage         string     `json:"stage"`
	Arm           string     `json:"arm"`
	Category      string     `json:"category"`
	Block         int        `json:"block"`
	ParentMS      float64    `json:"parent_ms"`
	OSMS          *float64   `json:"os_to_exit_ms"`
	FinishedNS    int64      `json:"finished_ns"`
	Output        output     `json:"output_bytes"`
	Exit          *int       `json:"exit"`
	Killed        bool       `json:"killed"`
	ParentOverrun bool       `json:"parent_overrun"`
	Record        string     `json:"record"`
	Load          []float64  `json:"load"`
	IdleGap       *float64   `json:"idle_gap_s"`
	Success       bool       `json:"success"`
	LaunchFailed  bool       `json:"launch_failed"`
	TraceInvalid  bool       `json:"trace_invalid"`
	TraceNS       []int64    `json:"trace_ns"`
	StartupMS     *float64   `json:"startup_ms"`
	ReadyMS       *float64   `json:"ready_ms"`
	PhaseMS       []*float64 `json:"phase_ms"`
}

type output struct {
	Stdout int `json:"stdout"`
	Stderr int `json:"stderr"`
}

// uniqueJSON checks duplicate decoded keys at every depth, before typed decoding.
// Bounds apply to the file/line as well as this recursion, including discarded fields.
func uniqueJSON(body []byte) error {
	if !utf8.Valid(body) {
		return ErrStudy
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return ErrStudy
		}
		token, err := decoder.Token()
		if err != nil {
			return ErrStudy
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			keys := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || keys[name] {
					return ErrStudy
				}
				keys[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrStudy
		}
		end, err := decoder.Token()
		if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
			return ErrStudy
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrStudy
	}
	return nil
}

func decodeStudy(body []byte, dest any) error {
	if uniqueJSON(body) != nil || exactStudyKeys(body, reflect.TypeOf(dest)) != nil {
		return ErrStudy
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(dest) != nil {
		return ErrStudy
	}
	return nil
}

// encoding/json accepts case-folded struct keys. Archive column names are exact;
// aliases such as success/Success must not overwrite one logical scoring value.
func exactStudyKeys(body []byte, typ reflect.Type) error {
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return nil // required scoring columns are checked separately
	}
	if typ.Kind() == reflect.Pointer {
		return exactStudyKeys(body, typ.Elem())
	}
	switch typ.Kind() {
	case reflect.Struct:
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil {
			return ErrStudy
		}
		fields := make(map[string]reflect.Type)
		for i := range typ.NumField() {
			field := typ.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			fields[name] = field.Type
		}
		for name, raw := range object {
			field, exists := fields[name]
			if !exists || exactStudyKeys(raw, field) != nil {
				return ErrStudy
			}
		}
	case reflect.Map:
		var object map[string]json.RawMessage
		if json.Unmarshal(body, &object) != nil {
			return ErrStudy
		}
		for _, raw := range object {
			if exactStudyKeys(raw, typ.Elem()) != nil {
				return ErrStudy
			}
		}
	case reflect.Slice:
		var values []json.RawMessage
		if json.Unmarshal(body, &values) != nil {
			return ErrStudy
		}
		for _, raw := range values {
			if exactStudyKeys(raw, typ.Elem()) != nil {
				return ErrStudy
			}
		}
	}
	return nil
}

func privateFile(root *os.Root, name string, limit int) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, ErrStudy
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > int64(limit) {
		return nil, ErrStudy
	}
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrStudy
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm() != 0o600 {
		return nil, ErrStudy
	}
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(body) > limit {
		return nil, ErrStudy
	}
	return body, nil
}

func readStudy(directory string) (plan, []attempt, error) {
	var p plan
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return p, nil, ErrStudy
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return p, nil, ErrStudy
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o700 {
		return p, nil, ErrStudy
	}
	body, err := privateFile(root, "plan.json", 1<<20)
	if err != nil || decodeStudy(body, &p) != nil {
		return p, nil, ErrStudy
	}
	body, err = privateFile(root, "attempts.jsonl", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil, nil // an unstarted study stays incomplete
	}
	if err != nil {
		return p, nil, ErrStudy
	}
	var results []attempt
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		line := scanner.Bytes()
		var row attempt
		if len(results) >= 1000 || decodeStudy(line, &row) != nil {
			return p, nil, ErrStudy
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(line, &fields) != nil {
			return p, nil, ErrStudy
		}
		for _, key := range []string{"attempt", "stage", "arm", "category", "block", "parent_ms", "os_to_exit_ms", "exit", "record", "success", "output_bytes"} {
			v, exists := fields[key]
			if !exists || (bytes.Equal(bytes.TrimSpace(v), []byte("null")) && key != "exit" && key != "os_to_exit_ms") {
				return p, nil, ErrStudy
			}
		}
		var sizes map[string]json.RawMessage
		if json.Unmarshal(fields["output_bytes"], &sizes) != nil || len(sizes) != 2 || sizes["stdout"] == nil || sizes["stderr"] == nil || bytes.Equal(sizes["stdout"], []byte("null")) || bytes.Equal(sizes["stderr"], []byte("null")) || row.Output.Stdout < 0 || row.Output.Stderr < 0 {
			return p, nil, ErrStudy
		}
		results = append(results, row)
	}
	if scanner.Err() != nil {
		return p, nil, ErrStudy
	}
	return p, results, nil
}
