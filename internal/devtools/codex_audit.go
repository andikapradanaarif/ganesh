package devtools

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var ErrCodexAudit = errors.New("private bounded Codex audit input required")

const (
	codexMaxRolloutBytes int64 = 32 << 20
	codexMaxTotalBytes   int64 = 64 << 20
	codexMaxMetaLine           = 64 << 10
)

type CodexAuditReport struct {
	ManifestRows            int    `json:"manifest_rows"`
	UnchangedFiles          int    `json:"unchanged_files"`
	ChangedFiles            int    `json:"changed_files"`
	MissingFiles            int    `json:"missing_files"`
	CurrentFileBytes        int64  `json:"current_file_bytes"`
	Status                  string `json:"status"`
	ManifestSHA256          string `json:"manifest_sha256"`
	GrantMapSHA256          string `json:"grant_map_sha256"`
	CurrentSnapshotSHA256   string `json:"current_snapshot_sha256"`
	ModelDispatchAuthorized bool   `json:"model_dispatch_authorized"`
}

type codexManifestRow struct{ path, digest string }

func codexManifest(body []byte) ([]codexManifestRow, error) {
	lines := bytes.Split(bytes.TrimSuffix(body, []byte{'\n'}), []byte{'\n'})
	if len(lines) != 2 {
		return nil, ErrCodexAudit
	}
	rows := make([]codexManifestRow, 0, 2)
	seen := make(map[string]bool)
	for _, line := range lines {
		if len(line) < 67 || string(line[64:66]) != "  " {
			return nil, ErrCodexAudit
		}
		digest := string(line[:64])
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size || strings.ToLower(digest) != digest {
			return nil, ErrCodexAudit
		}
		path := string(line[66:])
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || seen[path] || strings.ContainsAny(path, "\r\x00") {
			return nil, ErrCodexAudit
		}
		seen[path] = true
		rows = append(rows, codexManifestRow{path: path, digest: digest})
	}
	return rows, nil
}

func codexRelativePath(root, path string, opened *os.Root) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrCodexAudit
	}
	base := filepath.Base(rel)
	if !strings.HasPrefix(base, "rollout-") || !strings.HasSuffix(base, ".jsonl") {
		return "", ErrCodexAudit
	}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		if part == "" || part == "." || part == ".." {
			return "", ErrCodexAudit
		}
	}
	for i := 1; i < len(parts); i++ {
		component := filepath.Join(parts[:i]...)
		info, err := opened.Lstat(component)
		if err != nil || !info.IsDir() {
			return "", ErrCodexAudit
		}
	}
	return rel, nil
}

func codexMetaCWD(line []byte) (string, error) {
	if len(line) == 0 || len(line) > codexMaxMetaLine || uniqueJSON(line) != nil {
		return "", ErrCodexAudit
	}
	var record map[string]json.RawMessage
	if json.Unmarshal(line, &record) != nil {
		return "", ErrCodexAudit
	}
	for key := range record {
		if strings.EqualFold(key, "type") && key != "type" || strings.EqualFold(key, "payload") && key != "payload" {
			return "", ErrCodexAudit
		}
	}
	var kind string
	if json.Unmarshal(record["type"], &kind) != nil || kind != "session_meta" {
		return "", ErrCodexAudit
	}
	var payload map[string]json.RawMessage
	if json.Unmarshal(record["payload"], &payload) != nil {
		return "", ErrCodexAudit
	}
	for key := range payload {
		if strings.EqualFold(key, "cwd") && key != "cwd" {
			return "", ErrCodexAudit
		}
	}
	var cwd string
	if json.Unmarshal(payload["cwd"], &cwd) != nil || !filepath.IsAbs(cwd) || filepath.Clean(cwd) != cwd {
		return "", ErrCodexAudit
	}
	return cwd, nil
}

func codexHashFile(root *os.Root, name, expectedCWD string, remaining int64) (state string, size int64, digest string, err error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", 0, "", nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > codexMaxRolloutBytes || info.Size() > remaining {
		return "", 0, "", ErrCodexAudit
	}
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", 0, "", ErrCodexAudit
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return "", 0, "", ErrCodexAudit
	}
	hash := sha256.New()
	reader := bufio.NewReaderSize(io.TeeReader(io.LimitReader(f, codexMaxRolloutBytes+1), hash), codexMaxMetaLine)
	first, err := reader.ReadSlice('\n')
	if err != nil || len(first) > codexMaxMetaLine {
		return "", 0, "", ErrCodexAudit
	}
	cwd, err := codexMetaCWD(bytes.TrimSuffix(first, []byte{'\n'}))
	if err != nil || cwd != expectedCWD {
		return "", 0, "", ErrCodexAudit
	}
	rest, err := io.Copy(io.Discard, reader)
	n := int64(len(first)) + rest
	if err != nil || n != info.Size() || n > codexMaxRolloutBytes {
		return "", 0, "", ErrCodexAudit
	}
	if err := f.Close(); err != nil {
		return "", 0, "", ErrCodexAudit
	}
	after, err := root.Lstat(name)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(info, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", 0, "", ErrCodexAudit
	}
	return "present_hashed", n, hex.EncodeToString(hash.Sum(nil)), nil
}

// AuditCodex rechecks only the two previously selected Kiosk rollouts.
// It does not assess transcript provenance, scrub safety or model eligibility.
func AuditCodex(manifestDirectory, sessionsRoot, grantMapDirectory string) (report CodexAuditReport, auditErr error) {
	defer func() {
		if auditErr != nil {
			report = CodexAuditReport{}
		}
	}()
	if !filepath.IsAbs(sessionsRoot) || filepath.Clean(sessionsRoot) != sessionsRoot {
		return report, ErrCodexAudit
	}
	manifestInfo, err := os.Lstat(manifestDirectory)
	if err != nil || !manifestInfo.IsDir() || manifestInfo.Mode().Perm() != 0o700 {
		return report, ErrCodexAudit
	}
	manifestRoot, err := os.OpenRoot(manifestDirectory)
	if err != nil {
		return report, ErrCodexAudit
	}
	defer manifestRoot.Close()
	opened, err := manifestRoot.Stat(".")
	if err != nil || !os.SameFile(manifestInfo, opened) {
		return report, ErrCodexAudit
	}
	body, err := privateFile(manifestRoot, "codex-kiosk-manifest.txt", 1<<20)
	if err != nil {
		return report, ErrCodexAudit
	}
	rows, err := codexManifest(body)
	if err != nil {
		return report, ErrCodexAudit
	}
	report.ManifestSHA256 = devinDigest(body)

	grantInfo, err := os.Lstat(grantMapDirectory)
	if err != nil || !grantInfo.IsDir() || grantInfo.Mode().Perm() != 0o700 {
		return report, ErrCodexAudit
	}
	grantRoot, err := os.OpenRoot(grantMapDirectory)
	if err != nil {
		return report, ErrCodexAudit
	}
	defer grantRoot.Close()
	opened, err = grantRoot.Stat(".")
	if err != nil || !os.SameFile(grantInfo, opened) {
		return report, ErrCodexAudit
	}
	grantBody, err := privateFile(grantRoot, "project-roots.json", 1<<20)
	if err != nil {
		return report, ErrCodexAudit
	}
	var projectRoots map[string]string
	if decodeStudy(grantBody, &projectRoots) != nil || len(projectRoots) != 4 {
		return report, ErrCodexAudit
	}
	for _, project := range []string{"aurum", "borneo", "valo-backend", "kiosk"} {
		root := projectRoots[project]
		if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return report, ErrCodexAudit
		}
	}
	report.GrantMapSHA256 = devinDigest(grantBody)

	rootInfo, err := os.Lstat(sessionsRoot)
	if err != nil || !rootInfo.IsDir() {
		return report, ErrCodexAudit
	}
	sessions, err := os.OpenRoot(sessionsRoot)
	if err != nil {
		return report, ErrCodexAudit
	}
	defer sessions.Close()
	opened, err = sessions.Stat(".")
	if err != nil || !os.SameFile(rootInfo, opened) {
		return report, ErrCodexAudit
	}
	snapshot := sha256.New()
	for _, row := range rows {
		rel, err := codexRelativePath(sessionsRoot, row.path, sessions)
		if err != nil {
			return report, ErrCodexAudit
		}
		state, size, digest, err := codexHashFile(sessions, rel, projectRoots["kiosk"], codexMaxTotalBytes-report.CurrentFileBytes)
		if err != nil {
			return report, ErrCodexAudit
		}
		report.CurrentFileBytes += size
		switch {
		case state == "missing":
			report.MissingFiles++
		case digest == row.digest:
			report.UnchangedFiles++
		default:
			report.ChangedFiles++
		}
		for _, value := range []string{rel, state, digest} {
			_, _ = snapshot.Write([]byte{byte(len(value) >> 8), byte(len(value))})
			_, _ = snapshot.Write([]byte(value))
		}
	}
	report.ManifestRows = len(rows)
	report.CurrentSnapshotSHA256 = hex.EncodeToString(snapshot.Sum(nil))
	report.Status = "unchanged"
	if report.ChangedFiles+report.MissingFiles != 0 {
		report.Status = "drift_detected"
	}
	report.ModelDispatchAuthorized = false
	return report, nil
}
