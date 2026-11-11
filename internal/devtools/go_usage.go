package devtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

var ErrGoUsage = errors.New("private bounded zero-cost Go usage required")

const goUsageLimit = 32 << 10

type goUsageJSON struct {
	Prompt  *int64          `json:"prompt_tokens"`
	Output  *int64          `json:"completion_tokens"`
	Total   *int64          `json:"total_tokens"`
	Cost    json.RawMessage `json:"cost"`
	Details *struct {
		Reasoning *int64 `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
	PromptDetails *struct {
		Cached *int64 `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
	Go struct {
		Source    string          `json:"billing_source"`
		Allowance json.RawMessage `json:"allowance"`
		Tokens    struct {
			Input       *int64 `json:"input"`
			Output      *int64 `json:"output"`
			Reasoning   *int64 `json:"reasoning"`
			CacheRead   *int64 `json:"cache_read"`
			CacheWrite5 *int64 `json:"cache_write_5m"`
			CacheWrite1 *int64 `json:"cache_write_1h"`
		} `json:"tokens"`
	} `json:"opencode"`
}

type GoUsageReport struct {
	Status                  string `json:"status"`
	PromptTokens            int64  `json:"prompt_tokens"`
	OutputTokens            int64  `json:"output_tokens"`
	TotalTokens             int64  `json:"total_tokens"`
	UncachedInputTokens     int64  `json:"uncached_input_tokens"`
	CacheReadTokens         *int64 `json:"cache_read_tokens"`
	CacheWrite5mTokens      *int64 `json:"cache_write_5m_tokens"`
	CacheWrite1hTokens      *int64 `json:"cache_write_1h_tokens"`
	ReasoningTokens         int64  `json:"reasoning_tokens"`
	StandardReasoningTokens *int64 `json:"standard_reasoning_tokens"`
	StandardCacheReadTokens *int64 `json:"standard_cache_read_tokens"`
	BillingSource           string `json:"billing_source"`
	ReportedCost            string `json:"reported_cost"`
	ReportedAllowance       string `json:"reported_allowance"`
	UsageSHA256             string `json:"usage_sha256"`
	ModelIdentityVerified   bool   `json:"model_identity_verified"`
	GradingValidated        bool   `json:"grading_validated"`
	CalibrationAccepted     bool   `json:"calibration_accepted"`
	ModelDispatchAuthorized bool   `json:"model_dispatch_authorized"`
}

// Provider metadata may grow independently of the accounting fields. Ignore
// unrelated keys, but reject aliases of every field used for this decision.
// The whole object has already passed duplicate/depth/UTF-8 checks.
func goUsageKeys(body []byte, known ...string) (map[string]json.RawMessage, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '{' {
		return nil, false
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil {
		return nil, false
	}
	for key := range object {
		for _, name := range known {
			if key != name && strings.EqualFold(key, name) {
				return nil, false
			}
		}
	}
	return object, true
}

func goUsageNumber(body []byte) (string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || (body[0] != '-' && (body[0] < '0' || body[0] > '9')) {
		return "", false
	}
	// Valid JSON was checked before entry; a numeric prefix therefore means a
	// JSON number, never a numeric string, bool, object or absent value.
	return string(body), true
}

func goUsageZero(number string) bool {
	mantissa, _, _ := strings.Cut(strings.ToLower(number), "e")
	return strings.Trim(mantissa, "-0.") == ""
}

func reconcileGoInput(prompt, uncached int64, read, write5, write1, standardRead *int64) bool {
	if prompt <= 0 || uncached < 0 || uncached > prompt {
		return false
	}
	remaining := prompt - uncached
	for _, counter := range []*int64{read, write5, write1} {
		if counter == nil {
			continue // absent counters remain null in the report
		}
		if *counter < 0 || *counter > remaining {
			return false
		}
		remaining -= *counter
	}
	if remaining != 0 {
		return false
	}
	if standardRead != nil {
		if *standardRead < 0 || *standardRead > prompt {
			return false
		}
		if read == nil {
			return *standardRead == 0
		}
		return *standardRead == *read
	}
	return true
}

// ParseGoUsage reconciles one native Chat Completions usage object for the
// bounded zero-reasoning Go grader profile. It does not validate the enclosing
// response, model identity, grading, provenance or authority for another call.
func ParseGoUsage(body []byte) (report GoUsageReport, resultErr error) {
	defer func() {
		if resultErr != nil {
			report = GoUsageReport{}
		}
	}()
	if len(body) > goUsageLimit || !utf8.Valid(body) || uniqueJSON(body) != nil {
		return report, ErrGoUsage
	}
	root, ok := goUsageKeys(body, "prompt_tokens", "completion_tokens", "total_tokens", "cost", "opencode", "completion_tokens_details", "prompt_tokens_details")
	if !ok {
		return report, ErrGoUsage
	}
	g, ok := goUsageKeys(root["opencode"], "billing_source", "allowance", "tokens")
	if !ok {
		return report, ErrGoUsage
	}
	if _, ok := goUsageKeys(g["tokens"], "input", "output", "reasoning", "cache_read", "cache_write_5m", "cache_write_1h"); !ok {
		return report, ErrGoUsage
	}
	for field, keys := range map[string][]string{"completion_tokens_details": {"reasoning_tokens"}, "prompt_tokens_details": {"cached_tokens"}} {
		if value, exists := root[field]; exists && !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if _, ok := goUsageKeys(value, keys...); !ok {
				return report, ErrGoUsage
			}
		}
	}
	var usage goUsageJSON
	if json.Unmarshal(body, &usage) != nil || usage.Prompt == nil || usage.Output == nil || usage.Total == nil || usage.Go.Tokens.Input == nil || usage.Go.Tokens.Output == nil || usage.Go.Tokens.Reasoning == nil {
		return report, ErrGoUsage
	}
	cost, validCost := goUsageNumber(usage.Cost)
	allowance, validAllowance := goUsageNumber(usage.Go.Allowance)
	if !validCost || !goUsageZero(cost) || !validAllowance || (strings.HasPrefix(allowance, "-") && !goUsageZero(allowance)) {
		return report, ErrGoUsage
	}
	// Reject out-of-range/nonfinite allowance values, but preserve the original
	// number text rather than rounding it in the report. Never float-test cost:
	// a tiny nonzero charge could underflow to zero.
	amount, err := strconv.ParseFloat(allowance, 64)
	if err != nil || math.IsInf(amount, 0) || usage.Go.Source != "go" || *usage.Prompt <= 0 || *usage.Output <= 0 || *usage.Output > 2048 || *usage.Total < *usage.Prompt || *usage.Total-*usage.Prompt != *usage.Output || *usage.Go.Tokens.Output != *usage.Output || *usage.Go.Tokens.Reasoning != 0 {
		return report, ErrGoUsage
	}
	if usage.Details != nil {
		report.StandardReasoningTokens = usage.Details.Reasoning
		if usage.Details.Reasoning != nil && *usage.Details.Reasoning != 0 {
			return report, ErrGoUsage
		}
	}
	if usage.PromptDetails != nil {
		report.StandardCacheReadTokens = usage.PromptDetails.Cached
	}
	if !reconcileGoInput(*usage.Prompt, *usage.Go.Tokens.Input, usage.Go.Tokens.CacheRead, usage.Go.Tokens.CacheWrite5, usage.Go.Tokens.CacheWrite1, report.StandardCacheReadTokens) {
		return report, ErrGoUsage
	}
	report.Status = "reconciled_zero_cost_go"
	report.PromptTokens, report.OutputTokens, report.TotalTokens = *usage.Prompt, *usage.Output, *usage.Total
	report.UncachedInputTokens = *usage.Go.Tokens.Input
	report.CacheReadTokens, report.CacheWrite5mTokens, report.CacheWrite1hTokens = usage.Go.Tokens.CacheRead, usage.Go.Tokens.CacheWrite5, usage.Go.Tokens.CacheWrite1
	report.ReasoningTokens, report.BillingSource = *usage.Go.Tokens.Reasoning, "go"
	report.ReportedCost, report.ReportedAllowance, report.UsageSHA256 = cost, allowance, devinDigest(body)
	// All identity, grading, acceptance and dispatch flags intentionally remain false.
	return report, nil
}

func CheckGoUsage(directory, expectedUsage string) (GoUsageReport, error) {
	if !evidenceDigest(expectedUsage) {
		return GoUsageReport{}, ErrGoUsage
	}
	directory = filepath.Clean(directory)
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return GoUsageReport{}, ErrGoUsage
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return GoUsageReport{}, ErrGoUsage
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o700 {
		return GoUsageReport{}, ErrGoUsage
	}
	body, err := privateFile(root, "go-usage.json", goUsageLimit)
	if err != nil || devinDigest(body) != expectedUsage {
		return GoUsageReport{}, ErrGoUsage
	}
	return ParseGoUsage(body)
}
