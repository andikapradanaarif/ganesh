// Package captureipc implements an experimental local metadata receipt wire
// contract. It authenticates neither peers nor storage and starts no worker.
package captureipc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
	"unicode/utf8"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

const MaxFrame = 4096

var (
	ErrFrame    = errors.New("invalid local capture frame")
	ErrExchange = errors.New("local capture exchange failed")
)

func encodeMetadata(m capture.Metadata) ([]byte, error) {
	if m.Validate() != nil {
		return nil, ErrFrame
	}
	b, err := json.Marshal(m)
	if err != nil || len(b) > MaxFrame {
		return nil, ErrFrame
	}
	return b, nil
}

func requestDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func validReceipt(m capture.Metadata, r capture.Receipt) bool {
	return r.EventID == m.EventID && !r.CapturedAt.IsZero() && r.State == "captured_local" && r.Origin == "unknown" && r.Coverage == "incomplete"
}

// EncodeReceipt encodes a structurally valid peer declaration for one request.
// It does not establish that the peer actually stored anything durably.
func EncodeReceipt(m capture.Metadata, r capture.Receipt) ([]byte, error) {
	request, err := encodeMetadata(m)
	if err != nil || !validReceipt(m, r) {
		return nil, ErrFrame
	}
	r.CapturedAt = r.CapturedAt.UTC()
	b, err := json.Marshal(struct {
		SchemaVersion int    `json:"schema_version"`
		RequestSHA256 string `json:"request_sha256"`
		capture.Receipt
	}{1, requestDigest(request), r})
	if err != nil || len(b) > MaxFrame {
		return nil, ErrFrame
	}
	return b, nil
}

func decodeReceipt(body []byte, m capture.Metadata, digest string) (capture.Receipt, error) {
	var r capture.Receipt
	if len(body) == 0 || len(body) > MaxFrame || !utf8.Valid(body) {
		return r, ErrFrame
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return r, ErrFrame
	}
	seen := make(map[string]bool, 8)
	for d.More() {
		token, err := d.Token()
		key, ok := token.(string)
		if err != nil || !ok || seen[key] {
			return capture.Receipt{}, ErrFrame
		}
		seen[key] = true
		value, err := d.Token()
		if err != nil {
			return capture.Receipt{}, ErrFrame
		}
		text, textOK := value.(string)
		switch key {
		case "schema_version":
			if value != json.Number("1") {
				return capture.Receipt{}, ErrFrame
			}
		case "request_sha256":
			if !textOK || text != digest {
				return capture.Receipt{}, ErrFrame
			}
		case "event_id":
			if !textOK {
				return capture.Receipt{}, ErrFrame
			}
			r.EventID = text
		case "captured_at":
			if !textOK {
				return capture.Receipt{}, ErrFrame
			}
			r.CapturedAt, err = time.Parse(time.RFC3339Nano, text)
			if err != nil || r.CapturedAt.UTC().Format(time.RFC3339Nano) != text {
				return capture.Receipt{}, ErrFrame
			}
		case "state":
			if !textOK {
				return capture.Receipt{}, ErrFrame
			}
			r.State = text
		case "origin":
			if !textOK {
				return capture.Receipt{}, ErrFrame
			}
			r.Origin = text
		case "coverage":
			if !textOK {
				return capture.Receipt{}, ErrFrame
			}
			r.Coverage = text
		case "duplicate":
			var ok bool
			r.Duplicate, ok = value.(bool)
			if !ok {
				return capture.Receipt{}, ErrFrame
			}
		default:
			return capture.Receipt{}, ErrFrame
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') || d.Decode(new(any)) != io.EOF || len(seen) != 8 || !validReceipt(m, r) {
		return capture.Receipt{}, ErrFrame
	}
	return r, nil
}

func readFrame(input io.Reader) ([]byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return nil, ErrExchange
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > MaxFrame {
		return nil, ErrFrame
	}
	body := make([]byte, int(n))
	if _, err := io.ReadFull(input, body); err != nil {
		return nil, ErrExchange
	}
	return body, nil
}

func writeFrame(output io.Writer, body []byte) error {
	if len(body) == 0 || len(body) > MaxFrame {
		return ErrFrame
	}
	packet := make([]byte, 4+len(body))
	binary.BigEndian.PutUint32(packet[:4], uint32(len(body)))
	copy(packet[4:], body)
	for len(packet) > 0 {
		n, err := output.Write(packet)
		if err != nil || n <= 0 || n > len(packet) {
			return ErrExchange
		}
		packet = packet[n:]
	}
	return nil
}
