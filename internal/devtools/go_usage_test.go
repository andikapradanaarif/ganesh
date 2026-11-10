package devtools

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func usageFixture() map[string]any {
	return map[string]any{
		"prompt_tokens": 1787, "completion_tokens": 290, "total_tokens": 2077, "cost": 0,
		"prompt_tokens_details":     map[string]any{"cached_tokens": 512},
		"completion_tokens_details": map[string]any{"reasoning_tokens": 0},
		"opencode":                  map[string]any{"billing_source": "go", "allowance": json.Number("0.00022007"), "tokens": map[string]any{"input": 1275, "output": 290, "reasoning": 0, "cache_read": 512, "cache_write_5m": 0, "cache_write_1h": 0}},
	}
}

func usageTokens(u map[string]any) map[string]any {
	return u["opencode"].(map[string]any)["tokens"].(map[string]any)
}

func TestGoUsageCachedAndOptionalMetadata(t *testing.T) {
	u := usageFixture()
	u["vendor_extra"] = map[string]any{"note": "ghp_SYNTHETIC_SECRET", "path": "/private/synthetic-canary"}
	delete(u, "completion_tokens_details")
	delete(usageTokens(u), "cache_write_5m")
	delete(usageTokens(u), "cache_write_1h")
	body := evidenceJSON(t, u)
	report, err := ParseGoUsage(body)
	if err != nil || report.Status != "reconciled_zero_cost_go" || report.PromptTokens != 1787 || report.UncachedInputTokens != 1275 || report.CacheReadTokens == nil || *report.CacheReadTokens != 512 || report.OutputTokens != 290 || report.TotalTokens != 2077 || report.ReasoningTokens != 0 || report.StandardReasoningTokens != nil || report.CacheWrite5mTokens != nil || report.CacheWrite1hTokens != nil || report.ReportedAllowance != "0.00022007" || report.ReportedCost != "0" || report.UsageSHA256 != devinDigest(body) {
		t.Fatal("cached/optional usage reconciliation failed")
	}
	if report.ModelIdentityVerified || report.GradingValidated || report.CalibrationAccepted || report.ModelDispatchAuthorized {
		t.Fatal("usage accepted as grading or authority")
	}
	for _, private := range []string{"ghp_", "private/synthetic", "vendor_extra", "note"} {
		if strings.Contains(string(evidenceJSON(t, report)), private) {
			t.Fatal("uninterpreted metadata leaked")
		}
	}
	// The redundant standard reasoning counter is optional at either level.
	u["completion_tokens_details"] = nil
	if _, err := ParseGoUsage(evidenceJSON(t, u)); err != nil {
		t.Fatal("null optional details rejected")
	}
	u["completion_tokens_details"] = map[string]any{}
	if _, err := ParseGoUsage(evidenceJSON(t, u)); err != nil {
		t.Fatal("absent optional reasoning rejected")
	}
}

func TestGoUsageCounterVariants(t *testing.T) {
	for _, name := range []string{"uncached with zero buckets", "uncached with absent buckets", "all cached", "write buckets", "optional standard cache absent", "optional standard cache null", "maximum prompt without overflow", "zero cost exponent", "negative zero allowance", "precise allowance", "tiny positive allowance"} {
		t.Run(name, func(t *testing.T) {
			u := usageFixture()
			tokens := usageTokens(u)
			switch name {
			case "uncached with zero buckets":
				tokens["input"] = 1787
				tokens["cache_read"] = 0
				u["prompt_tokens_details"].(map[string]any)["cached_tokens"] = 0
			case "uncached with absent buckets":
				tokens["input"] = 1787
				delete(tokens, "cache_read")
				delete(tokens, "cache_write_5m")
				delete(tokens, "cache_write_1h")
				delete(u, "prompt_tokens_details")
			case "all cached":
				tokens["input"] = 0
				tokens["cache_read"] = 1787
				u["prompt_tokens_details"].(map[string]any)["cached_tokens"] = 1787
			case "write buckets":
				tokens["input"] = 1000
				tokens["cache_read"] = 512
				tokens["cache_write_5m"] = 100
				tokens["cache_write_1h"] = 175
			case "optional standard cache absent":
				delete(u, "prompt_tokens_details")
			case "optional standard cache null":
				u["prompt_tokens_details"] = nil
			case "maximum prompt without overflow":
				u["prompt_tokens"] = int64(math.MaxInt64 - 290)
				u["total_tokens"] = int64(math.MaxInt64)
				tokens["input"] = int64(math.MaxInt64 - 802)
			case "zero cost exponent":
				u["cost"] = json.Number("-0.000e+9999")
			case "negative zero allowance":
				u["opencode"].(map[string]any)["allowance"] = json.Number("-0.00")
			case "precise allowance":
				u["opencode"].(map[string]any)["allowance"] = json.Number("0.123456789012345678901234567890")
			case "tiny positive allowance":
				u["opencode"].(map[string]any)["allowance"] = json.Number("1e-9999")
			}
			report, err := ParseGoUsage(evidenceJSON(t, u))
			if err != nil {
				t.Fatal("valid counter representation rejected")
			}
			if name == "precise allowance" && report.ReportedAllowance != "0.123456789012345678901234567890" {
				t.Fatal("allowance rounded")
			}
			if name == "tiny positive allowance" && report.ReportedAllowance != "1e-9999" {
				t.Fatal("allowance underflow changed reported value")
			}
		})
	}
}

func TestGoUsageRejectsInvalidAccounting(t *testing.T) {
	mutations := map[string]func(map[string]any){
		"missing prompt": func(u map[string]any) { delete(u, "prompt_tokens") },
		"missing output": func(u map[string]any) { delete(u, "completion_tokens") },
		"missing total":  func(u map[string]any) { delete(u, "total_tokens") },
		"null prompt":    func(u map[string]any) { u["prompt_tokens"] = nil },
		"zero prompt":    func(u map[string]any) { u["prompt_tokens"] = 0 },
		"zero output":    func(u map[string]any) { u["completion_tokens"] = 0 },
		"output cap":     func(u map[string]any) { u["completion_tokens"] = 2049 },
		"float count":    func(u map[string]any) { u["prompt_tokens"] = json.Number("1787.0") },
		"string count":   func(u map[string]any) { u["prompt_tokens"] = "1787" },
		"bad total":      func(u map[string]any) { u["total_tokens"] = 2078 },
		"wrapped total": func(u map[string]any) {
			u["prompt_tokens"] = int64(math.MaxInt64)
			u["total_tokens"] = int64(math.MinInt64 + 289)
		},
		"missing cost":                 func(u map[string]any) { delete(u, "cost") },
		"null cost":                    func(u map[string]any) { u["cost"] = nil },
		"string cost":                  func(u map[string]any) { u["cost"] = "0" },
		"nonzero cost":                 func(u map[string]any) { u["cost"] = 1 },
		"negative cost":                func(u map[string]any) { u["cost"] = -1 },
		"tiny nonzero cost":            func(u map[string]any) { u["cost"] = json.Number("1e-9999") },
		"negative tiny cost":           func(u map[string]any) { u["cost"] = json.Number("-1e-9999") },
		"missing allowance":            func(u map[string]any) { delete(u["opencode"].(map[string]any), "allowance") },
		"string allowance":             func(u map[string]any) { u["opencode"].(map[string]any)["allowance"] = "0.01" },
		"negative allowance":           func(u map[string]any) { u["opencode"].(map[string]any)["allowance"] = json.Number("-1e-9999") },
		"overflow allowance":           func(u map[string]any) { u["opencode"].(map[string]any)["allowance"] = json.Number("1e9999") },
		"non-Go billing":               func(u map[string]any) { u["opencode"].(map[string]any)["billing_source"] = "balance" },
		"missing Go input":             func(u map[string]any) { delete(usageTokens(u), "input") },
		"missing Go output":            func(u map[string]any) { delete(usageTokens(u), "output") },
		"missing Go reasoning":         func(u map[string]any) { delete(usageTokens(u), "reasoning") },
		"null Go reasoning":            func(u map[string]any) { usageTokens(u)["reasoning"] = nil },
		"nonzero Go reasoning":         func(u map[string]any) { usageTokens(u)["reasoning"] = 1 },
		"negative Go input":            func(u map[string]any) { usageTokens(u)["input"] = -1 },
		"Go input exceeds prompt":      func(u map[string]any) { usageTokens(u)["input"] = 1788 },
		"Go output mismatch":           func(u map[string]any) { usageTokens(u)["output"] = 289 },
		"standard reasoning mismatch":  func(u map[string]any) { u["completion_tokens_details"].(map[string]any)["reasoning_tokens"] = 1 },
		"standard cache mismatch":      func(u map[string]any) { u["prompt_tokens_details"].(map[string]any)["cached_tokens"] = 511 },
		"cache negative":               func(u map[string]any) { usageTokens(u)["cache_read"] = -512 },
		"cache excess":                 func(u map[string]any) { usageTokens(u)["cache_read"] = 513 },
		"cache missing positive delta": func(u map[string]any) { delete(usageTokens(u), "cache_read") },
		"cache fraction":               func(u map[string]any) { usageTokens(u)["cache_read"] = 512.5 },
		"cache write overflow":         func(u map[string]any) { usageTokens(u)["cache_write_5m"] = int64(math.MaxInt64) },
		"alias root":                   func(u map[string]any) { u["Prompt_Tokens"] = 1787 },
		"alias Go":                     func(u map[string]any) { u["opencode"].(map[string]any)["Billing_Source"] = "go" },
		"alias tokens":                 func(u map[string]any) { usageTokens(u)["Input"] = 1275 },
		"alias optional details":       func(u map[string]any) { u["completion_tokens_details"].(map[string]any)["Reasoning_Tokens"] = 0 },
		"wrong optional object":        func(u map[string]any) { u["completion_tokens_details"] = []any{} },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			u := usageFixture()
			mutate(u)
			report, err := ParseGoUsage(evidenceJSON(t, u))
			if err != ErrGoUsage || !reflect.DeepEqual(report, GoUsageReport{}) {
				t.Fatal("invalid accounting or partial report admitted")
			}
		})
	}
	base := evidenceJSON(t, usageFixture())
	for _, body := range [][]byte{
		append(append([]byte{}, base...), []byte(` {}`)...),
		bytes.Replace(base, []byte(`"cost":0`), []byte(`"cost":0,"co\u0073t":0`), 1),
		bytes.Replace(base, []byte(`"reasoning":0`), []byte(`"reasoning":0,"reasoning":1`), 1),
		[]byte(`null`), []byte(`[]`), []byte(`{"prompt_tokens":NaN}`),
		bytes.Repeat([]byte("x"), goUsageLimit+1),
		append([]byte(`{"extra":"`), 0xff, '"', '}'),
	} {
		if report, err := ParseGoUsage(body); err != ErrGoUsage || !reflect.DeepEqual(report, GoUsageReport{}) {
			t.Fatal("invalid JSON admitted")
		}
	}
}

func TestGoUsagePrivateReader(t *testing.T) {
	for _, name := range []string{"valid", "missing", "mode", "dir mode", "symlink", "dir symlink", "nonregular", "oversize", "pin", "invalid pin"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if os.Chmod(dir, 0o700) != nil {
				t.Fatal("directory mode")
			}
			body := evidenceJSON(t, usageFixture())
			pin := devinDigest(body)
			path := filepath.Join(dir, "go-usage.json")
			if os.WriteFile(path, body, 0o600) != nil {
				t.Fatal("write")
			}
			switch name {
			case "missing":
				if os.Remove(path) != nil {
					t.Fatal("remove")
				}
			case "mode":
				if os.Chmod(path, 0o644) != nil {
					t.Fatal("mode")
				}
			case "dir mode":
				if os.Chmod(dir, 0o755) != nil {
					t.Fatal("mode")
				}
			case "symlink":
				if os.Rename(path, path+".saved") != nil || os.Symlink(path+".saved", path) != nil {
					t.Fatal("symlink")
				}
			case "dir symlink":
				link := dir + "-link"
				if os.Symlink(dir, link) != nil {
					t.Fatal("symlink")
				}
				t.Cleanup(func() { os.Remove(link) })
				dir = link
			case "nonregular":
				if os.Remove(path) != nil || os.Mkdir(path, 0o700) != nil {
					t.Fatal("directory file")
				}
			case "oversize":
				body = bytes.Repeat([]byte("x"), goUsageLimit+1)
				pin = devinDigest(body)
				if os.WriteFile(path, body, 0o600) != nil {
					t.Fatal("write")
				}
			case "pin":
				pin = strings.Repeat("0", 64)
			case "invalid pin":
				pin = strings.Repeat("G", 64)
			}
			report, err := CheckGoUsage(dir, pin)
			if name == "valid" {
				if err != nil || report.CacheReadTokens == nil || *report.CacheReadTokens != 512 {
					t.Fatal("valid private usage")
				}
				return
			}
			if err != ErrGoUsage || !reflect.DeepEqual(report, GoUsageReport{}) || strings.Contains(err.Error(), dir) {
				t.Fatal("unsafe file or private diagnostic")
			}
		})
	}
}
