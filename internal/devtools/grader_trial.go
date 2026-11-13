package devtools

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrGraderTrial = errors.New("grader trial stopped; inspect private state and report; do not retry")

const trialMaxInput = 12288
const trialMaxOutput = 2048

var trialModels = []string{"deepseek-v4.1-flash", "glm-5.3-flash", "claude-haiku-5-5"}

type trialLimits struct {
	Requests     int `json:"requests"`
	Input        int `json:"input_per_call"`
	Output       int `json:"combined_output_per_call"`
	InputTotal   int `json:"input_total"`
	OutputTotal  int `json:"combined_output_total"`
	Seconds      int `json:"seconds_per_call"`
	WholeSeconds int `json:"whole_seconds"`
	ExtraPaidUSD int `json:"additional_paid_usd"`
	Retries      int `json:"retries"`
}

var trialBudget = trialLimits{180, trialMaxInput, trialMaxOutput, 180 * trialMaxInput, 180 * trialMaxOutput, 120, 5400, 0, 0}

type trialJob struct {
	Case       string `json:"case_id"`
	Model      string `json:"model"`
	Repetition int    `json:"repetition"`
	Wire       string `json:"wire_file"`
}
type trialPlan struct {
	Issue          int               `json:"issue"`
	PacketManifest string            `json:"packet_manifest_sha256"`
	Binary         string            `json:"executable_sha256"`
	Seed           string            `json:"seed"`
	Limits         trialLimits       `json:"limits"`
	Files          map[string]string `json:"files_sha256"`
	Jobs           []trialJob        `json:"jobs"`
	Controls       []string          `json:"planted_critical_cases"`
	OwnerRequest   string            `json:"owner_request"`
	AccountBasis   string            `json:"extra_usage_state_basis"`
	Progression    string            `json:"progression_rule"`
}
type GraderTrialPreparation struct {
	Plan          string      `json:"plan_sha256"`
	Cases         int         `json:"development_cases"`
	Requests      int         `json:"requests"`
	MaxInputBytes int         `json:"largest_prompt_bytes_plus_512"`
	Limits        trialLimits `json:"limits"`
	RequestsSent  int         `json:"requests_sent"`
}
type trialState struct {
	Started  string `json:"started_utc"`
	Attempts int    `json:"attempts_reserved"`
	Input    int64  `json:"known_input_tokens"`
	Output   int64  `json:"known_combined_output_tokens"`
	Complete bool   `json:"complete"`
	Stopped  string `json:"stop_reason"`
}
type trialResult struct {
	BodySHA        string               `json:"native_response_sha256"`
	Status         int                  `json:"http_status"`
	ElapsedMS      int64                `json:"elapsed_ms"`
	ReturnedModel  string               `json:"returned_model"`
	Finish         string               `json:"finish"`
	UsageKnown     bool                 `json:"usage_known"`
	Usage          trialUsage           `json:"usage"`
	CompleteOutput bool                 `json:"complete_output"`
	Check          *GraderContractCheck `json:"check"`
	Stop           string               `json:"stop_reason"`
}
type trialProfile struct {
	Planned                int            `json:"planned"`
	Attempted              int            `json:"attempted"`
	Missing                int            `json:"missing_results"`
	KnownUsage             int            `json:"known_usage_records"`
	UnknownUsage           int            `json:"attempts_with_unknown_usage"`
	Allowance              string         `json:"known_allowance_fraction_sum"`
	LatencyP50             int64          `json:"all_recorded_attempts_latency_p50_ms"`
	LatencyP95             int64          `json:"all_recorded_attempts_latency_p95_ms"`
	Input                  int64          `json:"input_tokens"`
	Output                 int64          `json:"combined_output_tokens"`
	Reasoning              int64          `json:"reasoning_tokens_included_in_output"`
	CompleteOutput         int            `json:"completed_outputs"`
	Structure              int            `json:"structure_valid"`
	Evidence               int            `json:"evidence_valid"`
	KnownJudgments         int            `json:"three_global_judgments_known"`
	Wrapped                int            `json:"json_fences"`
	PlainUsable            int            `json:"plain_json_evidence_valid_completed"`
	DeclaredControlTrue    int            `json:"planted_critical_flags_true_before_evidence_check"`
	DeclaredControlFalse   int            `json:"planted_critical_flags_false_before_evidence_check"`
	DeclaredControlUnknown int            `json:"planted_critical_flags_unknown_or_missing_before_evidence_check"`
	Issues                 map[string]int `json:"overlapping_failure_counts"`
	Compared               int            `json:"reference_fields_compared"`
	Disagreements          int            `json:"reference_fields_disagreed"`
	ReferenceUnknown       int            `json:"reference_fields_unknown_or_disputed"`
	CriticalPlanned        int            `json:"planted_critical_trials_planned"`
	CriticalDetected       int            `json:"planted_critical_detected"`
	CriticalMissed         int            `json:"planted_critical_missed"`
	CriticalUnknown        int            `json:"planted_critical_unknown_or_missing"`
	FalseCritical          int            `json:"critical_false_positives_against_proposals"`
	RepeatPlanned          int            `json:"repeat_pairs_planned"`
	RepeatKnown            int            `json:"repeat_pairs_with_known_global_flags"`
	RepeatChanged          int            `json:"repeat_pairs_global_flags_changed"`
	OperationalPass        bool           `json:"development_operability_pass"`
}
type GraderTrialReport struct {
	Status           string                   `json:"status"`
	State            trialState               `json:"state"`
	Profiles         map[string]*trialProfile `json:"profiles"`
	ReferenceBasis   string                   `json:"reference_basis"`
	Limits           trialLimits              `json:"limits"`
	GraderAccepted   bool                     `json:"grader_accepted"`
	SemanticVerified bool                     `json:"semantic_accuracy_verified"`
	S0Answers        int                      `json:"s0_held_out_answers"`
}

func trialWire(request contractRequest, model string) ([]byte, error) {
	// A conservative byte proxy is deliberately separate from actual provider
	// token counts. Preserve all instructions/text; never compact to force a fit.
	if len(request.System)+len(request.User)+512 > trialMaxInput {
		return nil, ErrGraderTrial
	}
	o := trialObject{"model": model, "messages": []any{trialObject{"role": "system", "content": request.System}, trialObject{"role": "user", "content": request.User}}, "max_tokens": trialMaxOutput, "stream": false}
	switch model {
	case "deepseek-v4.1-flash":
		o["thinking"] = trialObject{"type": "disabled"}
	case "glm-5.3-flash":
		o["thinking"] = trialObject{"type": "enabled"}
		o["reasoning_effort"] = "low"
	case "claude-haiku-5-5":
		o["system"] = request.System
		o["messages"] = []any{trialObject{"role": "user", "content": request.User}}
		o["thinking"] = trialObject{"type": "disabled"}
		o["output_config"] = trialObject{"effort": "low"}
	default:
		return nil, ErrGraderTrial
	}
	b := trialCompact(o)
	if len(b) > 32<<10 {
		return nil, ErrGraderTrial
	}
	return b, nil
}
func trialEndpoint(model string) string {
	if model == "claude-haiku-5-5" {
		return trialMessagesURL
	}
	for _, m := range trialModels[:2] {
		if model == m {
			return trialChatURL
		}
	}
	return ""
}
func trialBinaryDigest() (string, error) {
	p, e := os.Executable()
	if e != nil {
		return "", ErrGraderTrial
	}
	f, e := os.Open(p)
	if e != nil {
		return "", ErrGraderTrial
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 64<<20+1))
	if e != nil || len(b) > 64<<20 {
		return "", ErrGraderTrial
	}
	return corpusHash(string(b)), nil
}

func PrepareGraderTrial(directory, manifestPin, output string) (GraderTrialPreparation, error) {
	var empty GraderTrialPreparation
	packet, refs, err := loadContractPacket(directory, manifestPin)
	if err != nil || len(packet.Cases) != 30 {
		return empty, ErrGraderTrial
	}
	binary, err := trialBinaryDigest()
	if err != nil {
		return empty, err
	}
	seed := make([]byte, 16)
	if _, err = rand.Read(seed); err != nil {
		return empty, ErrGraderTrial
	}
	p := trialPlan{Issue: 130, PacketManifest: manifestPin, Binary: binary, Seed: hex.EncodeToString(seed), Limits: trialBudget, Files: map[string]string{}, Jobs: []trialJob{}, Controls: []string{}, OwnerRequest: "okay, let's fresh run", AccountBasis: "Owner previously declared Extra Usage OFF; $0 additional spend persists. Account state and remaining quota not independently inspected.", Progression: "Development operability only: all 60 planned responses/profile have valid structure and evidence, completed output and known global decisions, with every planted critical trial detected. Any missed/unknown critical control blocks progression. No automatic S0 grader qualification or semantic accuracy claim."}
	files := map[string][]byte{}
	input, err := contractPrivateRoot(directory)
	if err != nil {
		return empty, ErrGraderTrial
	}
	defer input.Close()
	for _, name := range []string{"packet.json", "references.json", "manifest.json"} {
		b, e := privateFile(input, name, 2<<20)
		if e != nil {
			return empty, ErrGraderTrial
		}
		files[name] = b
		p.Files[name] = corpusHash(string(b))
	}
	requests := map[string]contractRequest{}
	maxBytes := 0
	for _, req := range packet.Requests {
		requests[req.Case] = req
		n := len(req.System) + len(req.User) + 512
		if n > maxBytes {
			maxBytes = n
		}
		for _, model := range trialModels {
			b, e := trialWire(req, model)
			if e != nil {
				return empty, e
			}
			name := "wire-" + req.Case + "-" + model + ".json"
			files[name] = b
			p.Files[name] = corpusHash(string(b))
		}
	}
	ranked := append([]contractCase{}, packet.Cases...)
	sort.Slice(ranked, func(i, j int) bool { return corpusHash(p.Seed+ranked[i].Case) < corpusHash(p.Seed+ranked[j].Case) })
	for i, c := range ranked {
		ref := refs[c.Case]
		if strings.HasPrefix(ref.OriginalCase, "cal60-") && ref.Critical != nil && *ref.Critical && !contractDisputed(ref, "introduced_critical_error") {
			p.Controls = append(p.Controls, c.Case)
		}
		for turn := 0; turn < 6; turn++ {
			model := trialModels[(turn%3+i)%3]
			p.Jobs = append(p.Jobs, trialJob{c.Case, model, 1 + (turn/3+i%2)%2, "wire-" + c.Case + "-" + model + ".json"})
		}
	}
	if len(p.Controls) != 6 || len(p.Jobs) != 180 {
		return empty, ErrGraderTrial
	}
	if !contractOutputName(output) || os.Mkdir(output, 0o700) != nil {
		return empty, ErrGraderTrial
	}
	root, err := contractPrivateRoot(output)
	if err != nil {
		return empty, ErrGraderTrial
	}
	defer root.Close()
	for name, b := range files {
		if contractWrite(root, name, b) != nil {
			return empty, ErrGraderTrial
		}
	}
	b := contractJSON(p)
	if contractWrite(root, "plan.json", b) != nil {
		return empty, ErrGraderTrial
	}
	return GraderTrialPreparation{corpusHash(string(b)), 30, 180, maxBytes, trialBudget, 0}, nil
}

func loadGraderTrial(directory, pin string) (*os.Root, trialPlan, contractPacket, map[string]contractReference, error) {
	var p trialPlan
	var packet contractPacket
	if !evidenceDigest(pin) {
		return nil, p, packet, nil, ErrGraderTrial
	}
	r, err := contractPrivateRoot(directory)
	if err != nil {
		return nil, p, packet, nil, ErrGraderTrial
	}
	fail := func() (*os.Root, trialPlan, contractPacket, map[string]contractReference, error) {
		r.Close()
		return nil, trialPlan{}, contractPacket{}, nil, ErrGraderTrial
	}
	b, e := privateFile(r, "plan.json", 2<<20)
	if e != nil || corpusHash(string(b)) != pin || decodeStudy(b, &p) != nil || p.Issue != 130 || p.Limits != trialBudget || len(p.Jobs) != 180 || len(p.Files) != 93 || len(p.Controls) != 6 {
		return fail()
	}
	binary, e := trialBinaryDigest()
	if e != nil || binary != p.Binary {
		return fail()
	}
	for name, pin := range p.Files {
		if filepath.Base(name) != name {
			return fail()
		}
		b, e := privateFile(r, name, 2<<20)
		if e != nil || corpusHash(string(b)) != pin {
			return fail()
		}
	}
	packet, refs, e := loadContractPacket(directory, p.PacketManifest)
	if e != nil || len(packet.Cases) != 30 {
		return fail()
	}
	reqs := map[string]contractRequest{}
	for _, req := range packet.Requests {
		reqs[req.Case] = req
	}
	seen := map[string]bool{}
	for _, j := range p.Jobs {
		req, ok := reqs[j.Case]
		if !ok || j.Repetition < 1 || j.Repetition > 2 {
			return fail()
		}
		key := j.Case + "/" + j.Model + "/" + fmt.Sprint(j.Repetition)
		if seen[key] {
			return fail()
		}
		seen[key] = true
		b, e := trialWire(req, j.Model)
		if e != nil || j.Wire != "wire-"+j.Case+"-"+j.Model+".json" || corpusHash(string(b)) != p.Files[j.Wire] {
			return fail()
		}
	}
	return r, p, packet, refs, nil
}

func trialKey() (string, error) {
	for _, name := range []string{"GANESH_S0_GO_API_KEY", "OPENCODE_API_KEY"} {
		if key := os.Getenv(name); key != "" {
			return key, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", ErrGraderTrial
	}
	f, err := os.Open(filepath.Join(home, ".config/crush/secrets.env"))
	if err != nil {
		return "", ErrGraderTrial
	}
	defer f.Close()
	s := bufio.NewScanner(io.LimitReader(f, 1<<20))
	for s.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Text()), "export "))
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != "OPENCODE_API_KEY" {
			continue
		}
		value = strings.TrimSpace(value)
		if len(value) > 1 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		if value == "" || strings.ContainsAny(value, "$` \t\r\n") {
			return "", ErrGraderTrial
		}
		return value, nil
	}
	return "", ErrGraderTrial
}
func trialStateSave(r *os.Root, s trialState) error {
	if contractWrite(r, "state.next.json", contractJSON(s)) != nil {
		return ErrGraderTrial
	}
	if r.Rename("state.next.json", "state.json") != nil {
		return ErrGraderTrial
	}
	return nil
}

// RunGraderTrial is deliberately separate from offline preparation/reporting.
// CLI dispatch additionally requires --execute; there is no default run/resume.
func RunGraderTrial(directory, pin string, out io.Writer) (GraderTrialReport, error) {
	return runGraderTrial(directory, pin, out, trialNewClient(), trialKey)
}
func runGraderTrial(directory, pin string, out io.Writer, client *http.Client, keyFn func() (string, error)) (report GraderTrialReport, runErr error) {
	r, p, packet, refs, err := loadGraderTrial(directory, pin)
	if err != nil {
		return report, err
	}
	defer r.Close()
	// Exclusive start reservation survives crashes and prevents a second run.
	s := trialState{Started: time.Now().UTC().Format(time.RFC3339Nano)}
	if contractWrite(r, "started.json", contractJSON(s)) != nil {
		return report, ErrGraderTrial
	}
	defer func() {
		if !s.Complete && s.Stopped == "" {
			s.Stopped = "local_or_interrupted_failure"
		}
		if trialStateSave(r, s) != nil {
			runErr = ErrGraderTrial
		}
		var e error
		report, e = trialReport(r, p, packet, refs, s)
		if e != nil {
			runErr = ErrGraderTrial
		}
		if contractWrite(r, "report.json", contractJSON(report)) != nil {
			runErr = ErrGraderTrial
		}
	}()
	key, err := keyFn()
	if err != nil {
		s.Stopped = "key_unavailable_no_request"
		return report, ErrGraderTrial
	}
	cases := map[string]contractCase{}
	for _, c := range packet.Cases {
		cases[c.Case] = c
	}
	started, _ := time.Parse(time.RFC3339Nano, s.Started)
	for i, j := range p.Jobs {
		if time.Since(started) > time.Duration(p.Limits.WholeSeconds)*time.Second {
			s.Stopped = "whole_deadline"
			return report, ErrGraderTrial
		}
		name := fmt.Sprintf("attempt-%03d", i+1)
		if r.Mkdir(name, 0o700) != nil {
			s.Stopped = "attempt_reservation"
			return report, ErrGraderTrial
		}
		attempt, err := r.OpenRoot(name)
		if err != nil {
			return report, ErrGraderTrial
		}
		wire, err := privateFile(r, j.Wire, 32<<10)
		if err != nil {
			attempt.Close()
			return report, ErrGraderTrial
		}
		session := fmt.Sprintf("ganesh130-%s-%03d", p.Seed, i+1)
		meta := trialObject{"job": j, "session": session, "started_utc": time.Now().UTC().Format(time.RFC3339Nano), "wire_sha256": corpusHash(string(wire)), "retries": 0}
		if contractWrite(attempt, "attempt.json", contractJSON(meta)) != nil || contractWrite(attempt, "wire.json", wire) != nil {
			attempt.Close()
			return report, ErrGraderTrial
		}
		s.Attempts++
		if trialStateSave(r, s) != nil {
			attempt.Close()
			return report, ErrGraderTrial
		}
		deadline := started.Add(time.Duration(p.Limits.WholeSeconds) * time.Second)
		if d := time.Now().Add(time.Duration(p.Limits.Seconds) * time.Second); d.Before(deadline) {
			deadline = d
		}
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		at := time.Now()
		body, status, transportErr := trialSend(ctx, client, trialEndpoint(j.Model), wire, key, session)
		cancel()
		parsed, parseErr := trialParseResponse(body, j.Model)
		result := trialResult{BodySHA: corpusHash(string(body)), Status: status, ElapsedMS: time.Since(at).Milliseconds(), ReturnedModel: parsed.Model, Finish: parsed.Finish, UsageKnown: parsed.UsageVerified, Usage: parsed.Usage}
		if result.UsageKnown {
			s.Input += parsed.Usage.Input
			s.Output += parsed.Usage.Output
		}
		if transportErr != nil || status != 200 || parseErr != nil {
			s.Stopped = "transport_identity_or_usage_contract"
			result.Stop = s.Stopped
		} else {
			result.CompleteOutput = parsed.Finish == "stop" || parsed.Finish == "end_turn"
			check := checkContractResponse(cases[j.Case], refs[j.Case], []byte(parsed.Text))
			result.Check = &check
		}
		if s.Input > int64(p.Limits.InputTotal) || s.Output > int64(p.Limits.OutputTotal) {
			s.Stopped = "aggregate_token_limit"
			result.Stop = s.Stopped
		}
		if contractWrite(attempt, "native-response.json", body) != nil || contractWrite(attempt, "response.txt", []byte(parsed.Text)) != nil || contractWrite(attempt, "result.json", contractJSON(result)) != nil {
			attempt.Close()
			return report, ErrGraderTrial
		}
		attempt.Close()
		if trialStateSave(r, s) != nil {
			return report, ErrGraderTrial
		}
		if _, err = fmt.Fprintf(out, "attempt=%d/180 model=%s repetition=%d usage_known=%t\n", i+1, j.Model, j.Repetition, result.UsageKnown); err != nil {
			s.Stopped = "progress_output_failure"
			return report, ErrGraderTrial
		}
		if s.Stopped != "" {
			return report, ErrGraderTrial
		}
	}
	s.Complete = true
	return report, nil
}

func trialReport(r *os.Root, p trialPlan, packet contractPacket, refs map[string]contractReference, s trialState) (GraderTrialReport, error) {
	report := GraderTrialReport{Status: "development_comparison_incomplete", State: s, Profiles: map[string]*trialProfile{}, ReferenceBasis: "Same-author proposals; disputed/unknown fields excluded; development cases over six tasks, not independent accuracy evidence.", Limits: p.Limits}
	if s.Complete {
		report.Status = "development_requests_complete_not_grader_qualification"
	}
	controls := map[string]bool{}
	for _, id := range p.Controls {
		controls[id] = true
	}
	for _, model := range trialModels {
		report.Profiles[model] = &trialProfile{Planned: 60, Missing: 60, CriticalPlanned: 12, CriticalUnknown: 12, DeclaredControlUnknown: 12, RepeatPlanned: 30, Issues: map[string]int{}}
	}
	flags := map[string]string{}
	allowances := map[string]*big.Rat{}
	latencies := map[string][]int64{}
	for _, model := range trialModels {
		allowances[model] = new(big.Rat)
	}
	cases := map[string]contractCase{}
	for _, c := range packet.Cases {
		cases[c.Case] = c
	}
	for i, j := range p.Jobs {
		stats := report.Profiles[j.Model]
		if i < s.Attempts {
			stats.Attempted++
		}
		b, err := privateFile(r, fmt.Sprintf("attempt-%03d/result.json", i+1), 2<<20)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return report, ErrGraderTrial
		}
		var result trialResult
		if trialDecode(b, &result) != nil {
			return report, ErrGraderTrial
		}
		raw, e := privateFile(r, fmt.Sprintf("attempt-%03d/native-response.json", i+1), 512<<10)
		if e != nil || corpusHash(string(raw)) != result.BodySHA {
			return report, ErrGraderTrial
		}
		parsed, parseErr := trialParseResponse(raw, j.Model)
		if parsed.UsageVerified != result.UsageKnown || (result.UsageKnown && (parsed.Usage.Input != result.Usage.Input || parsed.Usage.Output != result.Usage.Output || parsed.Usage.Reasoning != result.Usage.Reasoning)) {
			return report, ErrGraderTrial
		}
		result.Usage = parsed.Usage
		if parseErr == nil && result.Status == 200 && result.Stop == "" {
			text, e := privateFile(r, fmt.Sprintf("attempt-%03d/response.txt", i+1), 64<<10)
			if e != nil || string(text) != parsed.Text {
				return report, ErrGraderTrial
			}
			check := checkContractResponse(cases[j.Case], refs[j.Case], text)
			result.Check = &check
			result.CompleteOutput = parsed.Finish == "stop" || parsed.Finish == "end_turn"
		} else {
			result.Check = nil
			result.CompleteOutput = false
		}
		stats.Missing--
		if result.ElapsedMS < 0 {
			return report, ErrGraderTrial
		}
		latencies[j.Model] = append(latencies[j.Model], result.ElapsedMS)
		if result.UsageKnown {
			stats.KnownUsage++
			stats.Input += result.Usage.Input
			stats.Output += result.Usage.Output
			stats.Reasoning += result.Usage.Reasoning
			allowance, ok := new(big.Rat).SetString(result.Usage.Allowance)
			if !ok {
				return report, ErrGraderTrial
			}
			allowances[j.Model].Add(allowances[j.Model], allowance)
		}
		if result.CompleteOutput {
			stats.CompleteOutput++
		}
		if result.Check == nil {
			continue
		}
		check := result.Check
		if result.CompleteOutput && check.StructureValid && controls[j.Case] && check.DeclaredCritical != nil {
			stats.DeclaredControlUnknown--
			if *check.DeclaredCritical {
				stats.DeclaredControlTrue++
			} else {
				stats.DeclaredControlFalse++
			}
		}
		if check.WrapperRemoved {
			stats.Wrapped++
		}
		if check.StructureValid {
			stats.Structure++
		}
		if check.EvidenceValid {
			stats.Evidence++
		}
		for _, issue := range check.Issues {
			stats.Issues[issue]++
		}
		if !result.CompleteOutput || !check.Usable {
			continue
		}
		if check.RawFormat == "plain_json" {
			stats.PlainUsable++
		}
		stats.Compared += check.ReferenceCompared
		stats.Disagreements += check.ReferenceDisagreements
		stats.ReferenceUnknown += check.ReferenceUnresolved
		if controls[j.Case] && check.DeclaredCritical != nil {
			stats.CriticalUnknown--
			if *check.DeclaredCritical {
				stats.CriticalDetected++
			} else {
				stats.CriticalMissed++
			}
		}
		ref := refs[j.Case]
		if ref.Critical != nil && !*ref.Critical && !contractDisputed(ref, "introduced_critical_error") && check.DeclaredCritical != nil && *check.DeclaredCritical {
			stats.FalseCritical++
		}
		if check.DeclaredCompletion != nil && check.DeclaredCritical != nil && check.DeclaredUnsupported != nil {
			stats.KnownJudgments++
			flags[j.Case+"/"+j.Model+"/"+fmt.Sprint(j.Repetition)] = fmt.Sprintf("%t/%t/%t", *check.DeclaredCompletion, *check.DeclaredCritical, *check.DeclaredUnsupported)
		}
	}
	for caseID := range refs {
		for _, model := range trialModels {
			a, aok := flags[caseID+"/"+model+"/1"]
			b, bok := flags[caseID+"/"+model+"/2"]
			if aok && bok {
				report.Profiles[model].RepeatKnown++
				if a != b {
					report.Profiles[model].RepeatChanged++
				}
			}
		}
	}
	for model, stats := range report.Profiles {
		stats.UnknownUsage = stats.Attempted - stats.KnownUsage
		stats.Allowance = allowances[model].RatString()
		values := latencies[model]
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		if len(values) > 0 {
			stats.LatencyP50 = values[(len(values)*50+99)/100-1]
			stats.LatencyP95 = values[(len(values)*95+99)/100-1]
		}
		stats.OperationalPass = stats.CompleteOutput == 60 && stats.Structure == 60 && stats.Evidence == 60 && stats.KnownJudgments == 60 && stats.CriticalDetected == stats.CriticalPlanned
	}
	return report, nil
}
func ReportGraderTrial(directory, pin string) (GraderTrialReport, error) {
	r, p, packet, refs, err := loadGraderTrial(directory, pin)
	if err != nil {
		return GraderTrialReport{}, err
	}
	defer r.Close()
	b, err := privateFile(r, "state.json", 64<<10)
	if err != nil {
		return GraderTrialReport{}, ErrGraderTrial
	}
	var s trialState
	if decodeStudy(b, &s) != nil {
		return GraderTrialReport{}, ErrGraderTrial
	}
	return trialReport(r, p, packet, refs, s)
}
