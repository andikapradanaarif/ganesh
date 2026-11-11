package devtools

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Build new synthetic schema-3 examples, never convert saved model output.
func observationFixture(t *testing.T) (evidenceFrame, evidenceObservationResponse) {
	t.Helper()
	f, old := evidenceFixture(t)
	r := evidenceObservationResponse{3, f.Task, f.Case, old.Frame, evidenceBool(false), evidenceBool(false), nil, "private-observation-note"}
	for _, c := range old.Criteria {
		r.Criteria = append(r.Criteria, evidenceObservation{c.ID, "covered", c.Spans, "", "absent", []evidenceSpan{}, "No defeat reported after reviewing the whole answer."})
	}
	return f, r
}

func checkObservation(t *testing.T, f evidenceFrame, r evidenceObservationResponse) (GraderEvidenceReport, error) {
	t.Helper()
	fb := evidenceJSON(t, f)
	r.Frame = devinDigest(fb)
	return checkGraderEvidence(fb, evidenceJSON(t, r))
}

func assertObservationRejected(t *testing.T, fb, rb []byte) {
	t.Helper()
	if report, err := checkGraderEvidence(fb, rb); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
		t.Fatal("invalid observation admitted or partial report leaked")
	}
}

func TestObservationFrozenCombinationTable(t *testing.T) {
	// Explicit expectations from the frozen #92 proposal, not the combiner.
	for _, row := range []struct{ coverage, defeat, expected string }{
		{"covered", "absent", "met"}, {"covered", "present", "unmet"}, {"covered", "unknown", "unresolved"},
		{"missing", "absent", "unmet"}, {"missing", "present", "unmet"}, {"missing", "unknown", "unmet"},
		{"unknown", "absent", "unresolved"}, {"unknown", "present", "unmet"}, {"unknown", "unknown", "unresolved"},
	} {
		t.Run(row.coverage+"/"+row.defeat, func(t *testing.T) {
			f, r := observationFixture(t)
			r.Criteria[0].Coverage, r.Criteria[0].Defeat = row.coverage, row.defeat
			r.Criteria[0].CoverageExplanation = "Synthetic coverage declaration."
			if row.defeat != "absent" {
				r.Criteria[0].ContrarySpans = r.Criteria[0].SupportingSpans
			}
			// Join IDs independently of the order of either array.
			f.Criteria[0], f.Criteria[3] = f.Criteria[3], f.Criteria[0]
			r.Criteria[0], r.Criteria[2] = r.Criteria[2], r.Criteria[0]
			report, err := checkObservation(t, f, r)
			if err != nil || report.ResponseSchemaVersion != 3 || report.Criteria != 4 || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
				t.Fatal("combination or authority boundary")
			}
			met, unmet, unresolved := 3, 0, 0
			var completion *bool
			switch row.expected {
			case "met":
				met++
				completion = evidenceBool(true)
			case "unmet":
				unmet++
				completion = evidenceBool(false)
			case "unresolved":
				unresolved++
			}
			spans := 4
			if row.defeat != "absent" {
				spans++
			}
			if report.Met != met || report.Unmet != unmet || report.Unresolved != unresolved || report.VerifiedSpans != spans || !reflect.DeepEqual(report.DerivedCompletion, completion) {
				t.Fatal("frozen expected disposition or completion changed")
			}
		})
	}
}

func TestObservationCompletionPrecedence(t *testing.T) {
	for _, row := range []struct {
		name                 string
		missing, unknown     bool
		critical, completion *bool
	}{
		{"met", false, false, evidenceBool(false), evidenceBool(true)},
		{"critical only", false, false, evidenceBool(true), evidenceBool(false)},
		{"unknown critical", false, false, nil, nil},
		{"unresolved", false, true, evidenceBool(false), nil},
		{"missing beats unknown critical", true, false, nil, evidenceBool(false)},
		{"critical beats unresolved", false, true, evidenceBool(true), evidenceBool(false)},
		{"missing beats unresolved", true, true, evidenceBool(false), evidenceBool(false)},
	} {
		t.Run(row.name, func(t *testing.T) {
			f, r := observationFixture(t)
			r.Critical, r.Unsupported = row.critical, nil
			if row.missing {
				r.Criteria[0].Coverage, r.Criteria[0].CoverageExplanation = "missing", "Required content is absent."
				r.Criteria[0].SupportingSpans = []evidenceSpan{}
			}
			if row.unknown {
				r.Criteria[1].Defeat = "unknown"
			}
			report, err := checkObservation(t, f, r)
			if err != nil || !reflect.DeepEqual(report.DerivedCompletion, row.completion) || report.UnsupportedClaim != nil || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
				t.Fatal("completion or unknown flags changed")
			}
			if row.name == "critical only" && report.Met != 4 {
				t.Fatal("critical flag reclassified criterion observations")
			}
		})
	}
}

func TestObservationMeaningAndPrivacyBoundary(t *testing.T) {
	f, r := observationFixture(t)
	bad := "Discard local work everywhere now."
	f.Answer += " " + bad
	start := strings.Index(f.Answer, bad)
	r.Criteria[1].Defeat, r.Criteria[1].DefeatExplanation = "present", "The final directive defeats the earlier preservation requirement."
	r.Criteria[1].ContrarySpans = []evidenceSpan{{start, start + len(bad), bad}}
	report, err := checkObservation(t, f, r)
	if err != nil || report.Unmet != 1 || report.Met != 3 || report.DerivedCompletion == nil || *report.DerivedCompletion {
		t.Fatal("declared contradiction did not defeat coverage")
	}
	// A missed contradiction is still mechanically admissible. Never infer a
	// correction from the answer or explanation, even if wording seems obvious.
	r.Criteria[1].Defeat, r.Criteria[1].ContrarySpans = "absent", []evidenceSpan{}
	report, err = checkObservation(t, f, r)
	if err != nil || report.Met != 4 || report.DerivedCompletion == nil || !*report.DerivedCompletion {
		t.Fatal("a hidden semantic oracle changed caller declarations")
	}
	// The same harmful sentence can instead be quoted and explicitly rejected.
	f.Answer = strings.ReplaceAll(f.Answer, bad, "Unsafe example to reject: «"+bad+"» I reject it.")
	r.Criteria[1].DefeatExplanation = "The example is quoted and rejected, not endorsed."
	report, err = checkObservation(t, f, r)
	if err != nil || report.Met != 4 {
		t.Fatal("quoted rejection control rejected")
	}
	// A wrong present judgment with a byte-exact quotation also passes. Quote
	// validity is not relevance or endorsement verification in either direction.
	start = strings.Index(f.Answer, bad)
	r.Criteria[1].Defeat = "present"
	r.Criteria[1].ContrarySpans = []evidenceSpan{{start, start + len(bad), bad}}
	report, err = checkObservation(t, f, r)
	if err != nil || report.Unmet != 1 || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("quote membership promoted to semantic assurance")
	}
	for _, secret := range []string{"ghp_", f.Task, f.Case, "private-observation", "Cover:", bad, "reject", "requirement"} {
		if strings.Contains(string(evidenceJSON(t, report)), secret) {
			t.Fatal("private observation content leaked")
		}
	}
}

func TestObservationRejectsMalformedDeclarations(t *testing.T) {
	for _, name := range []string{"missing critical", "missing unsupported", "completion", "status", "coverage missing field", "coverage null", "coverage invalid", "defeat missing", "defeat null", "defeat invalid", "support null", "contrary null", "coverage explanation missing", "coverage explanation null", "defeat explanation blank", "missing coverage explanation", "covered no support", "present no contrary", "absent with contrary", "missing criterion", "duplicate criterion", "unknown criterion", "long coverage explanation", "long defeat explanation", "long notes", "wrong task", "wrong case", "wrong frame", "version 1 mixed", "version 2 mixed", "version unknown", "float version"} {
		t.Run(name, func(t *testing.T) {
			f, r := observationFixture(t)
			var obj map[string]json.RawMessage
			if json.Unmarshal(evidenceJSON(t, r), &obj) != nil {
				t.Fatal("fixture")
			}
			var criteria []map[string]json.RawMessage
			if json.Unmarshal(obj["criteria"], &criteria) != nil {
				t.Fatal("fixture")
			}
			c := criteria[0]
			switch name {
			case "missing critical":
				delete(obj, "introduced_critical_error")
			case "missing unsupported":
				delete(obj, "unsupported_claim")
			case "completion":
				obj["completion"] = json.RawMessage(`null`)
			case "status":
				c["status"] = json.RawMessage(`"met"`)
			case "coverage missing field":
				delete(c, "coverage")
			case "coverage null":
				c["coverage"] = json.RawMessage(`null`)
			case "coverage invalid":
				c["coverage"] = json.RawMessage(`"met"`)
				c["defeat"] = json.RawMessage(`"present"`)
				c["contrary_spans"] = c["supporting_spans"]
			case "defeat missing":
				delete(c, "defeat")
			case "defeat null":
				c["defeat"] = json.RawMessage(`null`)
			case "defeat invalid":
				c["defeat"] = json.RawMessage(`"none"`)
				c["coverage"] = json.RawMessage(`"missing"`)
				c["coverage_explanation"] = json.RawMessage(`"Missing content."`)
			case "support null":
				c["supporting_spans"] = json.RawMessage(`null`)
			case "contrary null":
				c["contrary_spans"] = json.RawMessage(`null`)
			case "coverage explanation missing":
				delete(c, "coverage_explanation")
			case "coverage explanation null":
				c["coverage_explanation"] = json.RawMessage(`null`)
			case "defeat explanation blank":
				c["defeat_explanation"] = json.RawMessage(`" "`)
			case "missing coverage explanation":
				c["coverage"] = json.RawMessage(`"missing"`)
			case "covered no support":
				c["supporting_spans"] = json.RawMessage(`[]`)
			case "present no contrary":
				c["defeat"] = json.RawMessage(`"present"`)
			case "absent with contrary":
				c["contrary_spans"] = c["supporting_spans"]
			case "missing criterion":
				criteria = criteria[:3]
			case "duplicate criterion":
				criteria[1]["id"] = c["id"]
			case "unknown criterion":
				c["id"] = json.RawMessage(`"C5"`)
			case "long coverage explanation":
				c["coverage_explanation"] = evidenceJSON(t, strings.Repeat("x", 1025))
			case "long defeat explanation":
				c["defeat_explanation"] = evidenceJSON(t, strings.Repeat("x", 1025))
			case "long notes":
				obj["notes"] = evidenceJSON(t, strings.Repeat("x", 4097))
			case "wrong task":
				obj["task_id"] = json.RawMessage(`"other-task"`)
			case "wrong case":
				obj["case_id"] = json.RawMessage(`"other-case"`)
			case "wrong frame":
				obj["answer_frame_sha256"] = evidenceJSON(t, strings.Repeat("0", 64))
			case "version 1 mixed":
				obj["schema_version"] = json.RawMessage(`1`)
			case "version 2 mixed":
				obj["schema_version"] = json.RawMessage(`2`)
			case "version unknown":
				obj["schema_version"] = json.RawMessage(`4`)
			case "float version":
				obj["schema_version"] = json.RawMessage(`3.0`)
			}
			obj["criteria"] = evidenceJSON(t, criteria)
			assertObservationRejected(t, evidenceJSON(t, f), evidenceJSON(t, obj))
		})
	}
}

func TestObservationStrictDecoderAndLaneSpans(t *testing.T) {
	for _, lane := range []string{"supporting_spans", "contrary_spans"} {
		for _, name := range []string{"null element", "duplicate range", "too many", "negative", "oversize end", "quote mismatch", "missing zero", "null zero", "float start", "split utf8", "long quote", "span alias", "span unknown"} {
			t.Run(lane+"/"+name, func(t *testing.T) {
				f, r := observationFixture(t)
				f.Answer = "Aé🙂Z" + strings.Repeat("x", 2050)
				r.Frame = devinDigest(evidenceJSON(t, f))
				for i := range r.Criteria {
					r.Criteria[i].SupportingSpans = []evidenceSpan{{0, 1, "A"}}
					r.Criteria[i].ContrarySpans = []evidenceSpan{{0, 1, "A"}}
					r.Criteria[i].Defeat = "present"
				}
				var obj map[string]json.RawMessage
				json.Unmarshal(evidenceJSON(t, r), &obj)
				var criteria []map[string]json.RawMessage
				json.Unmarshal(obj["criteria"], &criteria)
				span := map[string]any{"start": 0, "end": 1, "quote": "A"}
				spans := []any{span}
				switch name {
				case "null element":
					spans = []any{span, nil}
				case "duplicate range":
					spans = []any{span, span}
				case "too many":
					spans = []any{
						map[string]any{"start": 0, "end": 1, "quote": "A"},
						map[string]any{"start": 1, "end": 3, "quote": "é"},
						map[string]any{"start": 3, "end": 7, "quote": "🙂"},
						map[string]any{"start": 7, "end": 8, "quote": "Z"},
						map[string]any{"start": 8, "end": 9, "quote": "x"},
					}
				case "negative":
					span["start"] = -1
				case "oversize end":
					span["end"] = len(f.Answer) + 1
				case "quote mismatch":
					span["quote"] = "wrong"
				case "missing zero":
					delete(span, "start")
				case "null zero":
					span["start"] = nil
				case "float start":
					span["start"] = json.RawMessage(`0.0`)
				case "split utf8":
					span["start"] = 2
					span["end"] = 3
					span["quote"] = "�"
				case "long quote":
					span["start"] = len(f.Answer) - 2049
					span["end"] = len(f.Answer)
					span["quote"] = strings.Repeat("x", 2049)
				case "span alias":
					span["Start"] = span["start"]
					delete(span, "start")
				case "span unknown":
					span["other"] = true
				}
				criteria[0][lane] = evidenceJSON(t, spans)
				obj["criteria"] = evidenceJSON(t, criteria)
				assertObservationRejected(t, evidenceJSON(t, f), evidenceJSON(t, obj))
			})
		}
	}
	f, r := observationFixture(t)
	good := evidenceJSON(t, r)
	for _, mutation := range []func([]byte) []byte{
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schema_version":3`), []byte(`"schema_version":3,"schema_versio\u006e":3`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schema_version":3`), []byte(`"Schema_Version":3`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"coverage":"covered"`), []byte(`"coverage":"covered","coverag\u0065":"covered"`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"defeat":"absent"`), []byte(`"Defeat":"absent"`), 1)
		},
		func(b []byte) []byte { return append(b, []byte(` {}`)...) },
		func(b []byte) []byte { return bytes.Replace(b, []byte("private-observation-note"), []byte{0xff}, 1) },
	} {
		assertObservationRejected(t, evidenceJSON(t, f), mutation(good))
	}
	// Identical ranges in separate lanes are permitted, not deduplicated away.
	r.Criteria[0].Defeat = "present"
	r.Criteria[0].ContrarySpans = r.Criteria[0].SupportingSpans
	report, err := checkObservation(t, f, r)
	if err != nil || report.VerifiedSpans != 5 || report.Unmet != 1 {
		t.Fatal("separate evidence lanes lost")
	}
}

func TestObservationPrivateReader(t *testing.T) {
	dir := t.TempDir()
	if os.Chmod(dir, 0700) != nil {
		t.Fatal("mode")
	}
	f, r := observationFixture(t)
	fb, rb := evidenceJSON(t, f), evidenceJSON(t, r)
	if os.WriteFile(filepath.Join(dir, "answer-frame.json"), fb, 0600) != nil || os.WriteFile(filepath.Join(dir, "grader-evidence.json"), rb, 0600) != nil {
		t.Fatal("write")
	}
	report, err := CheckGraderEvidence(dir, devinDigest(fb), devinDigest(rb))
	if err != nil || report.ResponseSchemaVersion != 3 {
		t.Fatal("private version 3 read")
	}
	if os.Chmod(filepath.Join(dir, "grader-evidence.json"), 0644) != nil {
		t.Fatal("mode")
	}
	report, err = CheckGraderEvidence(dir, devinDigest(fb), devinDigest(rb))
	if err != ErrGraderEvidence || report != (GraderEvidenceReport{}) || strings.Contains(err.Error(), dir) {
		t.Fatal("unsafe version 3 reader or private error")
	}
}

func TestObservationLaneAndEnvelopeBounds(t *testing.T) {
	f, r := observationFixture(t)
	f.Answer = "Aé🙂Z"
	spans := []evidenceSpan{{0, 1, "A"}, {1, 3, "é"}, {3, 7, "🙂"}, {7, 8, "Z"}}
	for i := range r.Criteria {
		r.Criteria[i].SupportingSpans, r.Criteria[i].ContrarySpans = spans, spans
		r.Criteria[i].Defeat = "present"
		r.Criteria[i].CoverageExplanation, r.Criteria[i].DefeatExplanation = strings.Repeat("x", 1024), strings.Repeat("x", 1024)
	}
	r.Notes = strings.Repeat("x", 4096)
	report, err := checkObservation(t, f, r)
	if err != nil || report.VerifiedSpans != 32 || report.Unmet != 4 {
		t.Fatal("valid per-lane counts, UTF-8 boundaries or explanation/notes caps rejected")
	}
	fb := evidenceJSON(t, f)
	r.Frame = devinDigest(fb)
	rb := evidenceJSON(t, r)
	tooLargeResponse := append(append([]byte{}, rb...), bytes.Repeat([]byte(" "), evidenceFileLimit+1-len(rb))...)
	assertObservationRejected(t, fb, tooLargeResponse)
	tooLargeFrame := append(append([]byte{}, fb...), bytes.Repeat([]byte(" "), evidenceFileLimit+1-len(fb))...)
	r.Frame = devinDigest(tooLargeFrame)
	assertObservationRejected(t, tooLargeFrame, evidenceJSON(t, r))
}
