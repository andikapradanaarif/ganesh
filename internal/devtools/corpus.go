package devtools

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ErrCorpus = errors.New("private bounded corpus input required")

// The local raw snapshots use snake-case keys; keep this separate from the
// report so a missing field cannot be silently interpreted as an empty value.
type corpusRawJSON struct {
	SessionID    string `json:"session_id"`
	ProjectKey   string `json:"project_key"`
	Title        string `json:"title"`
	Checkpoint   string `json:"checkpoint"`
	Files        string `json:"files"`
	ParamVersion string `json:"param_version"`
	EndedAt      int64  `json:"ended_at"`
}

type corpusField struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type corpusManifest struct {
	SessionID    string      `json:"session_id"`
	Project      string      `json:"project"`
	ProjectKey   string      `json:"project_key"`
	EndedAt      int64       `json:"ended_at"`
	ParamVersion string      `json:"param_version"`
	Title        corpusField `json:"title"`
	Checkpoint   corpusField `json:"checkpoint"`
	Files        corpusField `json:"files"`
	Origin       string      `json:"origin"`
	ScrubStatus  string      `json:"scrub_status"`
}

type corpusCandidate struct {
	SessionID         string            `json:"session_id"`
	Project           string            `json:"project"`
	EndedAt           int64             `json:"ended_at"`
	ParamVersion      string            `json:"param_version"`
	SourceFieldSHA256 map[string]string `json:"source_field_sha256"`
	Title             string            `json:"title"`
	Checkpoint        string            `json:"checkpoint"`
	Files             string            `json:"files"`
	Redactions        map[string]int    `json:"redactions"`
	Origin            string            `json:"origin"`
	Status            string            `json:"status"`
	ModelDispatch     bool              `json:"model_dispatch_authorized"`
}

type CorpusAudit struct {
	RawRows                 int            `json:"raw_rows"`
	ManifestRows            int            `json:"manifest_rows"`
	CandidateRows           int            `json:"candidate_rows"`
	Projects                map[string]int `json:"projects"`
	Redactions              map[string]int `json:"redactions"`
	RawTextBytes            int            `json:"raw_text_bytes"`
	CandidateTextBytes      int            `json:"candidate_text_bytes"`
	SourceLinksValid        bool           `json:"source_links_valid"`
	DeclaredPatternsClear   bool           `json:"declared_patterns_clear"`
	ModelDispatchAuthorized bool           `json:"model_dispatch_authorized"`
}

var corpusRules = []struct {
	name string
	re   *regexp.Regexp
}{
	{"pem_block", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]+-----.*?-----END [A-Z ]+-----`)},
	{"fenced_code", regexp.MustCompile("(?s)```.*?```")},
	{"url", regexp.MustCompile(`https?://[^\s<>()\[\]{}"']+`)},
	{"email", regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)},
	{"absolute_path", regexp.MustCompile(`/(?:Users|home|private|var)/[^\s<>"']+`)},
	{"credential_assignment", regexp.MustCompile(`(?i)\b(?:password|passwd|api[_-]?key|access[_-]?token|auth[_-]?token|secret)\b\s*[:=]\s*[^\s,;}]+`)},
	{"known_token", regexp.MustCompile(`(?:sk-[A-Za-z0-9_-]{12,}|gh[pousr]_[A-Za-z0-9_]{12,}|github_pat_[A-Za-z0-9_]{12,}|AKIA[0-9A-Z]{16})`)},
	{"long_token", regexp.MustCompile(`[A-Za-z0-9_+/=-]{32,}`)},
}

func corpusHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func corpusScrub(s string, counts map[string]int) string {
	for _, rule := range corpusRules {
		matches := rule.re.FindAllStringIndex(s, -1)
		counts[rule.name] += len(matches)
		s = rule.re.ReplaceAllString(s, "[REDACTED_"+strings.ToUpper(rule.name)+"]")
	}
	return s
}

func corpusLines(body []byte, fn func([]byte) error) error {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count := 0
	for scanner.Scan() {
		count++
		if count > 1000 || fn(scanner.Bytes()) != nil {
			return ErrCorpus
		}
	}
	if scanner.Err() != nil || count == 0 {
		return ErrCorpus
	}
	return nil
}

func corpusRead(root *os.Root, name string) ([]byte, error) {
	if name == "" || filepath.Base(name) != name || name == "." {
		return nil, ErrCorpus
	}
	// The 2 MiB per-file cap bounds this pilot reader independently of row caps.
	body, err := privateFile(root, name, 2<<20)
	if err != nil {
		return nil, ErrCorpus
	}
	return body, nil
}

func corpusRequired(body []byte, keys ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(body, &fields) != nil {
		return false
	}
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return false
		}
	}
	return true
}

// AuditCorpus verifies the existing local-only reduction against private raw
// snapshots. It does not establish safe disclosure or semantic correctness.
func AuditCorpus(directory string, rawNames []string) (report CorpusAudit, auditErr error) {
	// A rejected corpus must not expose counts from the rows scanned before failure.
	defer func() {
		if auditErr != nil {
			report = CorpusAudit{}
		}
	}()
	if len(rawNames) == 0 || len(rawNames) > 10 {
		return report, ErrCorpus
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return report, ErrCorpus
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return report, ErrCorpus
	}
	defer root.Close()
	opened, err := root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return report, ErrCorpus
	}

	raw := make(map[string]corpusRawJSON)
	seenNames := make(map[string]bool)
	for _, name := range rawNames {
		if seenNames[name] {
			return report, ErrCorpus
		}
		seenNames[name] = true
		body, err := corpusRead(root, name)
		if err != nil {
			return report, err
		}
		var rows []corpusRawJSON
		if decodeStudy(body, &rows) != nil || len(rows) == 0 || len(rows) > 1000 {
			return report, ErrCorpus
		}
		var rawFields []json.RawMessage
		if json.Unmarshal(body, &rawFields) != nil || len(rawFields) != len(rows) {
			return report, ErrCorpus
		}
		for i, row := range rows {
			if !corpusRequired(rawFields[i], "session_id", "project_key", "title", "checkpoint", "files", "param_version", "ended_at") || row.SessionID == "" || row.ProjectKey == "" || raw[row.SessionID].SessionID != "" || len(row.Title)+len(row.Checkpoint)+len(row.Files) > 64<<10 {
				return report, ErrCorpus
			}
			raw[row.SessionID] = row
			report.RawTextBytes += len(row.Title) + len(row.Checkpoint) + len(row.Files)
		}
	}
	report.RawRows = len(raw)
	manifest := make(map[string]corpusManifest)
	body, err := corpusRead(root, "crush-digest-manifest.jsonl")
	if err != nil {
		return report, err
	}
	if corpusLines(body, func(line []byte) error {
		var m corpusManifest
		if decodeStudy(line, &m) != nil || !corpusRequired(line, "session_id", "project", "project_key", "ended_at", "param_version", "title", "checkpoint", "files", "origin", "scrub_status") || m.SessionID == "" || manifest[m.SessionID].SessionID != "" || m.Project == "" || m.Origin != "unknown_unreconciled" || m.ScrubStatus != "raw_unreviewed" {
			return ErrCorpus
		}
		r, ok := raw[m.SessionID]
		if !ok || r.ProjectKey != m.ProjectKey || r.EndedAt != m.EndedAt || r.ParamVersion != m.ParamVersion {
			return ErrCorpus
		}
		for _, pair := range []struct {
			value string
			field corpusField
		}{{r.Title, m.Title}, {r.Checkpoint, m.Checkpoint}, {r.Files, m.Files}} {
			if len(pair.value) != pair.field.Bytes || corpusHash(pair.value) != pair.field.SHA256 {
				return ErrCorpus
			}
		}
		manifest[m.SessionID] = m
		return nil
	}) != nil || len(manifest) != len(raw) {
		return report, ErrCorpus
	}
	report.ManifestRows = len(manifest)

	body, err = corpusRead(root, "crush-digest-candidates.jsonl")
	if err != nil {
		return report, err
	}
	seen := make(map[string]bool)
	report.Projects = make(map[string]int)
	report.Redactions = make(map[string]int)
	if corpusLines(body, func(line []byte) error {
		var c corpusCandidate
		if decodeStudy(line, &c) != nil || !corpusRequired(line, "session_id", "project", "ended_at", "param_version", "source_field_sha256", "title", "checkpoint", "files", "redactions", "origin", "status", "model_dispatch_authorized") || seen[c.SessionID] || c.ModelDispatch || c.Status != "candidate_unverified_local_only" || c.Origin != "unknown_unreconciled" {
			return ErrCorpus
		}
		m, ok := manifest[c.SessionID]
		if !ok || c.Project != m.Project || c.EndedAt != m.EndedAt || c.ParamVersion != m.ParamVersion || len(c.SourceFieldSHA256) != 3 || c.SourceFieldSHA256["title"] != m.Title.SHA256 || c.SourceFieldSHA256["checkpoint"] != m.Checkpoint.SHA256 || c.SourceFieldSHA256["files"] != m.Files.SHA256 {
			return ErrCorpus
		}
		r := raw[c.SessionID]
		counts := make(map[string]int)
		if c.Title != corpusScrub(r.Title, counts) || c.Checkpoint != corpusScrub(r.Checkpoint, counts) || c.Files != corpusScrub(r.Files, counts) {
			return ErrCorpus
		}
		for _, rule := range corpusRules {
			if c.Redactions[rule.name] != counts[rule.name] || rule.re.MatchString(c.Title) || rule.re.MatchString(c.Checkpoint) || rule.re.MatchString(c.Files) {
				return ErrCorpus
			}
			report.Redactions[rule.name] += counts[rule.name]
		}
		for name := range c.Redactions {
			found := false
			for _, rule := range corpusRules {
				if name == rule.name {
					found = true
					break
				}
			}
			if !found {
				return ErrCorpus
			}
		}
		seen[c.SessionID] = true
		report.Projects[c.Project]++
		report.CandidateTextBytes += len(c.Title) + len(c.Checkpoint) + len(c.Files)
		return nil
	}) != nil || len(seen) != len(manifest) {
		return report, ErrCorpus
	}
	report.CandidateRows = len(seen)
	report.SourceLinksValid = true
	report.DeclaredPatternsClear = true
	// Every candidate is explicitly local-only; this aggregate cannot authorize dispatch.
	report.ModelDispatchAuthorized = false
	return report, nil
}
