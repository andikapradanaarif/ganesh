package captureipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/capture"
)

func testMetadata() capture.Metadata {
	return capture.Metadata{SchemaVersion: 1, EventID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000002", Event: "tool_result", Outcome: "succeeded"}
}

func testReceipt() capture.Receipt {
	return capture.Receipt{EventID: testMetadata().EventID, CapturedAt: time.Unix(1, 123).UTC(), State: "captured_local", Origin: "unknown", Coverage: "incomplete"}
}

func TestFrameAndReceiptRoundTrip(t *testing.T) {
	m := testMetadata()
	request, err := encodeMetadata(m)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := capture.Decode(request)
	if err != nil || decoded != m {
		t.Fatal("metadata changed")
	}
	for _, duplicate := range []bool{false, true} {
		r := testReceipt()
		r.Duplicate = duplicate
		body, err := EncodeReceipt(m, r)
		if err != nil {
			t.Fatal(err)
		}
		var wire bytes.Buffer
		if writeFrame(oneByteWriter{&wire}, body) != nil {
			t.Fatal("frame write")
		}
		payload, err := readFrame(iotest.OneByteReader(&wire))
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeReceipt(payload, m, requestDigest(request))
		if err != nil || got != r {
			t.Fatal("receipt changed")
		}
	}
	// Digest covers the complete semantic request, even with the same event ID.
	changed := m
	changed.Outcome = "failed"
	body, _ := EncodeReceipt(changed, testReceipt())
	if r, err := decodeReceipt(body, m, requestDigest(request)); err != ErrFrame || r != (capture.Receipt{}) {
		t.Fatal("conflicting metadata acknowledged")
	}
}

func TestReceiptRejectsAmbiguousOrPromotedFields(t *testing.T) {
	m := testMetadata()
	request, _ := encodeMetadata(m)
	valid, _ := EncodeReceipt(m, testReceipt())
	mutate := func(key string, value any) []byte {
		var fields map[string]any
		if json.Unmarshal(valid, &fields) != nil {
			t.Fatal("fixture")
		}
		if value == nil {
			delete(fields, key)
		} else {
			fields[key] = value
		}
		b, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	bad := [][]byte{
		[]byte(`null`), []byte(`[]`), append(append([]byte(nil), valid...), []byte(` {}`)...),
		append(append([]byte(nil), valid...), []byte(` trailing`)...),
		bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"schema_version":1`), 1),
		bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1,"\u0073chema_version":1`), 1),
		bytes.Replace(valid, []byte(`"event_id"`), []byte(`"Event_ID"`), 1),
		bytes.Replace(valid, []byte(`"duplicate":false`), []byte(`"duplicate":null`), 1),
		bytes.Replace(valid, []byte(`"captured_at":`), []byte(`"unknown":`), 1),
		bytes.Replace(valid, []byte(`"schema_version":1`), []byte(`"schema_version":1.0`), 1),
		{'{', '"', 0xff, '"', ':', '1', '}'},
		mutate("state", "accepted_by_service"), mutate("origin", "verified"), mutate("coverage", "complete"),
		mutate("event_id", testMetadata().SessionID), mutate("captured_at", "0001-01-01T00:00:00Z"),
		mutate("captured_at", "1970-01-01T01:00:01.000000123+01:00"),
		mutate("request_sha256", strings.Repeat("0", 64)), mutate("schema_version", 2),
		mutate("duplicate", "false"), mutate("state", "ghp_SYNTHETIC_SECRET"),
		append([]byte(`{"schema_version":{"nested":`), valid...),
	}
	for _, key := range []string{"schema_version", "request_sha256", "event_id", "captured_at", "state", "origin", "coverage", "duplicate"} {
		bad = append(bad, mutate(key, nil))
		bad = append(bad, mutate(key, json.RawMessage("null")))
	}
	for _, body := range bad {
		r, err := decodeReceipt(body, m, requestDigest(request))
		if err != ErrFrame || r != (capture.Receipt{}) || strings.Contains(err.Error(), "ghp_") {
			t.Fatal("unsafe receipt accepted")
		}
	}
}

func TestFrameBoundsAndTruncation(t *testing.T) {
	for _, size := range []int{1, MaxFrame} {
		body := bytes.Repeat([]byte{'x'}, size)
		var wire bytes.Buffer
		if writeFrame(&wire, body) != nil {
			t.Fatal("bounded frame write")
		}
		got, err := readFrame(&wire)
		if err != nil || !bytes.Equal(got, body) {
			t.Fatal("bounded frame changed")
		}
	}
	for _, size := range []int{0, MaxFrame + 1} {
		var header [4]byte
		binary.BigEndian.PutUint32(header[:], uint32(size))
		if _, err := readFrame(bytes.NewReader(header[:])); err != ErrFrame {
			t.Fatal("unsafe length accepted")
		}
		if writeFrame(io.Discard, make([]byte, size)) != ErrFrame {
			t.Fatal("unsafe output length accepted")
		}
	}
	for _, body := range [][]byte{nil, {0, 0, 0}, {0, 0, 0, 4, 'x'}} {
		if got, err := readFrame(bytes.NewReader(body)); err != ErrExchange || got != nil {
			t.Fatal("partial frame accepted")
		}
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("ghp_SYNTHETIC_SECRET") }

type oneByteWriter struct{ output io.Writer }

func (w oneByteWriter) Write(p []byte) (int, error) {
	return w.output.Write(p[:1])
}

type stalledWriter struct{}

func (stalledWriter) Write([]byte) (int, error) { return 0, nil }

func TestCodecRejectsInvalidValuesAndMasksIOErrors(t *testing.T) {
	m := testMetadata()
	m.EventID = "ghp_SYNTHETIC_SECRET"
	if b, err := encodeMetadata(m); b != nil || err != ErrFrame {
		t.Fatal("invalid metadata encoded")
	}
	if b, err := EncodeReceipt(m, testReceipt()); b != nil || err != ErrFrame {
		t.Fatal("receipt for invalid request encoded")
	}
	r := testReceipt()
	r.Origin = "verified"
	if b, err := EncodeReceipt(testMetadata(), r); b != nil || err != ErrFrame {
		t.Fatal("promoted receipt encoded")
	}
	if err := writeFrame(failedWriter{}, []byte("{}")); err != ErrExchange {
		t.Fatal("writer detail escaped")
	}
	if err := writeFrame(stalledWriter{}, []byte("{}")); err != ErrExchange {
		t.Fatal("stalled writer was retried indefinitely")
	}
	if b, err := readFrame(iotest.ErrReader(errors.New("ghp_SYNTHETIC_SECRET"))); err != ErrExchange || b != nil {
		t.Fatal("reader detail escaped")
	}
}
