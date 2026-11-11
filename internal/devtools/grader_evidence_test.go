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

func evidenceBool(v bool) *bool { return &v }

func evidenceJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func evidenceFixture(t *testing.T) (evidenceFrame, evidenceResponse) {
	t.Helper()
	parts := []string{"The report is historical.", "Preserve local work.", "Check current state.", "Capture and origin remain unknown."}
	frame := evidenceFrame{Version: 1, Task: "private-synthetic-task", Case: "private-synthetic-case", Answer: strings.Join(parts, " ") + " café🙂 ghp_SYNTHETIC_CANARY"}
	response := evidenceResponse{Version: 1, Task: frame.Task, Case: frame.Case, Completion: evidenceBool(true), Critical: evidenceBool(false), Unsupported: evidenceBool(false), Notes: "private-note-canary"}
	for i, part := range parts {
		id := "C" + string(rune('1'+i))
		frame.Criteria = append(frame.Criteria, evidenceRequirement{id, "Cover: " + part})
		start := strings.Index(frame.Answer, part)
		response.Criteria = append(response.Criteria, evidenceCriterion{id, "met", []evidenceSpan{{start, start + len(part), part}}, ""})
	}
	response.Frame = devinDigest(evidenceJSON(t, frame))
	return frame, response
}

// Construct a future synthetic response, not an importer for old model output.
func evidenceV2JSON(t *testing.T, response evidenceResponse) []byte {
	t.Helper()
	response.Version = 2
	var object map[string]json.RawMessage
	if json.Unmarshal(evidenceJSON(t, response), &object) != nil {
		t.Fatal("synthetic response")
	}
	delete(object, "completion")
	return evidenceJSON(t, object)
}

func TestGraderEvidencePrivacyAndMeaningBoundary(t *testing.T) {
	frame, response := evidenceFixture(t)
	// A real quote can still fail the requirement it purports to cover. This
	// deliberately bad semantic judgment must never acquire verified relevance.
	response.Criteria[3].Spans = response.Criteria[2].Spans
	frameBody, responseBody := evidenceJSON(t, frame), evidenceJSON(t, response)
	report, err := checkGraderEvidence(frameBody, responseBody)
	if err != nil || report.Criteria != 4 || report.Met != 4 || report.VerifiedSpans != 4 || report.DerivedCompletion == nil || !*report.DerivedCompletion || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("quote membership was lost or promoted to meaning/permission")
	}
	encoded := string(evidenceJSON(t, report))
	for _, secret := range []string{"ghp_", frame.Task, frame.Case, "private-note-canary", "The report", "Cover:"} {
		if strings.Contains(encoded, secret) {
			t.Fatal("private text leaked")
		}
	}
	if report.FrameSHA256 != devinDigest(frameBody) || report.ResponseSHA256 != devinDigest(responseBody) {
		t.Fatal("digest binding")
	}
	v2, err := checkGraderEvidence(frameBody, evidenceV2JSON(t, response))
	if err != nil || v2.ResponseSchemaVersion != 2 || v2.Met != 4 || v2.DerivedCompletion == nil || !*v2.DerivedCompletion || v2.SemanticRelevanceVerified || v2.CalibrationAccepted || v2.ModelDispatchAuthorized {
		t.Fatal("version 2 promoted real but irrelevant quotations to assurance")
	}
	// The observed over-credit failure becomes structurally inconsistent once
	// the missing coverage is actually declared unmet.
	response.Criteria[3].Status, response.Criteria[3].Explanation = "unmet", "Required distinction absent."
	if report, err := checkGraderEvidence(frameBody, evidenceJSON(t, response)); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
		t.Fatal("incomplete answer admitted as complete")
	}
	response.Completion = evidenceBool(false)
	if report, err := checkGraderEvidence(frameBody, evidenceJSON(t, response)); err != nil || report.Unmet != 1 || report.DerivedCompletion == nil || *report.DerivedCompletion {
		t.Fatal("honest incomplete declaration rejected")
	}
}

func TestGraderEvidenceCompletionAndOrder(t *testing.T) {
	for _, test := range []struct {
		name                 string
		unmet, unresolved    bool
		critical, completion *bool
	}{
		{"all met", false, false, evidenceBool(false), evidenceBool(true)},
		{"unmet", true, false, evidenceBool(false), evidenceBool(false)},
		{"critical", false, false, evidenceBool(true), evidenceBool(false)},
		{"unresolved", false, true, evidenceBool(false), nil},
		{"unknown critical", false, false, nil, nil},
		{"unmet beats unknown critical", true, false, nil, evidenceBool(false)},
		{"critical beats unresolved", false, true, evidenceBool(true), evidenceBool(false)},
		{"unmet beats unresolved", true, true, evidenceBool(false), evidenceBool(false)},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame, response := evidenceFixture(t)
			response.Critical, response.Completion, response.Unsupported = test.critical, test.completion, nil
			if test.unmet {
				response.Criteria[0].Status = "unmet"
				response.Criteria[0].Spans = []evidenceSpan{}
				response.Criteria[0].Explanation = "Missing required detail."
			}
			if test.unresolved {
				response.Criteria[1].Status = "unresolved"
				response.Criteria[1].Explanation = "Meaning remains ambiguous."
			}
			// IDs, not position, join both arrays.
			frame.Criteria[0], frame.Criteria[3] = frame.Criteria[3], frame.Criteria[0]
			response.Criteria[0], response.Criteria[2] = response.Criteria[2], response.Criteria[0]
			fb := evidenceJSON(t, frame)
			response.Frame = devinDigest(fb)
			report, err := checkGraderEvidence(fb, evidenceJSON(t, response))
			if err != nil || report.ResponseSchemaVersion != 1 || !reflect.DeepEqual(report.DerivedCompletion, test.completion) || report.UnsupportedClaim != nil || report.SemanticRelevanceVerified {
				t.Fatal("tri-state derivation or criterion ID join")
			}
			report, err = checkGraderEvidence(fb, evidenceV2JSON(t, response))
			if err != nil || report.ResponseSchemaVersion != 2 || !reflect.DeepEqual(report.DerivedCompletion, test.completion) || report.UnsupportedClaim != nil || report.SemanticRelevanceVerified || report.CalibrationAccepted || report.ModelDispatchAuthorized {
				t.Fatal("version 2 changed tri-state derivation, ID join or authority")
			}
			for _, wrong := range []*bool{nil, evidenceBool(false), evidenceBool(true)} {
				if reflect.DeepEqual(wrong, test.completion) {
					continue
				}
				response.Completion = wrong
				if report, err := checkGraderEvidence(fb, evidenceJSON(t, response)); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
					t.Fatal("inconsistent completion accepted")
				}
			}
		})
	}
}

func TestGraderEvidenceRejectsMalformedStructure(t *testing.T) {
	mutations := map[string]func([]byte) []byte{
		"missing nullable": func(b []byte) []byte { return bytes.Replace(b, []byte(`"completion":true,`), nil, 1) },
		"null notes": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"notes":"private-note-canary"`), []byte(`"notes":null`), 1)
		},
		"unknown": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"completion":true`), []byte(`"completion":true,"extra":0`), 1)
		},
		"alias": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"completion":true`), []byte(`"Completion":true`), 1)
		},
		"duplicate escaped": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"completion":true`), []byte(`"completion":true,"completio\u006e":false`), 1)
		},
		"nested alias": func(b []byte) []byte { return bytes.Replace(b, []byte(`"status":"met"`), []byte(`"Status":"met"`), 1) },
		"nested unknown": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"status":"met"`), []byte(`"status":"met","extra":0`), 1)
		},
		"missing status": func(b []byte) []byte { return bytes.Replace(b, []byte(`"status":"met",`), nil, 1) },
		"null spans": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"spans":[`), []byte(`"spans":null,"discard":[`), 1)
		},
		"missing zero offset": func(b []byte) []byte { return bytes.Replace(b, []byte(`"start":0,`), nil, 1) },
		"null offset":         func(b []byte) []byte { return bytes.Replace(b, []byte(`"start":0`), []byte(`"start":null`), 1) },
		"float offset":        func(b []byte) []byte { return bytes.Replace(b, []byte(`"start":0`), []byte(`"start":0.0`), 1) },
		"huge offset": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"start":0`), []byte(`"start":99999999999999999999999999999999`), 1)
		},
		"string bool": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"completion":true`), []byte(`"completion":"true"`), 1)
		},
		"float version": func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schema_version":1`), []byte(`"schema_version":1.0`), 1)
		},
		"trailing":     func(b []byte) []byte { return append(b, []byte(` {}`)...) },
		"invalid utf8": func(b []byte) []byte { return bytes.Replace(b, []byte("private-note-canary"), []byte{0xff}, 1) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			frame, response := evidenceFixture(t)
			if report, err := checkGraderEvidence(evidenceJSON(t, frame), mutate(evidenceJSON(t, response))); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
				t.Fatal("malformed response or partial report admitted")
			}
		})
	}
	for _, name := range []string{"wrong version", "wrong case", "wrong task", "wrong frame", "missing criterion", "duplicate criterion", "unknown criterion", "missing met span", "duplicate span", "invented quote", "reversed span", "negative span", "oversize end", "blank unmet explanation", "unknown disposition", "too many spans", "long explanation", "long notes", "null spans", "missing frame criterion", "duplicate frame criterion", "blank requirement", "long requirement", "oversize answer", "oversize quote"} {
		t.Run(name, func(t *testing.T) {
			frame, response := evidenceFixture(t)
			switch name {
			case "wrong version":
				response.Version = 4
			case "wrong case":
				response.Case = "different-case"
			case "wrong task":
				response.Task = "different-task"
			case "missing criterion":
				response.Criteria = response.Criteria[:3]
			case "duplicate criterion":
				response.Criteria[1].ID = "C1"
			case "unknown criterion":
				response.Criteria[1].ID = "C5"
			case "missing met span":
				response.Criteria[1].Spans = []evidenceSpan{}
			case "duplicate span":
				response.Criteria[1].Spans = append(response.Criteria[1].Spans, response.Criteria[1].Spans[0])
			case "invented quote":
				response.Criteria[0].Spans[0].Quote = "Source-only detail not present."
			case "reversed span":
				response.Criteria[0].Spans[0].Start = response.Criteria[0].Spans[0].End
			case "negative span":
				response.Criteria[0].Spans[0].Start = -1
			case "oversize end":
				response.Criteria[0].Spans[0].End = int(^uint(0) >> 1)
			case "blank unmet explanation":
				response.Criteria[0].Status = "unmet"
				response.Completion = evidenceBool(false)
			case "unknown disposition":
				response.Criteria[0].Status = "met then defeated"
			case "too many spans":
				for range 4 {
					response.Criteria[0].Spans = append(response.Criteria[0].Spans, response.Criteria[0].Spans[0])
				}
			case "long explanation":
				response.Criteria[0].Explanation = strings.Repeat("x", 1025)
			case "long notes":
				response.Notes = strings.Repeat("x", 4097)
			case "null spans":
				response.Criteria[0].Spans = nil
			case "missing frame criterion":
				frame.Criteria = frame.Criteria[:3]
			case "duplicate frame criterion":
				frame.Criteria[1].ID = "C1"
			case "blank requirement":
				frame.Criteria[1].Requirement = " "
			case "long requirement":
				frame.Criteria[1].Requirement = strings.Repeat("x", 1025)
			case "oversize answer":
				frame.Answer = strings.Repeat("x", (32<<10)+1)
			case "oversize quote":
				frame.Answer = strings.Repeat("x", 2049)
				for i := range response.Criteria {
					response.Criteria[i].Spans = []evidenceSpan{{0, 2049, frame.Answer}}
				}
			}
			fb := evidenceJSON(t, frame)
			response.Frame = devinDigest(fb)
			if name == "wrong frame" {
				response.Frame = strings.Repeat("0", 64)
			}
			if report, err := checkGraderEvidence(fb, evidenceJSON(t, response)); err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
				t.Fatal("invalid contract accepted or partial report exposed")
			}
		})
	}
}

func TestGraderEvidenceUTF8ByteSpans(t *testing.T) {
	frame, response := evidenceFixture(t)
	frame.Answer = "Aé🙂Z"
	response.Frame = devinDigest(evidenceJSON(t, frame))
	for i := range response.Criteria {
		response.Criteria[i].Spans = []evidenceSpan{{1, 3, "é"}}
	}
	if _, err := checkGraderEvidence(evidenceJSON(t, frame), evidenceJSON(t, response)); err != nil {
		t.Fatal("valid UTF-8 byte span rejected")
	}
	for _, span := range []evidenceSpan{{2, 3, "�"}, {3, 6, "🙂"}, {4, 7, "🙂"}} {
		response.Criteria[0].Spans = []evidenceSpan{span}
		if _, err := checkGraderEvidence(evidenceJSON(t, frame), evidenceJSON(t, response)); err != ErrGraderEvidence {
			t.Fatal("split Unicode code point accepted")
		}
	}
}

func TestGraderEvidenceRejectsNullSpanElement(t *testing.T) {
	for _, name := range []string{"only null", "null after valid span"} {
		t.Run(name, func(t *testing.T) {
			frame, response := evidenceFixture(t)
			var object map[string]json.RawMessage
			if json.Unmarshal(evidenceJSON(t, response), &object) != nil {
				t.Fatal("response fixture")
			}
			var criteria []map[string]json.RawMessage
			if json.Unmarshal(object["criteria"], &criteria) != nil {
				t.Fatal("criterion fixture")
			}
			spans := []any{nil}
			if name == "null after valid span" {
				spans = []any{response.Criteria[0].Spans[0], nil}
			}
			criteria[0]["spans"] = evidenceJSON(t, spans)
			object["criteria"] = evidenceJSON(t, criteria)
			report, err := checkGraderEvidence(evidenceJSON(t, frame), evidenceJSON(t, object))
			if err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) {
				t.Fatal("null span admitted or earlier verified-span count exposed")
			}
		})
	}
}

func TestGraderEvidencePrivateReader(t *testing.T) {
	for _, name := range []string{"valid", "frame pin", "response pin", "missing frame", "missing response", "frame mode", "response mode", "dir mode", "dir symlink", "frame symlink", "response symlink", "frame too large", "response too large", "nonregular", "invalid digest"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if os.Chmod(dir, 0o700) != nil {
				t.Fatal("directory mode")
			}
			frame, response := evidenceFixture(t)
			fb, rb := evidenceJSON(t, frame), evidenceJSON(t, response)
			fp, rp := filepath.Join(dir, "answer-frame.json"), filepath.Join(dir, "grader-evidence.json")
			if os.WriteFile(fp, fb, 0o600) != nil || os.WriteFile(rp, rb, 0o600) != nil {
				t.Fatal("fixture")
			}
			framePin, responsePin := devinDigest(fb), devinDigest(rb)
			switch name {
			case "frame pin":
				framePin = strings.Repeat("0", 64)
			case "response pin":
				responsePin = strings.Repeat("0", 64)
			case "missing frame":
				if os.Remove(fp) != nil {
					t.Fatal("remove")
				}
			case "missing response":
				if os.Remove(rp) != nil {
					t.Fatal("remove")
				}
			case "frame mode":
				if os.Chmod(fp, 0o644) != nil {
					t.Fatal("mode")
				}
			case "response mode":
				if os.Chmod(rp, 0o644) != nil {
					t.Fatal("mode")
				}
			case "dir mode":
				if os.Chmod(dir, 0o755) != nil {
					t.Fatal("mode")
				}
			case "dir symlink":
				link := dir + "-link"
				if os.Symlink(dir, link) != nil {
					t.Fatal("symlink")
				}
				t.Cleanup(func() { os.Remove(link) })
				dir = link
			case "frame symlink":
				if os.Rename(fp, fp+".saved") != nil || os.Symlink(fp+".saved", fp) != nil {
					t.Fatal("symlink")
				}
			case "response symlink":
				if os.Rename(rp, rp+".saved") != nil || os.Symlink(rp+".saved", rp) != nil {
					t.Fatal("symlink")
				}
			case "frame too large":
				b := bytes.Repeat([]byte("x"), evidenceFileLimit+1)
				if os.WriteFile(fp, b, 0o600) != nil {
					t.Fatal("write")
				}
				framePin = devinDigest(b)
			case "response too large":
				b := bytes.Repeat([]byte("x"), evidenceFileLimit+1)
				if os.WriteFile(rp, b, 0o600) != nil {
					t.Fatal("write")
				}
				responsePin = devinDigest(b)
			case "nonregular":
				if os.Remove(fp) != nil || os.Mkdir(fp, 0o700) != nil {
					t.Fatal("directory file")
				}
			case "invalid digest":
				framePin = strings.Repeat("G", 64)
			}
			report, err := CheckGraderEvidence(dir, framePin, responsePin)
			if name == "valid" {
				if err != nil || report.Met != 4 || report.SemanticRelevanceVerified {
					t.Fatal("private valid read")
				}
				return
			}
			if err != ErrGraderEvidence || !reflect.DeepEqual(report, GraderEvidenceReport{}) || strings.Contains(err.Error(), dir) {
				t.Fatal("unsafe input or private diagnostics")
			}
		})
	}
}
