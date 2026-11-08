package devtools

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var ErrDevinAudit = errors.New("private bounded Devin audit input required")

var devinSessionID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const devinMaxTranscriptBytes int64 = 128 << 20

type devinIndexRow struct {
	ID               string `json:"id"`
	WorkingDirectory string `json:"working_directory"`
}

type devinManifestRow struct {
	SessionID        string `json:"session_id"`
	Project          string `json:"project"`
	WorkingDirectory string `json:"working_directory"`
	TranscriptPath   string `json:"transcript_path"`
	State            string `json:"state"`
	Bytes            int64  `json:"bytes,omitempty"`
	SHA256           string `json:"sha256,omitempty"`
	Origin           string `json:"origin"`
}

type DevinAuditReport struct {
	IndexRows               int            `json:"index_rows"`
	ManifestRows            int            `json:"manifest_rows"`
	OriginalStates          map[string]int `json:"original_states"`
	CurrentStates           map[string]int `json:"current_states"`
	Drift                   map[string]int `json:"drift"`
	OriginalFileBytes       int64          `json:"original_file_bytes"`
	CurrentFileBytes        int64          `json:"current_file_bytes"`
	IndexSHA256             string         `json:"index_sha256"`
	ManifestSHA256          string         `json:"manifest_sha256"`
	GrantMapSHA256          string         `json:"grant_map_sha256"`
	CurrentSnapshotSHA256   string         `json:"current_snapshot_sha256"`
	Status                  string         `json:"status"`
	ModelDispatchAuthorized bool           `json:"model_dispatch_authorized"`
}

func devinDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func devinHashFile(root *os.Root, name string, remaining int64) (state string, bytesRead int64, digest string, err error) {
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return "missing", 0, "", nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32<<20 || info.Size() > remaining {
		return "", 0, "", ErrDevinAudit
	}
	f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", 0, "", ErrDevinAudit
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return "", 0, "", ErrDevinAudit
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(f, 32<<20+1))
	if err != nil || n != info.Size() || n > 32<<20 {
		return "", 0, "", ErrDevinAudit
	}
	if err := f.Close(); err != nil {
		return "", 0, "", ErrDevinAudit
	}
	after, err := root.Lstat(name)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(info, after) || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return "", 0, "", ErrDevinAudit
	}
	return "present_hashed", n, hex.EncodeToString(hash.Sum(nil)), nil
}

// AuditDevin compares a saved index/manifest with a current transcript store.
// It is a point-in-time source-fidelity check, not corpus or origin acceptance.
func AuditDevin(directory, transcriptRoot, grantMapDirectory string) (report DevinAuditReport, auditErr error) {
	defer func() {
		if auditErr != nil {
			report = DevinAuditReport{}
		}
	}()
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 || !filepath.IsAbs(transcriptRoot) || filepath.Clean(transcriptRoot) != transcriptRoot {
		return report, ErrDevinAudit
	}
	evidence, err := os.OpenRoot(directory)
	if err != nil {
		return report, ErrDevinAudit
	}
	defer evidence.Close()
	opened, err := evidence.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		return report, ErrDevinAudit
	}
	mapInfo, err := os.Lstat(grantMapDirectory)
	if err != nil || !mapInfo.IsDir() || mapInfo.Mode().Perm() != 0o700 {
		return report, ErrDevinAudit
	}
	grantMapRoot, err := os.OpenRoot(grantMapDirectory)
	if err != nil {
		return report, ErrDevinAudit
	}
	defer grantMapRoot.Close()
	opened, err = grantMapRoot.Stat(".")
	if err != nil || !os.SameFile(mapInfo, opened) {
		return report, ErrDevinAudit
	}
	grantMapBody, err := privateFile(grantMapRoot, "project-roots.json", 1<<20)
	if err != nil {
		return report, ErrDevinAudit
	}
	var projectRoots map[string]string
	if decodeStudy(grantMapBody, &projectRoots) != nil || len(projectRoots) != 4 {
		return report, ErrDevinAudit
	}
	for _, project := range []string{"aurum", "borneo", "valo-backend", "kiosk"} {
		root := projectRoots[project]
		if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return report, ErrDevinAudit
		}
	}
	report.GrantMapSHA256 = devinDigest(grantMapBody)
	rootInfo, err := os.Lstat(transcriptRoot)
	if err != nil || !rootInfo.IsDir() {
		return report, ErrDevinAudit
	}
	transcripts, err := os.OpenRoot(transcriptRoot)
	if err != nil {
		return report, ErrDevinAudit
	}
	defer transcripts.Close()
	opened, err = transcripts.Stat(".")
	if err != nil || !os.SameFile(rootInfo, opened) {
		return report, ErrDevinAudit
	}

	indexBody, err := privateFile(evidence, "devin-sessions.json", 4<<20)
	if err != nil {
		return report, ErrDevinAudit
	}
	var indexRows []devinIndexRow
	if decodeStudy(indexBody, &indexRows) != nil || len(indexRows) == 0 || len(indexRows) > 1000 {
		return report, ErrDevinAudit
	}
	index := make(map[string]devinIndexRow)
	for _, row := range indexRows {
		if !devinSessionID.MatchString(row.ID) || row.WorkingDirectory == "" || index[row.ID].ID != "" {
			return report, ErrDevinAudit
		}
		index[row.ID] = row
	}
	report.IndexRows = len(index)
	report.IndexSHA256 = devinDigest(indexBody)

	manifestBody, err := privateFile(evidence, "devin-manifest.jsonl", 4<<20)
	if err != nil {
		return report, ErrDevinAudit
	}
	report.ManifestSHA256 = devinDigest(manifestBody)
	report.OriginalStates = make(map[string]int)
	report.CurrentStates = make(map[string]int)
	report.Drift = make(map[string]int)
	seen := make(map[string]bool)
	snapshot := sha256.New()
	if corpusLines(manifestBody, func(line []byte) error {
		var row devinManifestRow
		if decodeStudy(line, &row) != nil || !corpusRequired(line, "session_id", "project", "working_directory", "transcript_path", "state", "origin") || !devinSessionID.MatchString(row.SessionID) || seen[row.SessionID] || row.Origin != "unknown_unreconciled" {
			return ErrDevinAudit
		}
		if row.Project != "aurum" && row.Project != "borneo" && row.Project != "valo-backend" && row.Project != "kiosk" {
			return ErrDevinAudit
		}
		indexed, ok := index[row.SessionID]
		if !ok || indexed.WorkingDirectory != row.WorkingDirectory {
			return ErrDevinAudit
		}
		permittedRoot := projectRoots[row.Project]
		if row.WorkingDirectory != permittedRoot && !strings.HasPrefix(row.WorkingDirectory, permittedRoot+string(filepath.Separator)) {
			return ErrDevinAudit
		}
		name := row.SessionID + ".json"
		if row.TranscriptPath != filepath.Join(transcriptRoot, name) {
			return ErrDevinAudit
		}
		if row.State != "present_hashed" && row.State != "missing" {
			return ErrDevinAudit
		}
		if row.State == "present_hashed" {
			decoded, err := hex.DecodeString(row.SHA256)
			if row.Bytes < 0 || row.Bytes > 32<<20 || len(decoded) != sha256.Size || err != nil || strings.ToLower(row.SHA256) != row.SHA256 {
				return ErrDevinAudit
			}
			if row.Bytes > devinMaxTranscriptBytes-report.OriginalFileBytes {
				return ErrDevinAudit
			}
			report.OriginalFileBytes += row.Bytes
		}
		if row.State == "missing" && (row.Bytes != 0 || row.SHA256 != "") {
			return ErrDevinAudit
		}
		state, size, digest, err := devinHashFile(transcripts, name, devinMaxTranscriptBytes-report.CurrentFileBytes)
		if err != nil {
			return err
		}
		report.CurrentFileBytes += size
		key := row.Project + "/" + row.State
		report.OriginalStates[key]++
		report.CurrentStates[row.Project+"/"+state]++
		if row.State == "missing" && state == "present_hashed" {
			report.Drift["newly_present"]++
		}
		if row.State == "present_hashed" && state == "missing" {
			report.Drift["now_missing"]++
		}
		if row.State == "present_hashed" && state == "present_hashed" && (row.Bytes != size || row.SHA256 != digest) {
			report.Drift["content_changed"]++
		}
		// Length-prefixed identity and state bind each current observation without
		// exposing private IDs or paths in the aggregate report.
		for _, value := range []string{row.SessionID, state, digest} {
			_, _ = snapshot.Write([]byte{byte(len(value) >> 8), byte(len(value))})
			_, _ = snapshot.Write([]byte(value))
		}
		seen[row.SessionID] = true
		return nil
	}) != nil || len(seen) != len(index) {
		return report, ErrDevinAudit
	}
	report.ManifestRows = len(seen)
	report.CurrentSnapshotSHA256 = hex.EncodeToString(snapshot.Sum(nil))
	report.Status = "unchanged"
	for _, count := range report.Drift {
		if count != 0 {
			report.Status = "drift_detected"
		}
	}
	report.ModelDispatchAuthorized = false
	return report, nil
}
