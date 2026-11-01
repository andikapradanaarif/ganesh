package crush

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// Generate large discarded content without assembling it in the test reader.
type repeatedByte struct{ remaining int }

func (r *repeatedByte) Read(b []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(b), r.remaining)
	for i := range n {
		b[i] = 'x'
	}
	r.remaining -= n
	return n, nil
}

func largeResult(size int) io.Reader {
	body := resultBody(false)
	cut := strings.LastIndex(body, "ghp_SYNTHETIC_SECRET")
	return io.MultiReader(strings.NewReader(body[:cut]), &repeatedByte{size}, strings.NewReader(body[cut:]))
}

func TestLargeDiscardedFields(t *testing.T) {
	for _, field := range []string{"tool_input", "tool_response", "error", "cwd", "tool_name"} {
		var body string
		switch field {
		case "tool_response":
			body = resultBody(false)
		case "error":
			body = resultBody(true)
		default:
			body = attempt
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(body), &fields) != nil {
			t.Fatal("invalid test fixture")
		}
		text := strings.Repeat("x", 128*1024) + "ghp_SYNTHETIC_SECRET"
		value := any(text)
		if field == "tool_input" {
			value = map[string]any{"nested": []any{nil, true, false, -12.5e2, map[string]any{"private": text}}}
		}
		encoded, _ := json.Marshal(value)
		fields[field] = encoded
		large, _ := json.Marshal(fields)
		want, err := Reduce([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []io.Reader{strings.NewReader(string(large)), iotest.OneByteReader(strings.NewReader(string(large)))} {
			got, err := ReduceStream(context.Background(), input)
			if err != nil || got != want {
				t.Fatalf("large discarded %s: metadata=%+v error=%v", field, got, err)
			}
		}
	}
}

func TestNativeByteBoundAndEOF(t *testing.T) {
	base, _ := Reduce([]byte(resultBody(false)))
	// largeResult adds exactly size bytes to the original envelope.
	for _, extra := range []int{-1, 0, 1} {
		size := MaxNativeInput - len(resultBody(false)) + extra
		m, err := ReduceStream(context.Background(), largeResult(size))
		if extra <= 0 {
			if err != nil || m != base {
				t.Fatalf("within raw cap: %+v %v", m, err)
			}
		} else if !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) {
			t.Fatalf("over raw cap admitted: %+v %v", m, err)
		}
	}
	for _, suffix := range []string{`{}`, ` null`, "\x00", ` garbage`} {
		if _, err := ReduceStream(context.Background(), io.MultiReader(largeResult(128*1024), strings.NewReader(suffix))); !errors.Is(err, ErrPayload) {
			t.Fatal("trailing content admitted")
		}
	}
	// EOF may accompany the final bytes, or arrive on a later read.
	if m, err := ReduceStream(context.Background(), iotest.DataErrReader(strings.NewReader(attempt))); err != nil || m.Event != "tool_attempt" {
		t.Fatal("data plus EOF rejected")
	}
	if _, err := ReduceStream(context.Background(), io.MultiReader(strings.NewReader(attempt), iotest.ErrReader(errors.New("ghp_SYNTHETIC_SECRET")))); !errors.Is(err, ErrPayload) {
		t.Fatal("reader failure mistaken for EOF")
	}
}

func TestDiscardedJSONGrammar(t *testing.T) {
	const input = `{"file_path":"/synthetic/private","content":"ghp_SYNTHETIC_SECRET"}`
	for _, value := range []string{
		`{}`, `{"x":null}`, `{"x":true,"y":false}`, `{"x":-0.1e+12}`, `{"x":1E-2}`,
		`{"x":[]}`, `{"x":{"y":[1,"ok",{},[]]}}`, `{"x":"😺é\u263a\n\t\\\"\/"}`,
		`{"x":"\ud800"}`, `{"duplicate":1,"duplicate":2}`,
	} {
		if !json.Valid([]byte(value)) {
			t.Fatal("invalid expected-valid oracle input")
		}
		body := strings.Replace(attempt, input, value, 1)
		if _, err := ReduceStream(context.Background(), iotest.OneByteReader(strings.NewReader(body))); err != nil {
			t.Fatalf("valid discarded JSON rejected: %v", err)
		}
	}
	for _, value := range []string{
		`{"x":01}`, `{"x":-}`, `{"x":.1}`, `{"x":1.}`, `{"x":1e}`, `{"x":1e+}`,
		`{"x":+1}`, `{"x":tru}`, `{"x":NaN}`, `{"x":Infinity}`, `{"x":[1,]}`,
		`{"x":1,}`, `{x:1}`, `{"x" 1}`, `{"x":1 "y":2}`, `{"x":1true}`,
		`{"x":"\x"}`, `{"x":"\u00X0"}`, `{"x":"\u000"}`, `{"x":"unterminated}`,
		"{\"x\":\"\n\"}", "{\"x\":\"\xff\"}", "{\"x\":\"\xc0\xaf\"}", "{\"x\":\"\xed\xa0\x80\"}",
		"{\"x\":\"\xf4\x90\x80\x80\"}",
	} {
		body := strings.Replace(attempt, input, value, 1)
		if m, err := ReduceStream(context.Background(), iotest.OneByteReader(strings.NewReader(body))); !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) {
			t.Fatalf("invalid JSON admitted: %+v %v", m, err)
		}
	}
	for _, depth := range []int{MaxDepth - 2, MaxDepth - 1} {
		// Root and tool_input contribute two container levels already.
		value := `{"x":` + strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth) + "}"
		_, err := Reduce([]byte(strings.Replace(attempt, input, value, 1)))
		if (err == nil) != (depth == MaxDepth-2) {
			t.Fatalf("container depth boundary: %d %v", depth, err)
		}
	}
}

func TestStreamCancellationAndControls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if m, err := ReduceStream(ctx, strings.NewReader(attempt)); !errors.Is(err, ErrPayload) || m != (capture.Metadata{}) {
		t.Fatal("cancelled stream admitted")
	}
	for _, body := range []string{
		strings.Replace(attempt, `"event":`, `"\u0065vent":`, 1),
		strings.Replace(attempt, `"call-1"`, `"\u0063all-1"`, 1),
	} {
		got, err := Reduce([]byte(body))
		want, _ := Reduce([]byte(attempt))
		if err != nil || got != want {
			t.Fatal("escaped control identity changed")
		}
	}
	for _, body := range []string{
		strings.Replace(attempt, `"event":`, `"event":"PreToolUse","\u0065vent":`, 1),
		strings.Replace(attempt, `"event"`, `"`+strings.Repeat("x", maxFieldName)+`"`, 1),
		strings.Replace(attempt, `"call-1"`, `"`+strings.Repeat("x", maxControl)+`"`, 1),
	} {
		if _, err := Reduce([]byte(body)); !errors.Is(err, ErrPayload) {
			t.Fatal("control bound/duplicate admitted")
		}
	}
}

// Independent standard-library syntax/UTF-8 oracle for the custom skipper.
// Mutations affect only the discarded tool_input object, not protected fields.
func FuzzDiscardedToolInput(f *testing.F) {
	for _, value := range []string{`{}`, `{"x":1}`, `{"x":"a\\\"b"}`, `{"x":[true,null,-1e2]}`, `{"x":01}`, `{"x":"😺"}`} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 16*1024 {
			t.Skip()
		}
		body := strings.Replace(attempt, `{"file_path":"/synthetic/private","content":"ghp_SYNTHETIC_SECRET"}`, value, 1)
		_, err := Reduce([]byte(body))
		if err == nil && (!json.Valid([]byte(body)) || !utf8.ValidString(body)) {
			t.Fatal("non-JSON or non-UTF-8 input accepted")
		}
		trimmed := strings.TrimSpace(value)
		if json.Valid([]byte(value)) && utf8.ValidString(value) && strings.HasPrefix(trimmed, "{") {
			// A standalone valid object cannot escape into the parent envelope.
			// Count its containers with the independent stdlib token decoder.
			d := json.NewDecoder(strings.NewReader(value))
			depth, maxDepth := 1, 1 // surrounding envelope already contributes one
			for {
				token, tokenErr := d.Token()
				if tokenErr == io.EOF {
					break
				}
				if tokenErr != nil {
					t.Fatal("standard syntax oracle disagrees with json.Valid")
				}
				if delimiter, ok := token.(json.Delim); ok {
					if delimiter == '{' || delimiter == '[' {
						depth++
						maxDepth = max(maxDepth, depth)
					} else {
						depth--
					}
				}
			}
			if (err == nil) != (maxDepth <= MaxDepth) {
				t.Fatal("streaming acceptance disagrees with independent object/depth oracle")
			}
		}
	})
}

func BenchmarkReduceStream(b *testing.B) {
	for _, size := range []int{1024, 128 * 1024, 1024 * 1024} {
		b.Run(fmtSize(size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(size + len(resultBody(false))))
			for range b.N {
				if _, err := ReduceStream(context.Background(), largeResult(size)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func fmtSize(n int) string {
	switch n {
	case 1024:
		return "1KiB"
	case 128 * 1024:
		return "128KiB"
	default:
		return "1MiB"
	}
}
