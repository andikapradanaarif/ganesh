package s0package

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type FileResult struct {
	Project         string `json:"project"`
	Path            string `json:"path"`
	State           string `json:"state"`
	Bytes           int    `json:"bytes_hashed_locally"`
	SHA             string `json:"sha256,omitempty"`
	Blob            string `json:"git_blob,omitempty"`
	MatchesSnapshot bool   `json:"matches_reviewed_blob"`
	NativeMatch     *bool  `json:"native_instruction_matches,omitempty"`
}
type ProjectResult struct {
	Root       string `json:"root"`
	HeadBefore string `json:"head_before,omitempty"`
	HeadAfter  string `json:"head_after,omitempty"`
	HeadStable bool   `json:"head_stable"`
}
type Inspection struct {
	PackageSHA          string                   `json:"package_manifest_sha256"`
	Started             time.Time                `json:"started_utc"`
	Finished            time.Time                `json:"finished_utc"`
	Projects            map[string]ProjectResult `json:"projects"`
	Files               []FileResult             `json:"files"`
	Tools               map[string]bool          `json:"executables_on_path"`
	HashedBytes         int                      `json:"source_bytes_hashed_locally"`
	ReturnedSourceBytes int                      `json:"source_body_bytes_in_report"`
	AllSelectedMatch    bool                     `json:"all_selected_files_match_reviewed_snapshots"`
	CompleteTools       bool                     `json:"actual_tool_permissions_inventory_complete"`
	CompleteNotes       bool                     `json:"all_existing_notes_inventory_complete"`
	Dispatch            bool                     `json:"model_dispatch_authorized"`
	Calls               int                      `json:"model_calls"`
}

func head(ctx context.Context, root string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "HEAD")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1"}
	var result boundedHead
	cmd.Stdout = &result
	cmd.Stderr = io.Discard
	if cmd.Run() != nil {
		return ""
	}
	s := strings.TrimSpace(string(result.b))
	if len(s) != 40 && len(s) != 64 {
		return ""
	}
	if _, err := hex.DecodeString(s); err != nil {
		return ""
	}
	return s
}

type boundedHead struct{ b []byte }

func (b *boundedHead) Write(p []byte) (int, error) {
	if len(b.b)+len(p) > 256 {
		return 0, ErrInput
	}
	b.b = append(b.b, p...)
	return len(p), nil
}

// Inspect is the only operation here that reads project paths. The owner runs
// it after inspecting inspection-plan.json. It returns hashes, never file bodies,
// and neither resolves a full tool grant nor changes any execution approval.
func Inspect(ctx context.Context, dir, expected, out string) (Inspection, error) {
	result := Inspection{PackageSHA: expected, Started: time.Now().UTC(), Projects: map[string]ProjectResult{}, Tools: map[string]bool{}, AllSelectedMatch: true}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return result, ErrInput
	}
	defer root.Close()
	b, err := read(root, "manifest.json")
	if err != nil || sum(b) != expected {
		return result, ErrInput
	}
	var m manifest
	if json.Unmarshal(b, &m) != nil || m.Dispatch || m.Issue != 124 || m.Status != "OWNER_PREFLIGHT_PROPOSAL_HOLD_EXECUTION" {
		return result, ErrInput
	}
	for name, hash := range m.Files {
		data, err := read(root, name)
		if err != nil || sum(data) != hash {
			return result, ErrInput
		}
	}
	var plan InspectionPlan
	if m.Files["inspection-plan.json"] == "" || decode(root, "inspection-plan.json", &plan) != nil || len(plan.Roots) != 2 || len(plan.Files) > 128 {
		return result, ErrInput
	}
	for _, project := range []string{"kiosk", "valo-backend"} {
		if !filepath.IsAbs(plan.Roots[project]) {
			return result, ErrInput
		}
	}
	for _, f := range plan.Files {
		if !safePath(f.Path) || (f.Project != "kiosk" && f.Project != "valo-backend") {
			return result, ErrInput
		}
	}
	for _, name := range []string{"git", "rg", "go", "node", "bun"} {
		_, err := exec.LookPath(name)
		result.Tools[name] = err == nil
	}
	for project, path := range plan.Roots {
		pr := ProjectResult{Root: path, HeadBefore: head(ctx, path)}
		projectRoot, err := os.OpenRoot(path)
		for _, file := range plan.Files {
			if file.Project != project {
				continue
			}
			row := FileResult{Project: project, Path: file.Path, State: "unavailable_or_refused"}
			if err == nil && result.HashedBytes < 32<<20 {
				data, e := readLimit(projectRoot, file.Path, min(4<<20, (32<<20)-result.HashedBytes))
				if e == nil && result.HashedBytes+len(data) <= 32<<20 {
					row.State = "hashed"
					row.Bytes = len(data)
					row.SHA = sum(data)
					h := sha1.New()
					_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
					_, _ = h.Write(data)
					row.Blob = hex.EncodeToString(h.Sum(nil))
					for _, expected := range file.GitBlobs {
						row.MatchesSnapshot = row.MatchesSnapshot || row.Blob == expected
					}
					if file.NativeSHA != "" {
						match := file.NativeSHA == row.SHA
						row.NativeMatch = &match
						if len(file.GitBlobs) == 0 {
							row.MatchesSnapshot = match
						}
					}
					result.HashedBytes += len(data)
				}
			}
			if !row.MatchesSnapshot || row.NativeMatch != nil && !*row.NativeMatch {
				result.AllSelectedMatch = false
			}
			result.Files = append(result.Files, row)
		}
		if projectRoot != nil {
			_ = projectRoot.Close()
		}
		pr.HeadAfter = head(ctx, path)
		pr.HeadStable = pr.HeadBefore != "" && pr.HeadBefore == pr.HeadAfter
		if !pr.HeadStable {
			result.AllSelectedMatch = false
		}
		result.Projects[project] = pr
	}
	result.Finished = time.Now().UTC()
	if os.Mkdir(out, 0700) != nil {
		return result, ErrInput
	}
	if writeJSON(filepath.Join(out, "inspection.json"), result) != nil {
		return result, ErrInput
	}
	return result, nil
}
