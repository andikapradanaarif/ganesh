package devtools

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
)

var ErrDevinCandidates = errors.New("private pinned Devin candidate inputs required")

const devinExcerptPolicy = "bounded-patterns-v1"

type devinExcerpt struct {
	SessionID     string         `json:"session_id"`
	Project       string         `json:"project"`
	SourceSHA256  string         `json:"source_file_sha256,omitempty"`
	Status        string         `json:"status"`
	Exclusion     string         `json:"exclusion,omitempty"`
	StepIndex     *int           `json:"step_index,omitempty"`
	MessageSHA256 string         `json:"source_message_sha256,omitempty"`
	Text          string         `json:"text,omitempty"`
	TextSHA256    string         `json:"text_sha256,omitempty"`
	Redactions    map[string]int `json:"redactions,omitempty"`
	Origin        string         `json:"origin"`
	Closure       string         `json:"session_closure"`
	ModelDispatch bool           `json:"model_dispatch_authorized"`
}

type devinExcerptBatch struct {
	Schema         string         `json:"schema"`
	Policy         string         `json:"reduction_policy"`
	ManifestSHA256 string         `json:"source_manifest_sha256"`
	IndexSHA256    string         `json:"source_index_sha256"`
	GrantMapSHA256 string         `json:"grant_map_sha256"`
	Records        []devinExcerpt `json:"records"`
}

type DevinCandidateReport struct {
	Rows                 int            `json:"rows"`
	Candidates           int            `json:"candidates"`
	Exclusions           map[string]int `json:"exclusions"`
	SourceFileBytes      int64          `json:"source_file_bytes"`
	SelectedMessageBytes int            `json:"selected_message_bytes"`
	CandidateTextBytes   int            `json:"candidate_text_bytes"`
	Redactions           map[string]int `json:"redactions"`
	ArtifactSHA256       string         `json:"artifact_sha256"`
	ModelDispatch        bool           `json:"model_dispatch_authorized"`
}

// exactObject permits unused source fields but rejects aliases of any field
// whose meaning the excerpt producer consumes. No unused values are exported.
func exactObject(body []byte, names ...string) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil || fields == nil {
		return nil, false
	}
	for key := range fields {
		for _, name := range names {
			if strings.EqualFold(key, name) && key != name {
				return nil, false
			}
		}
	}
	return fields, true
}

func selectDevinExcerpt(body []byte, sessionID string) (message string, stepIndex int, reason string) {
	if uniqueJSON(body) != nil {
		return "", 0, "malformed_transcript"
	}
	fields, ok := exactObject(body, "schema_version", "session_id", "steps")
	if !ok {
		return "", 0, "malformed_transcript"
	}
	var schema, id string
	if json.Unmarshal(fields["schema_version"], &schema) != nil || schema != "ATIF-v1.7" {
		return "", 0, "unsupported_schema"
	}
	if json.Unmarshal(fields["session_id"], &id) != nil || id != sessionID {
		return "", 0, "session_identity_mismatch"
	}
	var steps []json.RawMessage
	if json.Unmarshal(fields["steps"], &steps) != nil || len(steps) == 0 || len(steps) > 20000 {
		return "", 0, "unsupported_steps"
	}
	found := false
	for i, step := range steps {
		values, ok := exactObject(step, "source", "message", "tool_calls")
		if !ok {
			return "", 0, "malformed_step"
		}
		var source string
		if json.Unmarshal(values["source"], &source) != nil {
			return "", 0, "malformed_step"
		}
		if source == "system" || source == "user" {
			continue
		}
		if source != "agent" {
			return "", 0, "unsupported_step_source"
		}
		var calls []json.RawMessage
		if raw, exists := values["tool_calls"]; exists && json.Unmarshal(raw, &calls) != nil {
			return "", 0, "malformed_tool_calls"
		}
		if len(calls) != 0 {
			continue
		}
		raw, exists := values["message"]
		if !exists || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			return "", 0, "malformed_agent_message"
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		message, stepIndex, found = text, i, true
	}
	if !found {
		return "", 0, "no_agent_message"
	}
	if len(message) > 16<<10 {
		return "", 0, "message_over_16_kib"
	}
	return message, stepIndex, ""
}

func devinSourceBytes(root *os.Root, row devinManifestRow) ([]byte, error) {
	name := row.SessionID + ".json"
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || before.Size() != row.Bytes || row.Bytes < 0 || row.Bytes > 32<<20 {
		return nil, ErrDevinCandidates
	}
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrDevinCandidates
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, ErrDevinCandidates
	}
	body, err := io.ReadAll(io.LimitReader(f, row.Bytes+1))
	if err != nil || int64(len(body)) != row.Bytes || devinDigest(body) != row.SHA256 {
		return nil, ErrDevinCandidates
	}
	if f.Close() != nil {
		return nil, ErrDevinCandidates
	}
	after, err := root.Lstat(name)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrDevinCandidates
	}
	return body, nil
}

func privateCandidateOutput(directory string, body []byte) error {
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return ErrDevinCandidates
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return ErrDevinCandidates
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm() != 0o700 {
		return ErrDevinCandidates
	}
	f, err := root.OpenFile("devin-excerpts.json", os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return ErrDevinCandidates
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return ErrDevinCandidates
	}
	if _, err := io.Copy(f, bytes.NewReader(body)); err != nil {
		return ErrDevinCandidates
	}
	if f.Sync() != nil || f.Close() != nil {
		return ErrDevinCandidates
	}
	return nil
}

// PrepareDevinCandidates exports bounded scrubbed excerpts into private storage.
// It never renders source prose to stdout or establishes semantic eligibility.
func PrepareDevinCandidates(directory, transcriptRoot, grantMapDirectory, expectedManifest, outputDirectory string) (report DevinCandidateReport, resultErr error) {
	defer func() {
		if resultErr != nil {
			report = DevinCandidateReport{}
		}
	}()
	audit, err := AuditDevin(directory, transcriptRoot, grantMapDirectory)
	if err != nil || audit.Status != "unchanged" || audit.ManifestSHA256 != expectedManifest {
		return report, ErrDevinCandidates
	}
	evidence, err := os.OpenRoot(directory)
	if err != nil {
		return report, ErrDevinCandidates
	}
	defer evidence.Close()
	manifest, err := privateFile(evidence, "devin-manifest.jsonl", 4<<20)
	if err != nil || devinDigest(manifest) != expectedManifest {
		return report, ErrDevinCandidates
	}
	sourceInfo, err := os.Lstat(transcriptRoot)
	if err != nil || !sourceInfo.IsDir() {
		return report, ErrDevinCandidates
	}
	sources, err := os.OpenRoot(transcriptRoot)
	if err != nil {
		return report, ErrDevinCandidates
	}
	defer sources.Close()
	opened, err := sources.Stat(".")
	if err != nil || !os.SameFile(sourceInfo, opened) {
		return report, ErrDevinCandidates
	}
	batch := devinExcerptBatch{Schema: "devin-excerpts-v1", Policy: devinExcerptPolicy, ManifestSHA256: expectedManifest, IndexSHA256: audit.IndexSHA256, GrantMapSHA256: audit.GrantMapSHA256}
	report.Exclusions = make(map[string]int)
	report.Redactions = make(map[string]int)
	if corpusLines(manifest, func(line []byte) error {
		var row devinManifestRow
		if decodeStudy(line, &row) != nil {
			return ErrDevinCandidates
		}
		item := devinExcerpt{SessionID: row.SessionID, Project: row.Project, SourceSHA256: row.SHA256, Status: "excluded", Origin: "unknown_unreconciled", Closure: "unknown", ModelDispatch: false}
		if row.State == "missing" {
			item.Exclusion = "source_missing"
		} else {
			body, err := devinSourceBytes(sources, row)
			if err != nil {
				return err
			}
			report.SourceFileBytes += int64(len(body))
			message, index, reason := selectDevinExcerpt(body, row.SessionID)
			item.Exclusion = reason
			if reason == "" {
				counts := make(map[string]int)
				text := corpusScrub(message, counts)
				if len(text) > 64<<10 {
					item.Exclusion = "reduced_message_over_64_kib"
				} else {
					for _, rule := range corpusRules {
						if rule.re.MatchString(text) {
							item.Exclusion = "residual_declared_pattern"
							break
						}
					}
				}
				if item.Exclusion == "" {
					item.Status, item.StepIndex, item.Text, item.TextSHA256, item.MessageSHA256, item.Redactions = "candidate_unverified_local_only", &index, text, corpusHash(text), corpusHash(message), counts
					report.Candidates++
					report.SelectedMessageBytes += len(message)
					report.CandidateTextBytes += len(text)
					for key, count := range counts {
						report.Redactions[key] += count
					}
				}
			}
		}
		if item.Exclusion != "" {
			report.Exclusions[item.Exclusion]++
		}
		batch.Records = append(batch.Records, item)
		report.Rows++
		return nil
	}) != nil || report.Rows != audit.ManifestRows || report.SourceFileBytes != audit.CurrentFileBytes {
		return report, ErrDevinCandidates
	}
	body, err := json.MarshalIndent(batch, "", "  ")
	if err != nil || len(body)+1 > 2<<20 {
		return report, ErrDevinCandidates
	}
	body = append(body, '\n')
	if privateCandidateOutput(outputDirectory, body) != nil {
		return report, ErrDevinCandidates
	}
	report.ArtifactSHA256 = devinDigest(body)
	report.ModelDispatch = false
	return report, nil
}
