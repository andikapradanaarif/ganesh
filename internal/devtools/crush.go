// Package devtools contains optional developer tooling, outside the capture CLI.
package devtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	BaselineRevision = "aed2bae006adc2f17c9ea2c693e2ca9cb4eca84f"
	EndingRevision   = "3378f47a8398c79a10e35cc61c994160cc0ae3ea"
)

var (
	ErrSource  = errors.New("pinned clean Crush source required")
	ErrFixture = errors.New("Crush fixture incomplete")
	ErrCommand = errors.New("invalid developer command")
)

type invocation struct {
	dir, name string
	args, env []string
	timeout   time.Duration
	stdout    io.Writer // both nil means bounded, private stdout capture
	stderr    io.Writer // metadata stderr is discarded; streamed stages keep it separate
}

type execute func(context.Context, invocation) ([]byte, error)

type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	if len(p) > (64<<10)-b.Len() {
		return 0, ErrFixture
	}
	return b.Buffer.Write(p)
}

func executeCommand(ctx context.Context, call invocation) ([]byte, error) {
	if (call.stdout == nil) != (call.stderr == nil) {
		return nil, ErrFixture
	}
	ctx, cancel := context.WithTimeout(ctx, call.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, call.name, call.args...)
	cmd.Dir, cmd.Env, cmd.WaitDelay = call.dir, call.env, time.Second
	var output limitedOutput
	if call.stdout == nil {
		cmd.Stdout = &output
	} else {
		cmd.Stdout, cmd.Stderr = call.stdout, call.stderr
	}
	if err := cmd.Run(); err != nil || ctx.Err() != nil {
		return nil, ErrFixture // never format paths, environments or child errors
	}
	return output.Bytes(), nil
}

func buildProfile(profile string) ([]string, string, error) {
	switch profile {
	case "development":
		return nil, "./cmd/ganesh", nil
	case "stripped":
		return []string{"-ldflags=-s -w"}, "./cmd/ganesh", nil
	case "collector":
		return []string{"-ldflags=-s -w"}, "./cmd/ganesh-capture", nil
	default:
		return nil, "", ErrCommand
	}
}

func offlineEnvironment(env []string) ([]string, error) {
	result := make([]string, 0, len(env)+6)
	for _, value := range env {
		key, val, _ := strings.Cut(value, "=")
		if (key == "GOFLAGS" || key == "GODEBUG") && val != "" {
			return nil, ErrFixture
		}
		switch key {
		case "GOPROXY", "GOSUMDB", "GOTOOLCHAIN", "GOWORK", "GANESH_TEST_BINARY", "GANESH_TEST_FIXED_ENDINGS":
			continue
		}
		result = append(result, value)
	}
	return append(result, "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local", "GOWORK=off"), nil
}

// RunCrush exercises synthetic fixtures through one of two exact source pins.
// It changes neither the source checkout nor installed client configuration.
func RunCrush(ctx context.Context, root, source, profile string, stdout, stderr io.Writer) error {
	return runCrush(ctx, root, source, profile, stdout, stderr, os.Environ(), executeCommand)
}

func runCrush(ctx context.Context, root, source, profile string, stdout, stderr io.Writer, environment []string, run execute) error {
	if stdout == nil || stderr == nil {
		return ErrFixture
	}
	flags, target, err := buildProfile(profile)
	if err != nil {
		return err
	}
	env, err := offlineEnvironment(environment)
	if err != nil {
		return err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return ErrSource
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return ErrSource
	}
	git := func(args ...string) ([]byte, error) {
		return run(ctx, invocation{dir: source, name: "git", args: args, env: env, timeout: 5 * time.Second})
	}
	revisionBytes, err := git("rev-parse", "HEAD")
	revision := strings.TrimSpace(string(revisionBytes))
	if err != nil || (revision != BaselineRevision && revision != EndingRevision) {
		return ErrSource
	}
	if _, err := git("diff", "--quiet", "HEAD", "--"); err != nil {
		return ErrSource
	}
	replacements := map[string]string{
		filepath.Join(source, "internal/hooks/ganesh_adapter_test.go"):     filepath.Join(root, "integration/crush/runner_test.go.fixture"),
		filepath.Join(source, "internal/agent/ganesh_turn_ending_test.go"): filepath.Join(root, "integration/crush/turn_ending_test.go.fixture"),
	}
	for virtual, fixture := range replacements {
		if _, err := os.Lstat(virtual); !errors.Is(err, os.ErrNotExist) {
			return ErrSource
		}
		if info, err := os.Stat(fixture); err != nil || !info.Mode().IsRegular() {
			return ErrFixture
		}
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard", "--", "*.go")
	if err != nil || len(untracked) != 0 {
		return ErrSource
	}
	fixed := "0"
	if revision == EndingRevision {
		fixed = "1"
	}
	env = append(env, "GANESH_TEST_FIXED_ENDINGS="+fixed)
	if _, err := fmt.Fprintf(stdout, "Source revision: %s; corrected ending dispatch: %s\n", revision, fixed); err != nil {
		return ErrFixture
	}
	temp, err := os.MkdirTemp("", "ganesh-crush-runner-")
	if err != nil {
		return ErrFixture
	}
	defer os.RemoveAll(temp)
	binary := filepath.Join(temp, "ganesh")
	buildArgs := append([]string{"build"}, flags...)
	buildArgs = append(buildArgs, "-o", binary, target)
	started := time.Now()
	if _, err := run(ctx, invocation{dir: root, name: "go", args: buildArgs, env: env, timeout: 60 * time.Second, stdout: stdout, stderr: stderr}); err != nil {
		return ErrFixture
	}
	if _, err := fmt.Fprintf(stdout, "Ganesh build completed in %.3f s; profile: %s\n", time.Since(started).Seconds(), profile); err != nil {
		return ErrFixture
	}
	body, err := json.Marshal(struct{ Replace map[string]string }{replacements})
	if err != nil {
		return ErrFixture
	}
	overlay := filepath.Join(temp, "overlay.json")
	if err := os.WriteFile(overlay, body, 0o600); err != nil {
		return ErrFixture
	}
	env = append(env, "GANESH_TEST_BINARY="+binary)
	started = time.Now()
	args := []string{"test", "-p=1", "-count=1", "-overlay", overlay, "./internal/hooks", "./internal/agent", "-run", "^TestGanesh(ToolHookAdapter|TurnEndingCancellation)$", "-timeout=20s", "-v"}
	if _, err := run(ctx, invocation{dir: source, name: "go", args: args, env: env, timeout: 60 * time.Second, stdout: stdout, stderr: stderr}); err != nil {
		return ErrFixture
	}
	if _, err := fmt.Fprintf(stdout, "Crush fixture stage completed in %.3f s\n", time.Since(started).Seconds()); err != nil {
		return ErrFixture
	}
	return nil
}
