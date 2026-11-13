package devtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type trialRoundTrip func(*http.Request) (*http.Response, error)

func (f trialRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func trialTestPlan(t *testing.T) (string, string) {
	t.Helper()
	base, ref, _ := contractFixture()
	packet := contractPacket{Version: 1, Cases: []contractCase{}, Requests: []contractRequest{}}
	refs := map[string]contractReference{}
	for i := 0; i < 30; i++ {
		c := base
		c.Case = fmt.Sprintf("case-%02d", i+1)
		c.Task = fmt.Sprintf("task-%02d", i%6+1)
		packet.Cases = append(packet.Cases, c)
		packet.Requests = append(packet.Requests, contractPrompt(c))
		r := ref
		r.OriginalCase = fmt.Sprintf("cal60-control-%02d", i+1)
		r.Critical = contractBool(i < 6)
		refs[c.Case] = r
	}
	parent := t.TempDir()
	input := filepath.Join(parent, "packet")
	prep, err := contractOutput(input, packet, refs, map[string]string{}, 6)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(parent, "trial")
	p, err := PrepareGraderTrial(input, prep.Manifest, output)
	if err != nil {
		t.Fatal(err)
	}
	if p.Requests != 180 || p.RequestsSent != 0 || p.Limits.Input != 12288 {
		t.Fatal("wrong preparation")
	}
	return output, p.Plan
}
func trialNativeFixture(model, text string) []byte {
	u := trialObject{"cost": json.Number("0"), "opencode": trialObject{"billing_source": "go", "allowance": json.Number("0.0001"), "tokens": trialObject{"input": json.Number("100"), "output": json.Number("20"), "reasoning": json.Number("0"), "cache_read": json.Number("0"), "cache_write_5m": json.Number("0"), "cache_write_1h": json.Number("0")}}}
	if model == "claude-haiku-5-5" {
		u["input_tokens"] = 100
		u["output_tokens"] = 20
		return contractJSON(trialObject{"model": model, "usage": u, "content": []any{trialObject{"type": "text", "text": text}}, "stop_reason": "end_turn"})
	}
	u["prompt_tokens"] = 100
	u["completion_tokens"] = 20
	u["total_tokens"] = 120
	return contractJSON(trialObject{"model": model, "usage": u, "choices": []any{trialObject{"message": trialObject{"content": text}, "finish_reason": "stop"}}})
}

func TestGraderTrialProfilesOrderAndNoDisclosure(t *testing.T) {
	dir, pin := trialTestPlan(t)
	root, p, packet, _, err := loadGraderTrial(dir, pin)
	if err != nil {
		t.Fatal(err)
	}
	root.Close()
	positions := map[string]int{}
	seen := map[string]bool{}
	for i, j := range p.Jobs {
		positions[j.Model+fmt.Sprint(i%6)]++
		key := j.Case + j.Model + fmt.Sprint(j.Repetition)
		if seen[key] {
			t.Fatal("duplicate job")
		}
		seen[key] = true
	}
	for _, n := range positions {
		if n != 10 {
			t.Fatal("model position imbalance")
		}
	}
	for _, model := range trialModels {
		wire, err := trialWire(packet.Requests[0], model)
		if err != nil {
			t.Fatal(err)
		}
		var o trialObject
		trialDecode(wire, &o)
		if strings.Contains(string(wire), "same_author") || strings.Contains(string(wire), "cal60-control") || strings.Contains(string(wire), "HIDDEN_REFERENCE_CASE") {
			t.Fatal("reference disclosed")
		}
		if model == "glm-5.3-flash" && o["thinking"].(map[string]any)["type"] != "enabled" {
			t.Fatal("GLM thinking profile")
		}
		if model == "claude-haiku-5-5" && o["system"] != packet.Requests[0].System {
			t.Fatal("native Messages profile")
		}
	}
	req := packet.Requests[0]
	req.User = strings.Repeat("x", trialMaxInput)
	if _, err := trialWire(req, trialModels[0]); err == nil {
		t.Fatal("oversize input not stopped")
	}
}

func TestGraderTrialCompleteMockBatchAndNoResume(t *testing.T) {
	dir, pin := trialTestPlan(t)
	_, _, response := contractFixture()
	text := string(contractJSON(response))
	sessions := map[string]bool{}
	calls := 0
	client := trialNewClient()
	client.Transport = trialRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		session := r.Header.Get("x-opencode-session")
		if session == "" || sessions[session] || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Fatal("bad session/header")
		}
		sessions[session] = true
		if d, ok := r.Context().Deadline(); !ok || time.Until(d) > 120*time.Second {
			t.Fatal("no bounded deadline")
		}
		var o trialObject
		b, _ := io.ReadAll(r.Body)
		trialDecode(b, &o)
		model := o["model"].(string)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(trialNativeFixture(model, text))), Header: http.Header{}}, nil
	})
	report, err := runGraderTrial(dir, pin, io.Discard, client, func() (string, error) { return "fixture-key", nil })
	if err != nil || calls != 180 || !report.State.Complete || report.State.Input != 18000 || report.GraderAccepted || report.SemanticVerified {
		t.Fatalf("mock run: calls=%d report=%+v err=%v", calls, report.State, err)
	}
	for _, p := range report.Profiles {
		if p.Structure != 60 || p.Evidence != 60 || p.KnownJudgments != 60 || p.CriticalMissed != 12 || p.OperationalPass || p.Missing != 0 || p.RepeatKnown != 30 || p.PlainUsable != 60 || p.DeclaredControlFalse != 12 || p.DeclaredControlUnknown != 0 || p.UnknownUsage != 0 || p.Allowance != "3/500" {
			t.Fatalf("wrong denominators: %+v", p)
		}
	}
	if _, err = runGraderTrial(dir, pin, io.Discard, client, func() (string, error) { t.Fatal("second key read"); return "", nil }); err == nil || calls != 180 {
		t.Fatal("automatic resume/retry")
	}
	if _, err = ReportGraderTrial(dir, pin); err != nil {
		t.Fatal("offline report failed")
	}
	path := filepath.Join(dir, "attempt-001", "native-response.json")
	if os.WriteFile(path, []byte("{}"), 0o600) != nil {
		t.Fatal("mutate")
	}
	if _, err = ReportGraderTrial(dir, pin); err == nil {
		t.Fatal("modified native evidence accepted")
	}
}

func TestGraderTrialTransportFailureStopsAndPreservesUnknown(t *testing.T) {
	dir, pin := trialTestPlan(t)
	calls := 0
	client := trialNewClient()
	client.Transport = trialRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("synthetic network failure")
	})
	report, err := runGraderTrial(dir, pin, io.Discard, client, func() (string, error) { return "fixture-key", nil })
	if err == nil || calls != 1 || report.State.Attempts != 1 || report.State.Complete || report.State.Stopped == "" {
		t.Fatal("failure retried or not recorded")
	}
	known := 0
	for _, p := range report.Profiles {
		known += p.KnownUsage
		if p.CriticalUnknown != 12 || p.OperationalPass {
			t.Fatal("missing critical outcomes hidden")
		}
	}
	if known != 0 {
		t.Fatal("unknown usage became zero-cost success")
	}
}

func TestGraderTrialMalformedContentDoesNotRetryOrStop(t *testing.T) {
	dir, pin := trialTestPlan(t)
	calls := 0
	client := trialNewClient()
	client.Transport = trialRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		b, _ := io.ReadAll(r.Body)
		var o trialObject
		trialDecode(b, &o)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(trialNativeFixture(o["model"].(string), "not JSON"))), Header: http.Header{}}, nil
	})
	report, err := runGraderTrial(dir, pin, io.Discard, client, func() (string, error) { return "fixture-key", nil })
	if err != nil || calls != 180 || !report.State.Complete {
		t.Fatal("bad grading content consumed replacement/stop")
	}
	for _, p := range report.Profiles {
		if p.Structure != 0 || p.KnownUsage != 60 || p.Missing != 0 || p.CriticalUnknown != 12 || p.Issues["invalid_json"] != 60 {
			t.Fatal("malformed judgments not retained")
		}
	}
}

func TestGraderTrialNativeAccountingAndBoundary(t *testing.T) {
	for _, model := range trialModels {
		body := trialNativeFixture(model, "{}")
		p, err := trialParseResponse(body, model)
		if err != nil || !p.UsageVerified || p.Usage.Input != 100 {
			t.Fatal("native fixture rejected")
		}
		var o trialObject
		trialDecode(body, &o)
		delete(o["usage"].(map[string]any), "cost")
		if _, err = trialParseResponse(contractJSON(o), model); err == nil {
			t.Fatal("missing cost accepted")
		}
		trialDecode(body, &o)
		o["usage"].(map[string]any)["cost"] = json.Number("0.01")
		if _, err = trialParseResponse(contractJSON(o), model); err == nil {
			t.Fatal("nonzero cost accepted")
		}
		trialDecode(body, &o)
		o["model"] = "foreign-model"
		p, err = trialParseResponse(contractJSON(o), model)
		if err == nil || !p.UsageVerified {
			t.Fatal("known usage lost on wrong identity")
		}
	}
	var o trialObject
	trialDecode(trialNativeFixture("glm-5.3-flash", "{}"), &o)
	u := o["usage"].(map[string]any)
	u["opencode"].(map[string]any)["tokens"].(map[string]any)["reasoning"] = json.Number("7")
	u["completion_tokens_details"] = trialObject{"reasoning_tokens": json.Number("7")}
	parsed, err := trialParseResponse(contractJSON(o), "glm-5.3-flash")
	if err != nil || parsed.Usage.Reasoning != 7 || parsed.Usage.Output != 20 {
		t.Fatal("reasoning double-counted or lost")
	}
	calls := 0
	client := trialNewClient()
	client.Transport = trialRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://example.invalid"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	_, status, err := trialSend(context.Background(), client, trialChatURL, []byte("{}"), "fixture-key", "session")
	if err != nil || status != 302 || calls != 1 {
		t.Fatal("redirect followed")
	}
	if _, _, err = trialSend(context.Background(), client, "https://example.invalid", nil, "fixture-key", "session"); err == nil || calls != 1 {
		t.Fatal("foreign host received key")
	}
}
