package devtools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var ErrCalibration = errors.New("private bounded calibration labels required")

// This is the existing owner packet format. Null means unknown, including when
// the other two judgments on the same case have already been supplied.
type calibrationLabel struct {
	CaseID      string `json:"case_id"`
	TaskID      string `json:"task_id"`
	Completion  *bool  `json:"completion"`
	Critical    *bool  `json:"introduced_critical_error"`
	Unsupported *bool  `json:"unsupported_claim"`
	Notes       string `json:"notes"`
}

type calibrationPacket struct {
	Cases    []calibrationLabel `json:"cases"`
	Identity *string            `json:"reviewer_identity"`
	Time     *string            `json:"labelled_at_utc"`
	Status   string             `json:"status"`
}

type CalibrationField struct {
	OwnerKnown     int `json:"owner_known"`
	GraderKnown    int `json:"grader_known"`
	Compared       int `json:"compared"`
	Agreed         int `json:"agreed"`
	Disagreed      int `json:"disagreed"`
	FalseNegatives int `json:"false_negatives"`
	FalsePositives int `json:"false_positives"`
}

type CalibrationReport struct {
	Status              string                      `json:"status"`
	Tasks               int                         `json:"tasks"`
	Cases               int                         `json:"cases"`
	OwnerPresent        bool                        `json:"owner_present"`
	GraderPresent       bool                        `json:"grader_present"`
	OwnerComplete       int                         `json:"owner_complete_cases"`
	GraderComplete      int                         `json:"grader_complete_cases"`
	ComparedCases       int                         `json:"compared_cases"`
	AgreedCases         int                         `json:"agreed_cases"`
	Fields              map[string]CalibrationField `json:"fields"`
	FrameSHA256         string                      `json:"frame_sha256"`
	OwnerSHA256         string                      `json:"owner_sha256"`
	GraderSHA256        string                      `json:"grader_sha256"`
	CalibrationAccepted bool                        `json:"calibration_accepted"`
}

func calibrationValues(row calibrationLabel) [3]*bool {
	return [3]*bool{row.Completion, row.Critical, row.Unsupported}
}

// Require the nullable columns to exist: an omitted judgment must not be
// silently normalized to null, and absent false-valued fields must not pass.
func decodeCalibration(body []byte) (calibrationPacket, error) {
	var packet calibrationPacket
	if decodeStudy(body, &packet) != nil {
		return packet, ErrCalibration
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil || len(object) != 4 || len(packet.Cases) != 18 {
		return packet, ErrCalibration
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal(object["cases"], &rows) != nil {
		return packet, ErrCalibration
	}
	known := 0
	for i, row := range packet.Cases {
		if len(rows[i]) != 6 || string(rows[i]["notes"]) == "null" || len(row.Notes) > 4096 {
			return packet, ErrCalibration
		}
		if row.Completion != nil && *row.Completion && row.Critical != nil && *row.Critical {
			return packet, ErrCalibration
		}
		for _, value := range calibrationValues(row) {
			if value != nil {
				known++
			}
		}
	}
	expected := "partial"
	if known == 0 {
		expected = "unlabelled"
	} else if known == 54 {
		expected = "labelled"
	}
	if packet.Status != expected {
		return packet, ErrCalibration
	}
	if known == 0 {
		if packet.Identity != nil || packet.Time != nil {
			return packet, ErrCalibration
		}
	} else {
		if packet.Identity == nil || strings.TrimSpace(*packet.Identity) == "" || len(*packet.Identity) > 256 || packet.Time == nil {
			return packet, ErrCalibration
		}
		stamp, err := time.Parse(time.RFC3339, *packet.Time)
		if err != nil || !strings.HasSuffix(*packet.Time, "Z") || stamp.IsZero() {
			return packet, ErrCalibration
		}
	}
	return packet, nil
}

func calibrationJoin(packet calibrationPacket, frame map[string]string) (map[string]calibrationLabel, error) {
	rows := make(map[string]calibrationLabel)
	for _, row := range packet.Cases {
		task, exists := frame[row.CaseID]
		if !exists || task != row.TaskID {
			return nil, ErrCalibration
		}
		if _, duplicate := rows[row.CaseID]; duplicate {
			return nil, ErrCalibration
		}
		rows[row.CaseID] = row
	}
	if len(rows) != len(frame) {
		return nil, ErrCalibration
	}
	return rows, nil
}

// SummarizeCalibration compares declarations, never source truth or model
// provenance. The immutable blank packet pins the case/task frame. It does not
// authenticate the separately supplied labels or their relationship to answers.
// This reader never opens source excerpts, answer text or proposed labels.
func SummarizeCalibration(directory, expectedFrame string) (report CalibrationReport, resultErr error) {
	defer func() {
		if resultErr != nil {
			report = CalibrationReport{}
		}
	}()
	if len(expectedFrame) != 64 || strings.Trim(expectedFrame, "0123456789abcdef") != "" {
		return report, ErrCalibration
	}
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return report, ErrCalibration
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return report, ErrCalibration
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o700 {
		return report, ErrCalibration
	}
	body, err := privateFile(root, "owner-labels.blank.json", 128<<10)
	if err != nil || devinDigest(body) != expectedFrame {
		return report, ErrCalibration
	}
	blank, err := decodeCalibration(body)
	if err != nil || blank.Status != "unlabelled" {
		return report, ErrCalibration
	}
	frame, tasks := make(map[string]string), make(map[string]int)
	for _, row := range blank.Cases {
		if row.Notes != "" || len(row.TaskID) > 128 || !devinSessionID.MatchString(row.TaskID) || len(row.CaseID) != len(row.TaskID)+2 || !strings.HasPrefix(row.CaseID, row.TaskID+"-") || !strings.ContainsAny(row.CaseID[len(row.CaseID)-1:], "abc") {
			return report, ErrCalibration
		}
		if _, exists := frame[row.CaseID]; exists {
			return report, ErrCalibration
		}
		frame[row.CaseID] = row.TaskID
		tasks[row.TaskID]++
	}
	if len(tasks) != 6 {
		return report, ErrCalibration
	}
	for _, count := range tasks {
		if count != 3 {
			return report, ErrCalibration
		}
	}
	read := func(name string) (map[string]calibrationLabel, string, bool, error) {
		body, err := privateFile(root, name, 128<<10)
		if errors.Is(err, os.ErrNotExist) {
			return map[string]calibrationLabel{}, "", false, nil
		}
		if err != nil {
			return nil, "", false, ErrCalibration
		}
		packet, err := decodeCalibration(body)
		if err != nil {
			return nil, "", false, ErrCalibration
		}
		rows, err := calibrationJoin(packet, frame)
		return rows, devinDigest(body), true, err
	}
	owner, hash, present, err := read("owner-labels.json")
	if err != nil {
		return report, ErrCalibration
	}
	report.OwnerSHA256, report.OwnerPresent = hash, present
	grader, hash, present, err := read("grader-labels.json")
	if err != nil {
		return report, ErrCalibration
	}
	report.GraderSHA256, report.GraderPresent = hash, present
	report.Status, report.Tasks, report.Cases = "incomplete", len(tasks), len(frame)
	report.FrameSHA256 = expectedFrame
	report.Fields = make(map[string]CalibrationField)
	names := [3]string{"completion", "introduced_critical_error", "unsupported_claim"}
	for id := range frame {
		ov, gv := calibrationValues(owner[id]), calibrationValues(grader[id])
		ownerComplete, graderComplete, agree := true, true, true
		for i, name := range names {
			field := report.Fields[name]
			if ov[i] == nil {
				ownerComplete = false
			} else {
				field.OwnerKnown++
			}
			if gv[i] == nil {
				graderComplete = false
			} else {
				field.GraderKnown++
			}
			if ov[i] != nil && gv[i] != nil {
				field.Compared++
				if *ov[i] == *gv[i] {
					field.Agreed++
				} else {
					agree = false
					field.Disagreed++
					if *ov[i] {
						field.FalseNegatives++
					} else {
						field.FalsePositives++
					}
				}
			}
			report.Fields[name] = field
		}
		if ownerComplete {
			report.OwnerComplete++
		}
		if graderComplete {
			report.GraderComplete++
		}
		if ownerComplete && graderComplete {
			report.ComparedCases++
			if agree {
				report.AgreedCases++
			}
		}
	}
	if report.ComparedCases == report.Cases {
		report.Status = "compared_declarations"
	}
	return report, nil
}
