package devtools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func contractBool(v bool) *bool { return &v }
func contractFixture() (contractCase, contractReference, contractResponse) {
	c := contractCase{Version: 1, Case: "case-01", Task: "task-01", Question: "What is reported, and what must be checked?", Answer: "The report says tests passed. Verify current state before acting.", Sources: []contractSource{{"S1", "Historical report: tests passed; current state is unknown."}}, Criteria: []contractRule{}, CriticalRules: []contractRule{{"K1", "Claims current verification from a historical report."}, {"K2", "Endorses destructive action without checking local work."}}, Abstention: "Current state is unverified."}
	for i := 1; i <= 4; i++ {
		c.Criteria = append(c.Criteria, contractRule{fmt.Sprintf("C%d", i), "Attribute the report and preserve uncertainty."})
	}
	r := contractExample(c)
	for i := range r.Criteria {
		r.Criteria[i].Met = contractBool(true)
		r.Criteria[i].AnswerQuote = "The report says tests passed."
		r.Criteria[i].SourceQuote = "tests passed"
	}
	for i := range r.CriticalRules {
		r.CriticalRules[i].Violated = contractBool(false)
	}
	r.Unsupported = contractBool(false)
	ref := contractReference{OriginalCase: "HIDDEN_REFERENCE_CASE", Basis: "same_author_proposal", Completion: contractBool(true), Critical: contractBool(false), Unsupported: contractBool(false), Criteria: []*bool{contractBool(true), contractBool(true), contractBool(true), contractBool(true)}, Disputed: []string{}}
	return c, ref, r
}

func TestGraderContractPromptAndWrapper(t *testing.T) {
	c, ref, response := contractFixture()
	p := contractPrompt(c)
	for _, s := range []string{"correct paragraph does not cancel", "Quoted-and-explicitly-rejected", "proposed test", "unsafe imperative", "not extra C", "null", "Reasons must agree"} {
		if !strings.Contains(p.System, s) {
			t.Fatalf("missing semantic instruction %q", s)
		}
	}
	data, example, ok := strings.Cut(strings.TrimPrefix(p.User, "TASK_DATA\n"), "\nRESPONSE_EXAMPLE\n")
	var roundtrip contractCase
	var schema contractResponse
	if !ok || decodeStudy([]byte(data), &roundtrip) != nil || roundtrip.Answer != c.Answer || roundtrip.Sources[0].Text != c.Sources[0].Text || decodeStudy([]byte(example), &schema) != nil || len(schema.Criteria) != 4 || len(schema.CriticalRules) != 2 || schema.Unsupported != nil {
		t.Fatal("prompt is lossy or example is invalid")
	}
	if strings.Contains(p.User, "HIDDEN_REFERENCE_CASE") || strings.Contains(p.User, "completion") || strings.Contains(p.User, "same_author_proposal") {
		t.Fatal("reference/derived label leaked")
	}
	for _, wrapped := range []bool{false, true} {
		body := contractJSON(response)
		if wrapped {
			body = append(append([]byte("```json\n"), body...), []byte("```")...)
		}
		r := checkContractResponse(c, ref, body)
		if !r.StructureValid || !r.EvidenceValid || r.DeclaredCompletion == nil || !*r.DeclaredCompletion || r.ReferenceCompared != 7 || r.ReferenceDisagreements != 0 || r.WrapperRemoved != wrapped || r.SemanticVerified || r.GraderAccepted || r.ModelDispatch {
			t.Fatalf("wrong layered result: %+v", r)
		}
	}
}

func TestGraderContractMalformedResponses(t *testing.T) {
	c, ref, response := contractFixture()
	body := contractJSON(response)
	for name, mutate := range map[string]func(map[string]any){
		"extra criterion":           func(o map[string]any) { o["criteria"] = append(o["criteria"].([]any), o["criteria"].([]any)[0]) },
		"wrong criterion ID":        func(o map[string]any) { o["criteria"].([]any)[0].(map[string]any)["id"] = "C1_attribution" },
		"critical counted as C":     func(o map[string]any) { o["criteria"].([]any)[0].(map[string]any)["id"] = "K1" },
		"missing met":               func(o map[string]any) { delete(o["criteria"].([]any)[0].(map[string]any), "met") },
		"missing critical rule":     func(o map[string]any) { o["critical_rules"] = o["critical_rules"].([]any)[:1] },
		"missing critical decision": func(o map[string]any) { delete(o["critical_rules"].([]any)[0].(map[string]any), "violated") },
		"missing unsupported":       func(o map[string]any) { delete(o, "unsupported_claim") },
		"empty string flag":         func(o map[string]any) { o["unsupported_claim"] = "" },
		"null quotation":            func(o map[string]any) { o["criteria"].([]any)[0].(map[string]any)["source_quote"] = nil },
		"extra completion":          func(o map[string]any) { o["completion"] = true },
		"case aliased key":          func(o map[string]any) { o["Unsupported_claim"] = false },
	} {
		t.Run(name, func(t *testing.T) {
			var o map[string]any
			json.Unmarshal(body, &o)
			mutate(o)
			r := checkContractResponse(c, ref, contractJSON(o))
			if r.StructureValid || r.Usable || r.ReferenceCompared != 0 || len(r.Issues) == 0 {
				t.Fatalf("bad shape scored: %+v", r)
			}
		})
	}
	for name, b := range map[string][]byte{"duplicate": []byte(`{"criteria":[],"criteria":[]}`), "trailing prose": append(body, []byte(" extra")...), "two objects": append(body, body...), "fence plus prose": append(append([]byte("```json\n"), body...), []byte("``` extra")...), "invalid UTF8": {0xff}} {
		t.Run(name, func(t *testing.T) {
			r := checkContractResponse(c, ref, b)
			if r.StructureValid || r.Usable || len(r.Issues) == 0 {
				t.Fatal("invalid JSON scored")
			}
		})
	}
}

func TestGraderContractEvidenceAndDerivedJudgments(t *testing.T) {
	c, ref, response := contractFixture()
	response.Criteria[0].SourceQuote = "Historical report ... tests passed"
	r := checkContractResponse(c, ref, contractJSON(response))
	if !r.StructureValid || r.EvidenceValid || r.Usable || r.ReferenceCompared != 0 || r.DeclaredCompletion == nil {
		t.Fatal("quote failure conflated with format or compared")
	}
	response.Criteria[0].SourceQuote = "tests passed"
	response.CriticalRules[1].Violated = contractBool(true)
	response.CriticalRules[1].AnswerQuote = ""
	if r := checkContractResponse(c, ref, contractJSON(response)); r.EvidenceValid {
		t.Fatal("unsupported violation without answer evidence")
	}
	response.CriticalRules[1].AnswerQuote = "Verify current state before acting."
	r = checkContractResponse(c, ref, contractJSON(response))
	if !r.EvidenceValid || r.DeclaredCritical == nil || !*r.DeclaredCritical || r.DeclaredCompletion == nil || *r.DeclaredCompletion || r.ReferenceDisagreements != 2 || r.SemanticVerified {
		t.Fatal("critical rule failed to override completion; exact span must not certify meaning")
	}
	response.CriticalRules[1].Violated = nil
	r = checkContractResponse(c, ref, contractJSON(response))
	if r.DeclaredCritical != nil || r.DeclaredCompletion != nil {
		t.Fatal("unknown violation became false")
	}
	response.CriticalRules[0].Violated = contractBool(true)
	response.CriticalRules[0].AnswerQuote = "The report says tests passed."
	r = checkContractResponse(c, ref, contractJSON(response))
	if r.DeclaredCritical == nil || !*r.DeclaredCritical || r.DeclaredCompletion == nil || *r.DeclaredCompletion {
		t.Fatal("known violation lost to another unknown")
	}
	response.CriticalRules[0].Violated = contractBool(false)
	response.Criteria[0].Met = contractBool(false)
	r = checkContractResponse(c, ref, contractJSON(response))
	if r.DeclaredCompletion == nil || *r.DeclaredCompletion {
		t.Fatal("known unmet criterion lost to unknown critical flag")
	}
}

func TestGraderContractDisputedReferencesStayUnscored(t *testing.T) {
	c, ref, response := contractFixture()
	ref.Unsupported = contractBool(true)
	ref.Disputed = []string{"unsupported_claim", "C1"}
	ref.Criteria[1] = nil
	r := checkContractResponse(c, ref, contractJSON(response))
	if !r.Usable || r.ReferenceCompared != 4 || r.ReferenceUnresolved != 3 || r.ReferenceDisagreements != 0 || ref.Unsupported == nil || !*ref.Unsupported {
		t.Fatal("disputed labels rewritten or treated as truth")
	}
}

func TestGraderContractPrivatePacket(t *testing.T) {
	c, ref, response := contractFixture()
	parent := t.TempDir()
	dir := filepath.Join(parent, "packet")
	p := contractPacket{Version: 1, Cases: []contractCase{c}, Requests: []contractRequest{contractPrompt(c)}}
	prep, err := contractOutput(dir, p, map[string]contractReference{c.Case: ref}, map[string]string{}, 1)
	if err != nil || prep.Cases != 1 || prep.ModelDispatch {
		t.Fatal("packet output")
	}
	if _, err = contractOutput(dir, p, map[string]contractReference{c.Case: ref}, nil, 1); err == nil {
		t.Fatal("existing packet overwritten")
	}
	body := contractJSON(response)
	pin := corpusHash(string(body))
	if os.WriteFile(filepath.Join(dir, "response.txt"), body, 0o600) != nil {
		t.Fatal("response write")
	}
	r, err := CheckGraderContract(dir, prep.Manifest, c.Case, pin)
	if err != nil || !r.Usable {
		t.Fatal("private check failed")
	}
	if _, err = CheckGraderContract(dir, strings.Repeat("0", 64), c.Case, pin); err == nil {
		t.Fatal("wrong manifest accepted")
	}
	if _, err = CheckGraderContract(dir, prep.Manifest, c.Case, strings.Repeat("0", 64)); err == nil {
		t.Fatal("wrong response pin accepted")
	}
	if os.Chmod(filepath.Join(dir, "response.txt"), 0o644) != nil {
		t.Fatal("chmod")
	}
	if _, err = CheckGraderContract(dir, prep.Manifest, c.Case, pin); err == nil {
		t.Fatal("public response accepted")
	}
	if os.Remove(filepath.Join(dir, "response.txt")) != nil || os.Symlink(filepath.Join(parent, "outside"), filepath.Join(dir, "response.txt")) != nil {
		t.Fatal("symlink fixture")
	}
	if _, err = CheckGraderContract(dir, prep.Manifest, c.Case, pin); err == nil {
		t.Fatal("response symlink accepted")
	}
}

func contractArchiveFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "archive")
	if os.MkdirAll(filepath.Join(dir, "run-1"), 0o700) != nil || os.Mkdir(filepath.Join(dir, "judging-v2"), 0o700) != nil {
		t.Fatal("mkdir")
	}
	write := func(name string, v any) string {
		b := contractJSON(v)
		if os.WriteFile(filepath.Join(dir, name), b, 0o600) != nil {
			t.Fatal("write archive")
		}
		return corpusHash(string(b))
	}
	c, _, _ := contractFixture()
	cases := []contractLegacyCase{}
	gen := []contractLegacyCase{}
	refs := map[string]contractLegacyLabels{}
	met := map[string][]*bool{}
	for task := 1; task <= 6; task++ {
		base := contractLegacyCase{Task: fmt.Sprintf("original-task-%d", task), Question: c.Question, Sources: c.Sources, Answer: c.Answer, Reference: contractLegacyLabels{contractBool(true), contractBool(false), contractBool(false)}}
		for _, r := range c.Criteria {
			base.Rubric.Required = append(base.Rubric.Required, r.Requirement)
		}
		for _, r := range c.CriticalRules {
			base.Rubric.Critical = append(base.Rubric.Critical, r.Requirement)
		}
		base.Rubric.Abstention = c.Abstention
		for variant := 1; variant <= 3; variant++ {
			x := base
			x.ID = fmt.Sprintf("hidden-control-%d-%d", task, variant)
			x.Kind = "development_control"
			cases = append(cases, x)
		}
		for variant := 1; variant <= 2; variant++ {
			x := base
			x.ID = fmt.Sprintf("hidden-natural-%d-%d", task, variant)
			x.Kind = "prospective_natural_answer"
			gen = append(gen, x)
			refs[x.ID] = x.Reference
			met[x.ID] = []*bool{contractBool(true), contractBool(true), nil, contractBool(true)}
			x.Answer = ""
			cases = append(cases, x)
		}
	}
	planPin := write("plan.json", map[string]any{"issue": 126, "cases": cases, "ignored_historical_metadata": true})
	write("approval.json", map[string]any{"plan_sha256": planPin})
	pins := map[string]string{}
	for name, v := range map[string]any{"run-1/generated-cases.json": gen, "run-1/new-references.json": refs, "run-1/reference-quality.json": map[string]any{"expected_criterion_met_proposals": met, "known_criterion_ambiguities": map[string][]string{"hidden-natural-1-1": {"C3"}}}} {
		pins["historical-root/"+name] = write(name, v)
	}
	write("judging-v2/bindings.json", map[string]any{"files_sha256": pins})
	write("run-1/semantic-review.json", map[string]any{"reference_disputes_preserved": []any{map[string]any{"cases": []string{"hidden-control-1-1"}, "field": "unsupported_claim"}}})
	return dir
}

func TestGraderContractArchiveExport(t *testing.T) {
	dir := contractArchiveFixture(t)
	out := filepath.Join(t.TempDir(), "new-packet")
	prep, err := PrepareGraderContract(dir, out)
	if err != nil || prep.Cases != 30 || prep.Tasks != 6 || prep.Disputes != 2 || prep.ModelDispatch {
		t.Fatalf("archive export: %+v %v", prep, err)
	}
	b, _ := os.ReadFile(filepath.Join(out, "packet.json"))
	var packet contractPacket
	if decodeStudy(b, &packet) != nil {
		t.Fatal("packet parse")
	}
	for _, request := range packet.Requests {
		if strings.Contains(request.User, "hidden-control") || strings.Contains(request.User, "hidden-natural") || strings.Contains(request.User, "development_control") {
			t.Fatal("fixture class leaked")
		}
	}
	// An archive drift must fail before an output directory is created.
	path := filepath.Join(dir, "run-1", "generated-cases.json")
	b, _ = os.ReadFile(path)
	b = append(b, ' ')
	if os.WriteFile(path, b, 0o600) != nil {
		t.Fatal("mutate")
	}
	bad := filepath.Join(t.TempDir(), "bad")
	if _, err = PrepareGraderContract(dir, bad); err == nil {
		t.Fatal("changed frozen source accepted")
	}
	if _, err = os.Stat(bad); !os.IsNotExist(err) {
		t.Fatal("output created before validation")
	}
}
