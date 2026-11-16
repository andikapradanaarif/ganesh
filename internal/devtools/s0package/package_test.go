package s0package

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andikapradanaarif/ganesh/internal/devtools/requestguard"
)

func fixture(t *testing.T, mutate func(*InspectionPlan)) (string, string) {
	t.Helper()
	dir := t.TempDir()
	plan := InspectionPlan{Roots: map[string]string{}}
	for _, project := range []string{"kiosk", "valo-backend"} {
		root := t.TempDir()
		plan.Roots[project] = root
		body := []byte("Synthetic private project text; never return this body.\n")
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), body, 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-q"}, {"add", "AGENTS.md"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "-c", "core.hooksPath=/dev/null", "commit", "-qm", "synthetic"}} {
			cmd := exec.Command("git", args...)
			cmd.Dir = root
			cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0"}
			if cmd.Run() != nil {
				t.Fatal("synthetic git setup failed")
			}
		}
		plan.Files = append(plan.Files, FileSpec{Project: project, Path: "AGENTS.md", NativeSHA: sum(body)})
	}
	if mutate != nil {
		mutate(&plan)
	}
	if err := writeJSON(filepath.Join(dir, "inspection-plan.json"), plan); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "inspection-plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	m := manifest{Issue: 124, Status: "OWNER_PREFLIGHT_PROPOSAL_HOLD_EXECUTION", PacketSHA: BoundSHA, GuardSHA: GuardReportSHA, Files: map[string]string{"inspection-plan.json": sum(b)}}
	data, err := jsonBytes(m)
	if err != nil || exclusive(filepath.Join(dir, "manifest.json"), data) != nil {
		t.Fatal("synthetic package failed")
	}
	return dir, sum(data)
}

func TestInspectReturnsOnlyMetadataAndCannotApprove(t *testing.T) {
	dir, hash := fixture(t, nil)
	out := filepath.Join(t.TempDir(), "report")
	r, err := Inspect(t.Context(), dir, hash, out)
	if err != nil {
		t.Fatal(err)
	}
	if !r.AllSelectedMatch || len(r.Files) != 2 || r.CompleteNotes || r.CompleteTools || r.Dispatch || r.Calls != 0 || r.ReturnedSourceBytes != 0 {
		t.Fatalf("incorrect readiness promotion: %+v", r)
	}
	b, err := os.ReadFile(filepath.Join(out, "inspection.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("Synthetic private project text")) {
		t.Fatal("source body leaked")
	}
	info, _ := os.Stat(filepath.Join(out, "inspection.json"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("report not private")
	}
	if _, err = Inspect(t.Context(), dir, hash, out); err == nil {
		t.Fatal("existing report overwritten")
	}
}

func TestChangedSourceIsReportedWithoutAcceptingNewBaseline(t *testing.T) {
	dir, hash := fixture(t, func(p *InspectionPlan) {
		if err := os.WriteFile(filepath.Join(p.Roots["kiosk"], "AGENTS.md"), []byte("unreviewed changed content"), 0600); err != nil {
			t.Fatal(err)
		}
	})
	r, err := Inspect(t.Context(), dir, hash, filepath.Join(t.TempDir(), "report"))
	if err != nil {
		t.Fatal(err)
	}
	if r.AllSelectedMatch || r.Dispatch {
		t.Fatal("changed source admitted")
	}
	for _, f := range r.Files {
		if f.Project == "kiosk" && (f.NativeMatch == nil || *f.NativeMatch || f.SHA == "") {
			t.Fatal("mismatch not retained")
		}
	}
}

func TestTamperedScopeAndUnsafePathsFailBeforeProjectReads(t *testing.T) {
	t.Run("hash", func(t *testing.T) {
		dir, hash := fixture(t, nil)
		if os.WriteFile(filepath.Join(dir, "inspection-plan.json"), []byte(`{"roots":{"kiosk":"/private"}}`), 0600) != nil {
			t.Fatal("mutation failed")
		}
		if _, err := Inspect(t.Context(), dir, hash, filepath.Join(t.TempDir(), "report")); err == nil {
			t.Fatal("tampered plan accepted")
		}
	})
	for _, path := range []string{"../outside", "/etc/passwd", "a/../../outside"} {
		t.Run(path, func(t *testing.T) {
			dir, hash := fixture(t, func(p *InspectionPlan) { p.Files[0].Path = path })
			if _, err := Inspect(t.Context(), dir, hash, filepath.Join(t.TempDir(), "report")); err == nil {
				t.Fatal("unsafe path accepted")
			}
		})
	}
	t.Run("symlink", func(t *testing.T) {
		dir, hash := fixture(t, func(p *InspectionPlan) {
			path := filepath.Join(p.Roots["kiosk"], "AGENTS.md")
			if os.Remove(path) != nil || os.Symlink("/etc/passwd", path) != nil {
				t.Fatal("symlink fixture failed")
			}
		})
		r, err := Inspect(t.Context(), dir, hash, filepath.Join(t.TempDir(), "report"))
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range r.Files {
			if f.Project == "kiosk" && (f.Bytes != 0 || f.SHA != "" || f.State != "unavailable_or_refused") {
				t.Fatal("symlink target read")
			}
		}
	})
}

func TestProposalArithmeticAndConditionalScope(t *testing.T) {
	b := ProposedBudget()
	if b.Slots != 276 || b.ProviderRequests != 936 || b.OutputTokens != 1388544 || b.ActiveSeconds != 33120 || b.PacedSeconds != 82080 || b.WallSeconds != 172800 {
		t.Fatalf("proposal arithmetic: %+v", b)
	}
	if b.PacedSeconds <= b.ActiveSeconds || b.WallSeconds <= b.PacedSeconds {
		t.Fatal("pacing omitted from wall time")
	}
	if _, err := Prepare(Options{Packet: t.TempDir(), Output: filepath.Join(t.TempDir(), "proposal")}); err == nil {
		t.Fatal("unpinned packet accepted")
	}
}

type events struct{}

func (events) Append(requestguard.Event) error { return nil }

func TestProposedAnswerCapLeavesRoomForAuxiliaryOutput(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		output := int64(40)
		if n == 2 {
			output = 4096
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"completed","usage":{"input_tokens":1,"output_tokens":%d}}`, output)
	}))
	defer server.Close()
	c := ProposedProfiles()["answer"]
	c.Endpoint = server.URL + "/responses"
	g, err := requestguard.New(c, events{})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	tr, err := g.Transport(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: tr}
	for _, cap := range []int{40, 4096} {
		body := fmt.Sprintf(`{"model":"gpt-6-luna","max_output_tokens":%d}`, cap)
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, c.Endpoint, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if s := g.Snapshot(); s.Attempts != 2 || s.KnownOutput != 4136 || s.Stopped != "" {
		t.Fatalf("auxiliary output consumed main reservation: %+v", s)
	}
}
