package s0

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Observation struct {
	Name                    string  `json:"name"`
	Executable              *string `json:"executable"`
	Version                 *string `json:"version"`
	Status                  string  `json:"status"`
	CaptureProfileSupported bool    `json:"capture_profile_supported"`
}

type probeSpec struct {
	name    string
	args    []string
	pattern *regexp.Regexp
}

var probes = []probeSpec{
	{"go", []string{"version"}, regexp.MustCompile(`^go version (go[0-9][0-9A-Za-z.+-]*) [a-z0-9_]+/[a-z0-9_]+$`)},
	{"codex", []string{"--version"}, regexp.MustCompile(`^codex-cli ([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)$`)},
	{"claude", []string{"--version"}, regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+) \(Claude Code\)$`)},
	{"devin", []string{"--version"}, regexp.MustCompile(`^devin ([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?(?: \([0-9a-f]{7,40}\))?)$`)},
	{"crush", []string{"--version"}, regexp.MustCompile(`^crush version (v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)$`)},
}

type probeRunner func(context.Context, string, []string) ([]byte, error)

// Discover uses only fixed version commands. Neither raw command output nor
// stderr is retained. Finding a binary never selects or supports its capture.
func Discover(ctx context.Context) []Observation { return discover(ctx, exec.LookPath, runProbe) }

func discover(ctx context.Context, lookup func(string) (string, error), run probeRunner) []Observation {
	observations := make([]Observation, 0, len(probes))
	for _, spec := range probes {
		o := Observation{Name: spec.name, Status: "not_on_path"}
		path, err := lookup(spec.name)
		if err == nil {
			o.Executable = &path
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			body, err := run(probeCtx, path, spec.args)
			if probeCtx.Err() != nil {
				o.Status = "probe_cancelled"
			} else if err != nil {
				o.Status = "probe_failed"
			} else {
				match := spec.pattern.FindStringSubmatch(strings.TrimSpace(string(body)))
				if len(match) != 2 {
					o.Status = "version_unrecognized"
				} else {
					o.Status = "observed"
					o.Version = &match[1]
				}
			}
			cancel()
		}
		observations = append(observations, o)
	}
	return observations
}

type boundedOutput struct {
	body  []byte
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-len(b.body) {
		return 0, errors.New("version output exceeds limit")
	}
	b.body = append(b.body, p...)
	return len(p), nil
}

func runProbe(ctx context.Context, path string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.WaitDelay = 100 * time.Millisecond
	output := boundedOutput{limit: 2048}
	cmd.Stdout = &output
	// A nil stderr is discarded by os/exec, never echoed into the run record.
	if err := cmd.Run(); err != nil {
		return nil, errors.New("version probe failed")
	}
	return output.body, nil
}
