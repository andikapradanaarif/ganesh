package devtools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func calibrationFixture(t *testing.T) (string, string, calibrationPacket) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	packet := calibrationPacket{Status: "unlabelled"}
	for task := range 6 {
		for variant := range 3 {
			id := fmt.Sprintf("synthetic-task-%d", task)
			packet.Cases = append(packet.Cases, calibrationLabel{TaskID: id, CaseID: fmt.Sprintf("%s-%c", id, 'a'+variant)})
		}
	}
	body := saveCalibration(t, dir, "owner-labels.blank.json", packet)
	return dir, devinDigest(body), packet
}

func saveCalibration(t *testing.T, dir, name string, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), body, 0o600); err != nil {
		t.Fatal(err)
	}
	return body
}

func calibrationBool(value bool) *bool { return &value }

func labelCalibration(packet *calibrationPacket) {
	identity, stamp := "synthetic reviewer", "2026-11-08T17:43:28Z"
	packet.Identity, packet.Time, packet.Status = &identity, &stamp, "labelled"
	for i := range packet.Cases {
		packet.Cases[i].Completion = calibrationBool(false)
		packet.Cases[i].Critical = calibrationBool(false)
		packet.Cases[i].Unsupported = calibrationBool(false)
		packet.Cases[i].Notes = "ghp_SYNTHETIC_CANARY never echo this note"
	}
}

func TestCalibrationComparison(t *testing.T) {
	dir, pin, packet := calibrationFixture(t)
	report, err := SummarizeCalibration(dir, pin)
	if err != nil || report.Status != "incomplete" || report.Cases != 18 || report.OwnerComplete != 0 || report.ComparedCases != 0 || report.OwnerPresent || report.CalibrationAccepted {
		t.Fatalf("unstarted packet: %+v, %v", report, err)
	}
	labelCalibration(&packet)
	packet.Cases[0].Critical = calibrationBool(true)
	packet.Cases[1].Unsupported = calibrationBool(true)
	packet.Cases[2].Completion = calibrationBool(true)
	saveCalibration(t, dir, "owner-labels.json", packet)
	packet.Cases[0].Critical = calibrationBool(false) // critical false negative
	packet.Cases[3].Critical = calibrationBool(true)  // critical false positive
	packet.Cases[1].Unsupported = calibrationBool(false)
	packet.Cases[2].Completion = calibrationBool(false)
	packet.Cases[4].Completion = calibrationBool(true)
	// Reverse order to exercise ID joins rather than positional comparison.
	for i, j := 0, len(packet.Cases)-1; i < j; i, j = i+1, j-1 {
		packet.Cases[i], packet.Cases[j] = packet.Cases[j], packet.Cases[i]
	}
	saveCalibration(t, dir, "grader-labels.json", packet)
	report, err = SummarizeCalibration(dir, pin)
	if err != nil || report.Status != "compared_declarations" || report.OwnerComplete != 18 || report.GraderComplete != 18 || report.ComparedCases != 18 || report.AgreedCases != 13 || report.CalibrationAccepted {
		t.Fatalf("comparison: %+v, %v", report, err)
	}
	want := CalibrationField{OwnerKnown: 18, GraderKnown: 18, Compared: 18, Agreed: 16, Disagreed: 2, FalseNegatives: 1, FalsePositives: 1}
	if report.Fields["introduced_critical_error"] != want || report.Fields["completion"] != want {
		t.Fatal("incorrect confusion counts")
	}
	want.Agreed, want.Disagreed, want.FalsePositives = 17, 1, 0
	if report.Fields["unsupported_claim"] != want {
		t.Fatal("incorrect unsupported counts")
	}
	body, _ := json.Marshal(report)
	for _, private := range []string{"ghp_", "synthetic-task", "synthetic reviewer", dir} {
		if strings.Contains(string(body), private) {
			t.Fatal("private content leaked")
		}
	}
	packet.Cases[0].Critical = nil
	packet.Status = "partial"
	saveCalibration(t, dir, "grader-labels.json", packet)
	report, err = SummarizeCalibration(dir, pin)
	if err != nil || report.Status != "incomplete" || report.ComparedCases != 17 || report.GraderComplete != 17 || report.Fields["introduced_critical_error"].Compared != 17 || report.Fields["completion"].Compared != 18 {
		t.Fatal("unknown field counted as false or as full comparison")
	}
}

func TestCalibrationBlankAndPerfectAgreement(t *testing.T) {
	dir, pin, packet := calibrationFixture(t)
	saveCalibration(t, dir, "owner-labels.json", packet)
	saveCalibration(t, dir, "grader-labels.json", packet)
	report, err := SummarizeCalibration(dir, pin)
	if err != nil || !report.OwnerPresent || !report.GraderPresent || report.Status != "incomplete" || report.ComparedCases != 0 {
		t.Fatal("blank copies became completed labels")
	}
	for _, field := range report.Fields {
		if field != (CalibrationField{}) {
			t.Fatal("null labels became negative judgments")
		}
	}
	labelCalibration(&packet)
	saveCalibration(t, dir, "owner-labels.json", packet)
	saveCalibration(t, dir, "grader-labels.json", packet)
	report, err = SummarizeCalibration(dir, pin)
	if err != nil || report.Status != "compared_declarations" || report.AgreedCases != 18 || report.CalibrationAccepted {
		t.Fatal("perfect declarations accepted calibration or lost a case")
	}
}

func TestCalibrationRejectsInvalidPackets(t *testing.T) {
	for _, name := range []string{"duplicate", "missing row", "extra row", "wrong task", "unknown case", "missing field", "null notes", "duplicate key", "aliased key", "unknown key", "trailing", "wrong type", "invalid status", "missing identity", "invalid time", "critical completion", "oversize", "permissions", "symlink", "frame hash", "frame duplicate", "frame labelled", "directory permissions", "directory symlink"} {
		t.Run(name, func(t *testing.T) {
			dir, pin, packet := calibrationFixture(t)
			labelCalibration(&packet)
			switch name {
			case "duplicate":
				packet.Cases[1] = packet.Cases[0]
			case "missing row":
				packet.Cases = packet.Cases[1:]
			case "extra row":
				packet.Cases = append(packet.Cases, packet.Cases[0])
			case "wrong task":
				packet.Cases[0].TaskID = "other-task"
			case "unknown case":
				packet.Cases[0].CaseID = "other-case"
			case "invalid status":
				packet.Status = "unlabelled"
			case "missing identity":
				packet.Identity = nil
			case "invalid time":
				stamp := "yesterday"
				packet.Time = &stamp
			case "critical completion":
				packet.Cases[0].Critical, packet.Cases[0].Completion = calibrationBool(true), calibrationBool(true)
			}
			body := saveCalibration(t, dir, "grader-labels.json", packet)
			path := filepath.Join(dir, "grader-labels.json")
			switch name {
			case "missing field":
				body = []byte(strings.Replace(string(body), `"completion":false,`, "", 1))
			case "null notes":
				body = []byte(strings.Replace(string(body), `"notes":"ghp_SYNTHETIC_CANARY never echo this note"`, `"notes":null`, 1))
			case "duplicate key":
				body = []byte(strings.Replace(string(body), `"completion":false`, `"completion":true,"completion":false`, 1))
			case "aliased key":
				body = []byte(strings.Replace(string(body), `"completion":false`, `"Completion":false`, 1))
			case "unknown key":
				body = []byte(strings.Replace(string(body), `"completion":false`, `"extra":false,"completion":false`, 1))
			case "wrong type":
				body = []byte(strings.Replace(string(body), `"completion":false`, `"completion":"false"`, 1))
			case "trailing":
				body = append(body, []byte(`{}`)...)
			case "oversize":
				body = make([]byte, (128<<10)+1)
			case "frame hash":
				pin = strings.Repeat("0", 64)
			case "frame duplicate", "frame labelled":
				var blank calibrationPacket
				original, err := os.ReadFile(filepath.Join(dir, "owner-labels.blank.json"))
				if err != nil || json.Unmarshal(original, &blank) != nil {
					t.Fatal("fixture")
				}
				if name == "frame duplicate" {
					blank.Cases[1] = blank.Cases[0]
				} else {
					labelCalibration(&blank)
				}
				pin = devinDigest(saveCalibration(t, dir, "owner-labels.blank.json", blank))
			}
			if err := os.WriteFile(path, body, 0o600); err != nil {
				t.Fatal(err)
			}
			if name == "permissions" {
				if err := os.Chmod(path, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if name == "symlink" {
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".saved", path); err != nil {
					t.Fatal(err)
				}
			}
			if name == "directory permissions" {
				if err := os.Chmod(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if name == "directory symlink" {
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				dir = link
			}
			report, err := SummarizeCalibration(dir, pin)
			if err != ErrCalibration || !reflect.DeepEqual(report, CalibrationReport{}) {
				t.Fatalf("invalid packet accepted or partial report returned: %v", err)
			}
		})
	}
}
