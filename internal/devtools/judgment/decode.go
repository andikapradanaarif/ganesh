// Package judgment decodes offline predicate judgments without deciding their
// semantic correctness. It has no provider, credentials or transport dependency.
package judgment

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

type Decision string

const (
	Yes     Decision = "true"
	No      Decision = "false"
	Unknown Decision = "unknown"
)

type Verdict struct {
	Completion  Decision `json:"completion"`
	Critical    Decision `json:"critical"`
	Unsupported Decision `json:"unsupported"`
}

func valid(v Decision) bool { return v == Yes || v == No || v == Unknown }

// Every required/critical/unsupported predicate uses the SAME record shape.
// Unknown metadata is reported and never becomes evidence or changes a bit.
type DecodedRecord struct {
	Decision    Decision
	Reason      string
	AnswerQuote string
	SourceQuote string
}
type Decoded struct {
	Extractable    bool
	Wrapped        bool
	Records        map[string]DecodedRecord
	MetadataIssues []string
	Error          string
}

func uniqueObjectKeys(b []byte) bool {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var visit func(int) bool
	visit = func(depth int) bool {
		if depth > 32 {
			return false
		}
		t, e := d.Token()
		if e != nil {
			return false
		}
		delim, ok := t.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				t, e := d.Token()
				name, ok := t.(string)
				key := strings.ToLower(name)
				if e != nil || !ok || seen[key] {
					return false
				}
				seen[key] = true
				if !visit(depth + 1) {
					return false
				}
			}
			t, e = d.Token()
			return e == nil && t == json.Delim('}')
		case '[':
			for d.More() {
				if !visit(depth + 1) {
					return false
				}
			}
			t, e = d.Token()
			return e == nil && t == json.Delim(']')
		default:
			return false
		}
	}
	if !visit(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}

// Decode requires an unambiguous boolean/null decision for each supplied ID.
// Metadata diagnostics never repair or overwrite those decisions.
func Decode(b []byte, expectedIDs []string) Decoded {
	r := Decoded{Records: map[string]DecodedRecord{}, MetadataIssues: []string{}}
	fail := func(s string) Decoded { r.Records = map[string]DecodedRecord{}; r.Error = s; return r }
	if len(b) > 64<<10 || !utf8.Valid(b) {
		return fail("invalid_input")
	}
	expected := map[string]bool{}
	for _, id := range expectedIDs {
		if id == "" || expected[id] {
			return fail("invalid_expected_ids")
		}
		expected[id] = true
	}
	if len(expected) == 0 {
		return fail("invalid_expected_ids")
	}
	b = bytes.TrimSpace(b)
	for _, prefix := range []string{"```json\n", "```\n"} {
		if bytes.HasPrefix(b, []byte(prefix)) && bytes.HasSuffix(b, []byte("```")) {
			r.Wrapped = true
			b = bytes.TrimSpace(bytes.TrimSuffix(bytes.TrimPrefix(b, []byte(prefix)), []byte("```")))
			break
		}
	}
	if !uniqueObjectKeys(b) {
		return fail("invalid_or_ambiguous_json")
	}
	var root map[string]json.RawMessage
	if json.Unmarshal(b, &root) != nil || root == nil {
		return fail("object_required")
	}
	rows, ok := root["decisions"]
	if !ok || bytes.Equal(bytes.TrimSpace(rows), []byte("null")) {
		return fail("decisions_required")
	}
	for key := range root {
		if key != "decisions" {
			r.MetadataIssues = append(r.MetadataIssues, "extra_top_field:"+key)
		}
	}
	var raw []map[string]json.RawMessage
	if json.Unmarshal(rows, &raw) != nil || len(raw) != len(expected) {
		return fail("decision_count")
	}
	for _, o := range raw {
		var id string
		if json.Unmarshal(o["id"], &id) != nil || !expected[id] {
			return fail("unexpected_id")
		}
		if _, exists := r.Records[id]; exists {
			return fail("duplicate_id")
		}
		value, ok := o["decision"]
		if !ok {
			return fail("missing_decision")
		}
		v := Unknown
		switch string(bytes.TrimSpace(value)) {
		case "true":
			v = Yes
		case "false":
			v = No
		case "null":
		default:
			return fail("wrong_decision_type")
		}
		row := DecodedRecord{Decision: v}
		for key, body := range o {
			switch key {
			case "id", "decision":
			case "reason", "answer_quote", "source_quote":
				var s string
				if bytes.Equal(bytes.TrimSpace(body), []byte("null")) || json.Unmarshal(body, &s) != nil || len(s) > 2048 {
					r.MetadataIssues = append(r.MetadataIssues, id+":"+key+":invalid")
					continue
				}
				switch key {
				case "reason":
					row.Reason = s
				case "answer_quote":
					row.AnswerQuote = s
				case "source_quote":
					row.SourceQuote = s
				}
			default:
				r.MetadataIssues = append(r.MetadataIssues, id+":extra_field:"+key)
			}
		}
		r.Records[id] = row
	}
	sort.Strings(r.MetadataIssues)
	r.Extractable = true
	return r
}

// Evidence diagnostics are separate from declared decisions. Exact presence
// still does not establish relevance, truth, or reason/decision coherence.
func EvidenceIssues(r Decoded, answer string, sources []string) []string {
	issues := []string{}
	if !r.Extractable {
		return []string{"unextractable"}
	}
	for id, row := range r.Records {
		if row.AnswerQuote != "" && !strings.Contains(answer, row.AnswerQuote) {
			issues = append(issues, id+":inexact_answer_quote")
		}
		if row.SourceQuote != "" {
			found := false
			for _, s := range sources {
				found = found || strings.Contains(s, row.SourceQuote)
			}
			if !found {
				issues = append(issues, id+":inexact_source_quote")
			}
		}
	}
	sort.Strings(issues)
	return issues
}

// Derive is pure ternary logic; the model need not emit redundant global bits.
func Derive(r Decoded, required, critical []string, unsupported string) (Verdict, error) {
	if !r.Extractable || len(required) == 0 || len(critical) == 0 {
		return Verdict{}, fmt.Errorf("unextractable or missing predicate set")
	}
	known := map[string]bool{}
	hasUnmet, requiredUnknown, hasCritical, criticalUnknown := false, false, false, false
	for _, group := range []struct {
		ids  []string
		kind string
	}{{required, "required"}, {critical, "critical"}, {[]string{unsupported}, "unsupported"}} {
		for _, id := range group.ids {
			row, ok := r.Records[id]
			if !ok || known[id] || !valid(row.Decision) {
				return Verdict{}, fmt.Errorf("missing or overlapping predicates")
			}
			known[id] = true
			if group.kind == "required" {
				hasUnmet = hasUnmet || row.Decision == No
				requiredUnknown = requiredUnknown || row.Decision == Unknown
			}
			if group.kind == "critical" {
				hasCritical = hasCritical || row.Decision == Yes
				criticalUnknown = criticalUnknown || row.Decision == Unknown
			}
		}
	}
	if len(known) != len(r.Records) {
		return Verdict{}, fmt.Errorf("unassigned predicate")
	}
	c := Unknown
	if hasCritical {
		c = Yes
	} else if !criticalUnknown {
		c = No
	}
	completion := Unknown
	if hasUnmet || c == Yes {
		completion = No
	} else if !requiredUnknown && c == No {
		completion = Yes
	}
	return Verdict{completion, c, r.Records[unsupported].Decision}, nil
}
