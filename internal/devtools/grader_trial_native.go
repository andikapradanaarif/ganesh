// Native parser/transport adapted from the preserved #126 Go driver (SHA-256 ffa5a65f30db62d26896b61e3f1feac63cf25c8df97f5814b15d204962c1c9e8).
// Only naming, the prospective input ceiling, allowed grading routes and user agent change.
package devtools

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type trialObject = map[string]any

const trialChatURL = "https://opencode.ai/inference/go/openai/v1/chat/completions"
const trialMessagesURL = "https://opencode.ai/inference/go/anthropic/v1/messages"
const trialGenerator = "gpt-6-luna" // Parsing compatibility only; this runner never generates answers.
type trialUsage struct {
	Input     int64           `json:"input_tokens"`
	Output    int64           `json:"output_tokens_including_reasoning"`
	Reasoning int64           `json:"reasoning_tokens"`
	Cost      string          `json:"reported_cost"`
	Allowance string          `json:"reported_allowance"`
	Billing   string          `json:"billing_source"`
	Native    json.RawMessage `json:"native"`
}

type trialParsed struct {
	Text          string     `json:"text"`
	Model         string     `json:"model"`
	Finish        string     `json:"finish"`
	Usage         trialUsage `json:"usage"`
	UsageVerified bool       `json:"usage_verified"`
}

func trialCompact(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(ErrGraderTrial)
	}
	return b
}

func trialDecode(b []byte, v any) error {
	if len(b) > 512<<10 {
		return ErrGraderTrial
	}
	if e := trialUniqueJSON(b); e != nil {
		return e
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	return d.Decode(v)
}

func trialUniqueJSON(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 64 {
			return ErrGraderTrial
		}
		t, e := d.Token()
		if e != nil {
			return ErrGraderTrial
		}
		if x, ok := t.(json.Delim); ok {
			switch x {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					s, ok := k.(string)
					if e != nil || !ok || seen[strings.ToLower(s)] {
						return ErrGraderTrial
					}
					seen[strings.ToLower(s)] = true
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
				t, e = d.Token()
				if e != nil || t != json.Delim('}') {
					return ErrGraderTrial
				}
			case '[':
				for d.More() {
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
				t, e = d.Token()
				if e != nil || t != json.Delim(']') {
					return ErrGraderTrial
				}
			default:
				return ErrGraderTrial
			}
		}
		return nil
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return ErrGraderTrial
	}
	return nil
}

func trialNumber(o trialObject, k string, required bool) (int64, error) {
	v, ok := o[k]
	if !ok && !required {
		return 0, nil
	}
	n, ok := v.(json.Number)
	if !ok {
		return 0, ErrGraderTrial
	}
	i, e := n.Int64()
	if e != nil || i < 0 || i > 1<<30 {
		return 0, ErrGraderTrial
	}
	return i, nil
}

func trialNumericString(o trialObject, k string, zero bool) (string, error) {
	n, ok := o[k].(json.Number)
	if !ok {
		return "", ErrGraderTrial
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok || r.Sign() < 0 || zero && r.Sign() != 0 {
		return "", ErrGraderTrial
	}
	return n.String(), nil
}

func trialValidateUsage(native trialObject, kind string) (trialUsage, error) {
	var u trialUsage
	u.Native = trialCompact(native)
	g, ok := native["opencode"].(map[string]any)
	if !ok {
		return u, ErrGraderTrial
	}
	u.Billing, _ = g["billing_source"].(string)
	if u.Billing != "go" && u.Billing != "go-plus" {
		return u, ErrGraderTrial
	}
	var e error
	u.Cost, e = trialNumericString(native, "cost", true)
	if e != nil {
		return u, e
	}
	u.Allowance, e = trialNumericString(g, "allowance", false)
	if e != nil {
		return u, e
	}
	tokens, ok := g["tokens"].(map[string]any)
	if !ok {
		return u, ErrGraderTrial
	}
	in, e := trialNumber(tokens, "input", true)
	if e != nil {
		return u, e
	}
	out, e := trialNumber(tokens, "output", true)
	if e != nil {
		return u, e
	}
	reason, e := trialNumber(tokens, "reasoning", true)
	if e != nil {
		return u, e
	}
	cache := int64(0)
	for _, k := range []string{"cache_read", "cache_write_5m", "cache_write_1h"} {
		n, e := trialNumber(tokens, k, false)
		if e != nil {
			return u, e
		}
		cache += n
	}
	u.Input = in + cache
	u.Output = out
	u.Reasoning = reason
	if u.Input <= 0 || u.Input > trialMaxInput || out <= 0 || out > trialMaxOutput || reason > out {
		return u, ErrGraderTrial
	}
	var standardIn, standardOut int64
	switch kind {
	case "chat":
		standardIn, e = trialNumber(native, "prompt_tokens", true)
		if e != nil {
			return u, e
		}
		standardOut, e = trialNumber(native, "completion_tokens", true)
		if e != nil {
			return u, e
		}
		total, e := trialNumber(native, "total_tokens", true)
		if e != nil || total != standardIn+standardOut {
			return u, ErrGraderTrial
		}
	case "messages":
		standardIn, e = trialNumber(native, "input_tokens", true)
		if e != nil {
			return u, e
		}
		for _, k := range []string{"cache_read_input_tokens", "cache_creation_input_tokens"} {
			n, e := trialNumber(native, k, false)
			if e != nil {
				return u, e
			}
			standardIn += n
		}
		standardOut, e = trialNumber(native, "output_tokens", true)
		if e != nil {
			return u, e
		}
	case "responses":
		standardIn, e = trialNumber(native, "input_tokens", true)
		if e != nil {
			return u, e
		}
		standardOut, e = trialNumber(native, "output_tokens", true)
		if e != nil {
			return u, e
		}
		total, e := trialNumber(native, "total_tokens", true)
		if e != nil || total != standardIn+standardOut {
			return u, ErrGraderTrial
		}
	default:
		return u, ErrGraderTrial
	}
	if standardIn != u.Input || standardOut != out {
		return u, ErrGraderTrial
	}
	if kind == "chat" && reason == 0 && u.Billing == "go" && u.Input <= 8192 {
		if _, e := ParseGoUsage(u.Native); e != nil {
			return u, e
		}
	}
	for _, key := range []string{"completion_tokens_details", "output_tokens_details"} {
		if d, ok := native[key].(map[string]any); ok {
			if v, exists := d["reasoning_tokens"]; exists {
				n, e := trialNumber(trialObject{"n": v}, "n", true)
				if e != nil || n != reason {
					return u, ErrGraderTrial
				}
			}
		}
	}
	return u, nil
}

func trialModelMatches(requested, returned string) bool {
	if requested == returned {
		return true
	}
	if !strings.HasPrefix(returned, requested+"-") {
		return false
	}
	_, e := time.Parse("2006-01-02", strings.TrimPrefix(returned, requested+"-"))
	return e == nil
}

func trialParseResponse(body []byte, model string) (trialParsed, error) {
	var p trialParsed
	var o trialObject
	if trialDecode(body, &o) != nil {
		return p, ErrGraderTrial
	}
	p.Model, _ = o["model"].(string)
	native, ok := o["usage"].(map[string]any)
	if !ok {
		return p, ErrGraderTrial
	}
	kind := "chat"
	if model == trialGenerator {
		kind = "responses"
	}
	if model == "claude-haiku-5-5" {
		kind = "messages"
	}
	u, e := trialValidateUsage(native, kind)
	p.Usage = u
	if e != nil {
		return p, e
	}
	p.UsageVerified = true
	if !trialModelMatches(model, p.Model) {
		return p, ErrGraderTrial
	}
	switch kind {
	case "chat":
		xs, ok := o["choices"].([]any)
		if !ok || len(xs) != 1 {
			return p, ErrGraderTrial
		}
		c, ok := xs[0].(map[string]any)
		if !ok {
			return p, ErrGraderTrial
		}
		m, ok := c["message"].(map[string]any)
		if !ok {
			return p, ErrGraderTrial
		}
		if t, ok := m["tool_calls"].([]any); ok && len(t) > 0 {
			return p, ErrGraderTrial
		}
		p.Text, _ = m["content"].(string)
		p.Finish, _ = c["finish_reason"].(string)
	case "messages":
		xs, ok := o["content"].([]any)
		if !ok {
			return p, ErrGraderTrial
		}
		for _, v := range xs {
			m, ok := v.(map[string]any)
			if !ok || m["type"] != "text" {
				return p, ErrGraderTrial
			}
			s, ok := m["text"].(string)
			if !ok {
				return p, ErrGraderTrial
			}
			p.Text += s
		}
		p.Finish, _ = o["stop_reason"].(string)
	case "responses":
		p.Finish, _ = o["status"].(string)
		xs, ok := o["output"].([]any)
		if !ok {
			return p, ErrGraderTrial
		}
		for _, v := range xs {
			m, ok := v.(map[string]any)
			if !ok {
				return p, ErrGraderTrial
			}
			if m["type"] == "reasoning" {
				continue
			}
			if m["type"] != "message" || m["role"] != "assistant" {
				return p, ErrGraderTrial
			}
			ys, ok := m["content"].([]any)
			if !ok {
				return p, ErrGraderTrial
			}
			for _, v := range ys {
				n, ok := v.(map[string]any)
				if !ok || n["type"] != "output_text" {
					return p, ErrGraderTrial
				}
				s, ok := n["text"].(string)
				if !ok {
					return p, ErrGraderTrial
				}
				p.Text += s
			}
		}
	}
	if len(p.Text) > 64<<10 {
		return p, ErrGraderTrial
	}
	return p, nil
}

func trialNewClient() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true, ForceAttemptHTTP2: false, TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: 120 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Timeout: 120 * time.Second}
}

func trialSend(ctx context.Context, client *http.Client, url string, b []byte, key, session string) ([]byte, int, error) {
	if url != trialChatURL && url != trialMessagesURL {
		return nil, 0, ErrGraderTrial
	}
	req, e := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if e != nil {
		return nil, 0, ErrGraderTrial
	}
	req.GetBody = nil
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Ganesh-S0/130 (owner-run corrected contract)")
	req.Header.Set("x-opencode-session", session)
	if url == trialMessagesURL {
		req.Header.Set("anthropic-version", "2023-06-01")
	}
	r, e := client.Do(req)
	if e != nil {
		return nil, 0, ErrGraderTrial
	}
	defer r.Body.Close()
	body, e := io.ReadAll(io.LimitReader(r.Body, (512<<10)+1))
	if e != nil || len(body) > 512<<10 {
		return body[:min(len(body), 512<<10)], r.StatusCode, ErrGraderTrial
	}
	return body, r.StatusCode, nil
}
