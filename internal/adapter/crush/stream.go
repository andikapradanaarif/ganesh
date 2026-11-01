package crush

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"unicode/utf8"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

// These bounds apply only to the raw native envelope. The normalized capture
// schema and its 64 KiB admission limit are unchanged.
const (
	MaxNativeInput = 4 * 1024 * 1024
	MaxDepth       = 64
	maxControl     = 1024 // encoded bytes, including string quotes
	maxFieldName   = 256
)

const (
	eventField = iota
	aliasField
	sessionField
	callField
	cwdField
	nameField
	inputField
	responseField
	errorField
)

var nativeFields = map[string]int{
	"event": eventField, "hook_event_name": aliasField,
	"session_id": sessionField, "tool_use_id": callField,
	"cwd": cwdField, "tool_name": nameField, "tool_input": inputField,
	"tool_response": responseField, "error": errorField,
}

// ReduceStream validates a complete native JSON envelope and discards content
// while reading it. Large strings and nested values are never assembled in
// memory. Errors never expose parser diagnostics, reader errors or input bytes.
// Callers must bound blocked reads; the CLI uses its original process deadline.
func ReduceStream(ctx context.Context, input io.Reader) (capture.Metadata, error) {
	bounded := &nativeReader{ctx: ctx, input: io.LimitReader(input, MaxNativeInput+1)}
	p := nativeParser{input: bufio.NewReaderSize(bounded, 4096)}
	values, seen, err := p.envelope()
	if err != nil || bounded.read > MaxNativeInput || ctx.Err() != nil {
		return capture.Metadata{}, ErrPayload
	}
	return nativeMetadata(values, seen)
}

type nativeReader struct {
	ctx   context.Context
	input io.Reader
	read  int
}

func (r *nativeReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.input.Read(b)
	r.read += n
	return n, err
}

type nativeParser struct{ input *bufio.Reader }

func (p *nativeParser) peek() (byte, error) {
	b, err := p.input.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (p *nativeParser) space() (byte, error) {
	for {
		b, err := p.peek()
		if err != nil || !(b == ' ' || b == '\t' || b == '\n' || b == '\r') {
			return b, err
		}
		_, _ = p.input.ReadByte()
	}
}

func (p *nativeParser) expect(want byte) error {
	b, err := p.input.ReadByte()
	if err != nil || b != want {
		return ErrPayload
	}
	return nil
}

func (p *nativeParser) envelope() ([4]string, uint16, error) {
	var values [4]string
	var seen uint16
	if b, err := p.space(); err != nil || b != '{' || p.expect('{') != nil {
		return values, seen, ErrPayload
	}
	for {
		if b, err := p.space(); err != nil || b != '"' {
			return values, seen, ErrPayload
		}
		key, _, err := p.stringValue(true, maxFieldName)
		field, known := nativeFields[key]
		if err != nil || !known || seen&(1<<field) != 0 {
			return values, seen, ErrPayload
		}
		seen |= 1 << field
		if _, err := p.space(); err != nil || p.expect(':') != nil {
			return values, seen, ErrPayload
		}
		b, err := p.space()
		if err != nil {
			return values, seen, ErrPayload
		}
		if field == inputField {
			if b != '{' || p.value(2) != nil {
				return values, seen, ErrPayload
			}
		} else {
			if b != '"' {
				return values, seen, ErrPayload
			}
			value, nonempty, err := p.stringValue(field <= callField, maxControl)
			if err != nil || (!nonempty && field != responseField && field != errorField) {
				return values, seen, ErrPayload
			}
			if field <= callField {
				values[field] = value
			}
		}
		b, err = p.space()
		if err != nil {
			return values, seen, ErrPayload
		}
		if b == '}' {
			_, _ = p.input.ReadByte()
			break
		}
		if p.expect(',') != nil {
			return values, seen, ErrPayload
		}
	}
	if _, err := p.space(); err != io.EOF {
		return values, seen, ErrPayload
	}
	return values, seen, nil
}

// stringValue retains only small control strings when keep is true. For
// discarded strings it validates escapes and UTF-8 using fixed scratch space.
// Like encoding/json v1, escaped lone surrogates are syntactically admitted;
// control-string decoding uses encoding/json's replacement semantics.
func (p *nativeParser) stringValue(keep bool, limit int) (string, bool, error) {
	if p.expect('"') != nil {
		return "", false, ErrPayload
	}
	var raw []byte
	if keep {
		raw = make([]byte, 1, 64)
		raw[0] = '"'
	}
	appendRaw := func(b []byte) bool {
		if !keep {
			return true
		}
		if len(raw)+len(b) > limit {
			return false
		}
		raw = append(raw, b...)
		return true
	}
	nonempty := false
	var scratch [6]byte
	for {
		b, err := p.input.ReadByte()
		if err != nil || b < 0x20 {
			return "", false, ErrPayload
		}
		scratch[0] = b
		if b == '"' {
			if !appendRaw(scratch[:1]) {
				return "", false, ErrPayload
			}
			if !keep {
				return "", nonempty, nil
			}
			var value string
			if json.Unmarshal(raw, &value) != nil {
				return "", false, ErrPayload
			}
			return value, nonempty, nil
		}
		nonempty = true
		n := 1
		if b == '\\' {
			escape, err := p.input.ReadByte()
			if err != nil {
				return "", false, ErrPayload
			}
			scratch[1], n = escape, 2
			switch escape {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for n < 6 {
					c, err := p.input.ReadByte()
					if err != nil || !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
						return "", false, ErrPayload
					}
					scratch[n], n = c, n+1
				}
			default:
				return "", false, ErrPayload
			}
		} else if b >= utf8.RuneSelf {
			switch {
			case b >= 0xc2 && b <= 0xdf:
				n = 2
			case b >= 0xe0 && b <= 0xef:
				n = 3
			case b >= 0xf0 && b <= 0xf4:
				n = 4
			default:
				return "", false, ErrPayload
			}
			for i := 1; i < n; i++ {
				scratch[i], err = p.input.ReadByte()
				if err != nil {
					return "", false, ErrPayload
				}
			}
			if !utf8.Valid(scratch[:n]) {
				return "", false, ErrPayload
			}
		}
		if !appendRaw(scratch[:n]) {
			return "", false, ErrPayload
		}
	}
}

func (p *nativeParser) value(depth int) error {
	b, err := p.space()
	if err != nil {
		return ErrPayload
	}
	switch b {
	case '"':
		_, _, err := p.stringValue(false, 0)
		return err
	case '{', '[':
		if depth > MaxDepth {
			return ErrPayload
		}
		_, _ = p.input.ReadByte()
		end := byte('}')
		if b == '[' {
			end = ']'
		}
		if next, err := p.space(); err == nil && next == end {
			return p.expect(end)
		}
		for {
			if b == '{' {
				if _, err := p.space(); err != nil {
					return ErrPayload
				}
				if _, _, err := p.stringValue(false, 0); err != nil {
					return err
				}
				if _, err := p.space(); err != nil || p.expect(':') != nil {
					return ErrPayload
				}
			}
			if p.value(depth+1) != nil {
				return ErrPayload
			}
			next, err := p.space()
			if err != nil {
				return ErrPayload
			}
			if next == end {
				return p.expect(end)
			}
			if p.expect(',') != nil {
				return ErrPayload
			}
		}
	case 't', 'f', 'n':
		literal := map[byte]string{'t': "true", 'f': "false", 'n': "null"}[b]
		for i := range len(literal) {
			if p.expect(literal[i]) != nil {
				return ErrPayload
			}
		}
		return nil
	default:
		return p.number()
	}
}

func (p *nativeParser) number() error {
	b, _ := p.peek()
	if b == '-' {
		_, _ = p.input.ReadByte()
		b, _ = p.peek()
	}
	if b == '0' {
		_, _ = p.input.ReadByte()
	} else if !p.digits() {
		return ErrPayload
	}
	b, _ = p.peek()
	if b == '.' {
		_, _ = p.input.ReadByte()
		if !p.digits() {
			return ErrPayload
		}
	}
	b, _ = p.peek()
	if b == 'e' || b == 'E' {
		_, _ = p.input.ReadByte()
		if b, _ := p.peek(); b == '+' || b == '-' {
			_, _ = p.input.ReadByte()
		}
		if !p.digits() {
			return ErrPayload
		}
	}
	return nil
}

func (p *nativeParser) digits() bool {
	found := false
	for {
		b, err := p.peek()
		if err != nil || b < '0' || b > '9' {
			return found
		}
		found = true
		_, _ = p.input.ReadByte()
	}
}
