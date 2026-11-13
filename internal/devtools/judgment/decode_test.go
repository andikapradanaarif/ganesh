package judgment

import (
	"reflect"
	"strings"
	"testing"
)

var predicateIDs = []string{"C1", "K1", "U1"}

const plain = `{"decisions":[{"id":"C1","decision":true,"reason":"reported","answer_quote":"Tests passed","source_quote":"exit=0"},{"id":"K1","decision":false,"reason":"no violation","answer_quote":"","source_quote":""},{"id":"U1","decision":false,"reason":"supported","answer_quote":"","source_quote":""}]}`

func TestDecoderHandwrittenGolden(t *testing.T) {
	r := Decode([]byte(plain), predicateIDs)
	want := map[string]DecodedRecord{"C1": {Yes, "reported", "Tests passed", "exit=0"}, "K1": {No, "no violation", "", ""}, "U1": {No, "supported", "", ""}}
	if !r.Extractable || !reflect.DeepEqual(r.Records, want) || len(r.MetadataIssues) != 0 || len(EvidenceIssues(r, "Tests passed", []string{"exit=0"})) != 0 {
		t.Fatalf("golden: %+v", r)
	}
	v, e := Derive(r, []string{"C1"}, []string{"K1"}, "U1")
	if e != nil || v != (Verdict{Yes, No, No}) {
		t.Fatal("handwritten aggregation golden")
	}
}

func TestDeclaredDecisionIsNotRepairedFromReason(t *testing.T) {
	r := Decode([]byte(`{"decisions":[{"id":"C1","decision":false},{"id":"K1","decision":true,"reason":"The named error does not occur."},{"id":"K2","decision":false,"source_quote":"literal source"},{"id":"U1","decision":true}]}`), []string{"C1", "K1", "K2", "U1"})
	if !r.Extractable || r.Records["K1"].Decision != Yes || r.Records["U1"].Decision != Yes {
		t.Fatal("reason overrode declared bit, or overlapping K/U was disallowed")
	}
}

func TestInputAndIdentifierBounds(t *testing.T) {
	for _, ids := range [][]string{nil, {""}, {"C1", "C1"}} {
		if r := Decode([]byte(plain), ids); r.Extractable || r.Error != "invalid_expected_ids" {
			t.Fatal("invalid expected predicate IDs")
		}
	}
	for _, b := range [][]byte{[]byte(strings.Repeat(" ", (64<<10)+1)), {0xff}} {
		if r := Decode(b, predicateIDs); r.Extractable || r.Error != "invalid_input" {
			t.Fatal("invalid input size or UTF-8")
		}
	}
}
func TestDecoderHarmlessSerializationAndUniformEvidence(t *testing.T) {
	forms := []string{
		plain,
		"```json\n" + plain + "\n```",
		` { "decisions": [ {"decision":false,"id":"U1"}, {"source_quote":"","decision":false,"id":"K1"}, {"id":"C1","decision":true} ] } `,
		`{"diagnostic":"supplementary metadata","decisions":[{"id":"C1","decision":true,"source_quote":"exit=0"},{"id":"K1","decision":false,"source_quote":"exit=0","note":"extra information"},{"id":"U1","decision":false,"source_quote":"exit=0"}]}`,
	}
	for _, s := range forms {
		r := Decode([]byte(s), predicateIDs)
		v, e := Derive(r, []string{"C1"}, []string{"K1"}, "U1")
		if !r.Extractable || e != nil || v != (Verdict{Yes, No, No}) {
			t.Fatal("formatting, item order or supplementary fields changed decision")
		}
	}
}
func TestDecoderDoesNotCoerceAmbiguity(t *testing.T) {
	cases := []string{
		`{"decisions":[{"id":"C1","decision":true,"decision":false},{"id":"K1","decision":false},{"id":"U1","decision":false}]}`,
		`{"decisions":[{"id":"C1","decision":true,"Decision":false},{"id":"K1","decision":false},{"id":"U1","decision":false}]}`,
		`{"decisions":[{"id":"C1","decision":true},{"id":"C1","decision":false},{"id":"U1","decision":false}]}`,
		`{"decisions":[{"id":"C1","decision":"true"},{"id":"K1","decision":false},{"id":"U1","decision":false}]}`,
		`{"decisions":[{"id":"C1"},{"id":"K1","decision":false},{"id":"U1","decision":false}]}`,
		`{"decisions":[{"id":"C1","decision":true},{"id":"K1","decision":false}]}`,
		`{"decisions":[{"id":"C1","decision":true},{"id":"K1","decision":false},{"id":"other","decision":false}]}`,
		`{"decisions":null}`,
		plain + ` extra`, plain + plain,
	}
	for _, s := range cases {
		r := Decode([]byte(s), predicateIDs)
		if r.Extractable || len(r.Records) != 0 || r.Error == "" {
			t.Fatal("ambiguous or incomplete mapping admitted")
		}
	}
}
func TestDecoderDecisionAndEvidenceStaySeparate(t *testing.T) {
	s := `{"decisions":[{"id":"C1","decision":true,"source_quote":"exit ... zero"},{"id":"K1","decision":false,"answer_quote":null},{"id":"U1","decision":null}]}`
	r := Decode([]byte(s), predicateIDs)
	if !r.Extractable || len(r.MetadataIssues) != 1 || len(EvidenceIssues(r, "Tests passed", []string{"exit=0"})) != 1 {
		t.Fatal("evidence diagnostics lost or conflated with extractability")
	}
	v, e := Derive(r, []string{"C1"}, []string{"K1"}, "U1")
	if e != nil || v != (Verdict{Yes, No, Unknown}) {
		t.Fatal("bad quotation changed declared decision or null was coerced")
	}
}
func TestDeriveHandwrittenTernaryCases(t *testing.T) {
	for _, tc := range []struct {
		required, critical, unsupported Decision
		want                            Verdict
	}{
		{Yes, No, No, Verdict{Yes, No, No}}, {Yes, Yes, No, Verdict{No, Yes, No}}, {No, No, No, Verdict{No, No, No}}, {Unknown, No, No, Verdict{Unknown, No, No}}, {Yes, Unknown, No, Verdict{Unknown, Unknown, No}}, {No, Unknown, No, Verdict{No, Unknown, No}}, {Unknown, Yes, Unknown, Verdict{No, Yes, Unknown}},
	} {
		r := Decoded{Extractable: true, Records: map[string]DecodedRecord{"C1": {Decision: tc.required}, "K1": {Decision: tc.critical}, "U1": {Decision: tc.unsupported}}}
		v, e := Derive(r, []string{"C1"}, []string{"K1"}, "U1")
		if e != nil || v != tc.want {
			t.Fatalf("ternary %+v: got=%+v err=%v", tc, v, e)
		}
	}
	r := Decoded{Extractable: true, Records: map[string]DecodedRecord{"C1": {Decision: Yes}, "K1": {Decision: Unknown}, "K2": {Decision: Yes}, "U1": {Decision: No}}}
	v, e := Derive(r, []string{"C1"}, []string{"K1", "K2"}, "U1")
	if e != nil || v != (Verdict{No, Yes, No}) {
		t.Fatal("unknown branch hid a known violation")
	}
	if _, e = Derive(r, []string{"C1"}, []string{"K1"}, "U1"); e == nil {
		t.Fatal("unassigned OR branch silently dropped")
	}
}
