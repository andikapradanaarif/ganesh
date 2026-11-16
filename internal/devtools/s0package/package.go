// Package s0package prepares private S0 proposals and local metadata preflights.
// There is no model-dispatch operation in this package.
package s0package

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andikapradanaarif/ganesh/internal/devtools/requestguard"
)

const BoundSHA = "c22dbd8d04e5ac5ff8b60014b03d1f3cce9f5a6254dc3915dcf5587a53131d15"
const GuardReportSHA = "e225346696c4c1dd7a71bdeb7ab6f0ab33a53e75b4584e789e440b6155ea23d8"

var ErrInput = errors.New("S0 package input or local inspection failed; no model call made")

type object = map[string]any
type Pin struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
type FileSpec struct {
	Project   string   `json:"project"`
	Path      string   `json:"path"`
	GitBlobs  []string `json:"expected_git_blobs"`
	NativeSHA string   `json:"expected_native_sha256,omitempty"`
}
type InspectionPlan struct {
	Roots map[string]string `json:"roots"`
	Files []FileSpec        `json:"files"`
}
type Slot struct {
	ID              string `json:"id"`
	Phase           string `json:"phase"`
	Profile         string `json:"profile"`
	Task            string `json:"task_id"`
	Arm             string `json:"arm,omitempty"`
	Sequence        int    `json:"sequence"`
	InputFiles      []Pin  `json:"input_files,omitempty"`
	GradingProposal *Pin   `json:"grading_proposal,omitempty"`
}
type Budget struct {
	AnswerSessions         int `json:"maximum_answer_sessions"`
	GradingSessions        int `json:"maximum_grading_sessions"`
	CalibrationGenerations int `json:"proposed_fresh_calibration_answers"`
	CalibrationJudgments   int `json:"proposed_calibration_judgments"`
	Slots                  int `json:"maximum_process_slots"`
	ProviderRequests       int `json:"maximum_provider_requests"`
	OutputTokens           int `json:"maximum_output_tokens"`
	InputStop              int `json:"post_response_input_stop"`
	ActiveSeconds          int `json:"maximum_active_model_seconds"`
	PacingSeconds          int `json:"minimum_answer_start_spacing_seconds"`
	PacedSeconds           int `json:"nominal_paced_seconds_at_full_session_caps"`
	WallSeconds            int `json:"proposed_wall_time_after_approval_seconds"`
}

func ProposedBudget() Budget {
	return Budget{AnswerSessions: 132, GradingSessions: 132, CalibrationGenerations: 6, CalibrationJudgments: 6, Slots: 276,
		ProviderRequests: 132*6 + 132 + 6 + 6, OutputTokens: 132*8192 + 132*2048 + 6*4096 + 6*2048,
		InputStop: 2000000, ActiveSeconds: 132*(180+60) + 6*(180+60), PacingSeconds: 600,
		PacedSeconds: (132-1)*600 + 180 + 60 + (6-1)*600 + 180 + 60, WallSeconds: 48 * 60 * 60}
}
func ProposedProfiles() map[string]requestguard.Config {
	a := requestguard.Config{Endpoint: "https://opencode.ai/inference/go/openai/v1/responses", Model: "gpt-6-luna", MaxRequests: 6, MaxOutputPerRequest: 4096, MaxOutputTotal: 8192, StopAfterInputTokens: 2000000, Timeout: 180 * time.Second, MaxBodyBytes: 4 << 20}
	c := a
	c.MaxRequests = 1
	c.MaxOutputTotal = 4096
	g := requestguard.Config{Protocol: "chat-completions", Endpoint: "https://opencode.ai/inference/go/openai/v1/chat/completions", Model: "deepseek-v4.1-flash", MaxRequests: 1, MaxOutputPerRequest: 2048, MaxOutputTotal: 2048, StopAfterInputTokens: 2000000, Timeout: 60 * time.Second, MaxBodyBytes: 4 << 20}
	return map[string]requestguard.Config{"answer": a, "calibration-answer": c, "grade": g}
}

type Options struct{ Packet, Calibration, Roots, GuardReport, Output string }
type manifest struct {
	Status         string            `json:"status"`
	Issue          int               `json:"issue"`
	Dispatch       bool              `json:"model_dispatch_authorized"`
	PacketSHA      string            `json:"bound_packet_sha256"`
	CalibrationSHA string            `json:"reserved_calibration_manifest_sha256"`
	GuardSHA       string            `json:"guard_report_sha256"`
	Files          map[string]string `json:"files_sha256"`
}
type binding struct {
	Group    string `json:"group"`
	Sequence int    `json:"sequence"`
	Task     string `json:"task_id"`
	Arm      string `json:"arm"`
	Files    []Pin  `json:"allowed_answer_files"`
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func safePath(s string) bool {
	return s != "" && s != "." && !filepath.IsAbs(s) && filepath.Clean(s) == s && s != ".." && !strings.HasPrefix(s, "../") && !strings.Contains(s, "\\")
}
func read(root *os.Root, name string) ([]byte, error) {
	return readLimit(root, name, 4<<20)
}
func readLimit(root *os.Root, name string, limit int) ([]byte, error) {
	if !safePath(name) {
		return nil, ErrInput
	}
	for part := name; ; part = filepath.Dir(part) {
		info, err := root.Lstat(part)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, ErrInput
		}
		if filepath.Dir(part) == "." {
			break
		}
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, ErrInput
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, ErrInput
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(b) > limit {
		return nil, ErrInput
	}
	after, e := f.Stat()
	current, e2 := root.Lstat(name)
	if e != nil || e2 != nil || !os.SameFile(info, current) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return nil, ErrInput
	}
	return b, nil
}
func decode(root *os.Root, name string, v any) error {
	b, err := read(root, name)
	if err != nil {
		return err
	}
	if json.Unmarshal(b, v) != nil {
		return ErrInput
	}
	return nil
}
func exclusive(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrInput
	}
	n, err := f.Write(b)
	closeErr := f.Close()
	if err != nil || n != len(b) || closeErr != nil {
		return ErrInput
	}
	return nil
}
func jsonBytes(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), err
}
func writeJSON(path string, v any) error {
	b, err := jsonBytes(v)
	if err != nil {
		return ErrInput
	}
	return exclusive(path, b)
}

// Prepare reads only the already selected private artifacts. It never reads the
// real project roots, contacts a provider, creates a run ledger or adopts tasks.
func Prepare(o Options) (string, error) {
	packet, err := os.OpenRoot(o.Packet)
	if err != nil {
		return "", ErrInput
	}
	defer packet.Close()
	b, err := read(packet, "manifest.json")
	if err != nil || sum(b) != BoundSHA {
		return "", ErrInput
	}
	var source struct {
		Files map[string]string `json:"artifact_sha256"`
	}
	if json.Unmarshal(b, &source) != nil || len(source.Files) == 0 {
		return "", ErrInput
	}
	for name, hash := range source.Files {
		data, err := read(packet, name)
		if err != nil || sum(data) != hash {
			return "", ErrInput
		}
	}
	guardRoot, err := os.OpenRoot(filepath.Dir(o.GuardReport))
	if err != nil {
		return "", ErrInput
	}
	defer guardRoot.Close()
	guardBytes, err := read(guardRoot, filepath.Base(o.GuardReport))
	if err != nil || sum(guardBytes) != GuardReportSHA {
		return "", ErrInput
	}
	var guard struct {
		Passed bool   `json:"passed"`
		Calls  int    `json:"real_inference_requests"`
		Binary string `json:"binary_sha256"`
	}
	if json.Unmarshal(guardBytes, &guard) != nil || !guard.Passed || guard.Calls != 0 {
		return "", ErrInput
	}
	calRoot, err := os.OpenRoot(filepath.Dir(o.Calibration))
	if err != nil {
		return "", ErrInput
	}
	defer calRoot.Close()
	calBytes, err := read(calRoot, filepath.Base(o.Calibration))
	if err != nil {
		return "", ErrInput
	}
	var cal struct {
		Tasks []struct {
			ID        string            `json:"id"`
			Files     map[string]string `json:"files"`
			PromptSHA string            `json:"prompt_sha256"`
			RubricSHA string            `json:"rubric_sha256"`
			OracleSHA string            `json:"source_keyed_oracle_sha256"`
		} `json:"tasks"`
	}
	if json.Unmarshal(calBytes, &cal) != nil || len(cal.Tasks) != 6 {
		return "", ErrInput
	}
	rootMap, err := os.ReadFile(o.Roots)
	if err != nil || len(rootMap) > 65536 {
		return "", ErrInput
	}
	var allRoots map[string]string
	if json.Unmarshal(rootMap, &allRoots) != nil {
		return "", ErrInput
	}
	plan := InspectionPlan{Roots: map[string]string{}}
	for _, p := range []string{"kiosk", "valo-backend"} {
		if !filepath.IsAbs(allRoots[p]) {
			return "", ErrInput
		}
		plan.Roots[p] = allRoots[p]
	}
	var catalog struct {
		Entries map[string]struct {
			Project string `json:"project"`
			Path    string `json:"path"`
			Blob    string `json:"git_blob_id"`
		} `json:"catalog"`
	}
	if decode(packet, "source-index.json", &catalog) != nil {
		return "", ErrInput
	}
	selected := map[string]*FileSpec{}
	for _, e := range catalog.Entries {
		if _, ok := plan.Roots[e.Project]; !ok || e.Path == "" || e.Blob == "" {
			continue
		}
		if !safePath(e.Path) {
			return "", ErrInput
		}
		key := e.Project + ":" + e.Path
		f := selected[key]
		if f == nil {
			f = &FileSpec{Project: e.Project, Path: e.Path}
			selected[key] = f
		}
		found := false
		for _, h := range f.GitBlobs {
			found = found || h == e.Blob
		}
		if !found {
			f.GitBlobs = append(f.GitBlobs, e.Blob)
		}
	}
	var baseline struct {
		Native map[string][]struct {
			Path string `json:"path"`
			SHA  string `json:"snapshot_sha256"`
		} `json:"native_instruction_snapshots"`
	}
	if decode(packet, "baseline.json", &baseline) != nil {
		return "", ErrInput
	}
	for project, files := range baseline.Native {
		for _, file := range files {
			key := project + ":" + file.Path
			f := selected[key]
			if f == nil {
				f = &FileSpec{Project: project, Path: file.Path}
				selected[key] = f
			}
			f.NativeSHA = file.SHA
		}
	}
	for _, f := range selected {
		sort.Strings(f.GitBlobs)
		plan.Files = append(plan.Files, *f)
	}
	sort.Slice(plan.Files, func(i, j int) bool {
		return plan.Files[i].Project+plan.Files[i].Path < plan.Files[j].Project+plan.Files[j].Path
	})
	var answers struct {
		Bindings []binding `json:"bindings"`
	}
	var grading struct {
		Tasks []struct {
			Task string `json:"task_id"`
			File string `json:"file"`
			SHA  string `json:"sha256"`
		} `json:"tasks"`
	}
	if decode(packet, "answer-input-bindings.json", &answers) != nil || decode(packet, "grading/index.json", &grading) != nil || len(answers.Bindings) != 132 {
		return "", ErrInput
	}
	grades := map[string]Pin{}
	for _, g := range grading.Tasks {
		if source.Files[g.File] != g.SHA {
			return "", ErrInput
		}
		grades[g.Task] = Pin{g.File, g.SHA}
	}
	slots := []Slot{}
	seen := map[string]bool{}
	groups := map[string]int{}
	for _, a := range answers.Bindings {
		if a.Group != "base" && a.Group != "conditional" || a.Sequence <= 0 || grades[a.Task].File == "" {
			return "", ErrInput
		}
		id := a.Group + "." + fmtSequence(a.Sequence)
		if seen[id] {
			return "", ErrInput
		}
		seen[id] = true
		groups[a.Group]++
		for _, f := range a.Files {
			if source.Files[f.File] != f.SHA256 || strings.HasPrefix(f.File, "grading/") || strings.Contains(f.File, "oracle") || strings.Contains(f.File, "rubric") {
				return "", ErrInput
			}
		}
		slots = append(slots, Slot{ID: id + ".answer", Phase: a.Group, Profile: "answer", Task: a.Task, Arm: a.Arm, Sequence: a.Sequence, InputFiles: a.Files}, Slot{ID: id + ".grade", Phase: a.Group, Profile: "grade", Task: a.Task, Arm: a.Arm, Sequence: a.Sequence, GradingProposal: ptrPin(grades[a.Task])})
	}
	if groups["base"] != 108 || groups["conditional"] != 24 {
		return "", ErrInput
	}
	calPins := []Pin{{File: o.Calibration, SHA256: sum(calBytes)}}
	for i, t := range cal.Tasks {
		for kind, hash := range map[string]string{"prompt": t.PromptSHA, "rubric": t.RubricSHA, "oracle": t.OracleSHA} {
			data, err := read(calRoot, t.Files[kind])
			if err != nil || sum(data) != hash {
				return "", ErrInput
			}
			calPins = append(calPins, Pin{File: filepath.Join(filepath.Dir(o.Calibration), t.Files[kind]), SHA256: hash})
		}
		slots = append(slots, Slot{ID: "calibration." + fmtSequence(i+1) + ".answer", Phase: "calibration_proposal", Profile: "calibration-answer", Task: t.ID, Sequence: i + 1, InputFiles: []Pin{{File: filepath.Join(filepath.Dir(o.Calibration), t.Files["prompt"]), SHA256: t.PromptSHA}}}, Slot{ID: "calibration." + fmtSequence(i+1) + ".grade", Phase: "calibration_proposal", Profile: "grade", Task: t.ID, Sequence: i + 1})
	}
	if len(slots) != ProposedBudget().Slots {
		return "", ErrInput
	}
	sort.Slice(calPins, func(i, j int) bool { return calPins[i].File < calPins[j].File })
	rank := map[string]int{"calibration_proposal": 0, "base": 1, "conditional": 2}
	sort.SliceStable(slots, func(i, j int) bool {
		if rank[slots[i].Phase] != rank[slots[j].Phase] {
			return rank[slots[i].Phase] < rank[slots[j].Phase]
		}
		return slots[i].Sequence < slots[j].Sequence
	})
	if err = os.Mkdir(o.Output, 0700); err != nil {
		return "", ErrInput
	}
	files := map[string]any{"inspection-plan.json": plan, "process-slots.json": slots, "profiles-proposal.json": ProposedProfiles(), "budget-proposal.json": ProposedBudget(), "calibration-references.json": calPins,
		"readiness.json": object{"model_dispatch_authorized": false, "tasks_adopted": false, "grader_accepted": false, "source_processor_disclosure_approved": false, "account_quota_and_extra_usage_verified": false, "complete_request_fit_verified": false, "tool_inventory_and_permissions_complete": false, "owner_run_approved": false, "overall_focused_effort_known": false, "held_out_answers": 0},
		"authority.json": object{"grading_selection": "existing #83/#84 DeepSeek/OpenCode Go exception", "answer_model": "gpt-6-luna proposed, not owner-frozen", "source_of_project_roots": o.Roots, "bound_packet": o.Packet, "guard_report": o.GuardReport, "candidate_binary_sha256": guard.Binary, "new_calls_authorized": 0, "calibration_proposal": "six fresh model-written answers to the reserved source-keyed tasks plus six judgments; prior constructed answers and labels unchanged; acceptance is not automatic", "calibration_source_material_and_acceptance_unresolved": true, "arm_A_B_empty": true, "complete_wire_assembly_frozen": false}}
	m := manifest{Status: "OWNER_PREFLIGHT_PROPOSAL_HOLD_EXECUTION", Issue: 124, PacketSHA: BoundSHA, CalibrationSHA: sum(calBytes), GuardSHA: GuardReportSHA, Files: map[string]string{}}
	for name, value := range files {
		data, err := jsonBytes(value)
		if err != nil || exclusive(filepath.Join(o.Output, name), data) != nil {
			return "", ErrInput
		}
		m.Files[name] = sum(data)
	}
	data, err := jsonBytes(m)
	if err != nil || exclusive(filepath.Join(o.Output, "manifest.json"), data) != nil {
		return "", ErrInput
	}
	return sum(data), nil
}
func fmtSequence(i int) string {
	const digits = "0123456789"
	if i < 1 || i > 999 {
		return "invalid"
	}
	return string([]byte{digits[i/100], digits[i/10%10], digits[i%10]})
}
func ptrPin(p Pin) *Pin { return &p }
