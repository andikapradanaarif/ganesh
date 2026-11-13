// ganesh-dev hosts optional Go validation tools, never hook execution.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andikapradanaarif/ganesh/internal/devtools"
	"github.com/andikapradanaarif/ganesh/internal/safetyprobe"
)

type options struct {
	command, source, profile, directory, study, raw, transcriptRoot, sessionsRoot, grantMapDirectory, expectedManifest, outputDirectory, expectedFrame, expectedResponse, expectedUsage, caseID string
	execute                                                                                                                                                                                     bool
}

func parse(args []string) (options, error) {
	var opts options
	if len(args) == 0 {
		return opts, devtools.ErrCommand
	}
	opts.command = args[0]
	flags := flag.NewFlagSet(opts.command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	switch opts.command {
	case "grader-trial-prepare":
		flags.StringVar(&opts.directory, "dir", "", "private corrected contract packet")
		flags.StringVar(&opts.expectedManifest, "manifest-sha256", "", "pinned contract manifest")
		flags.StringVar(&opts.outputDirectory, "output-dir", "", "new private owner-run directory")
	case "grader-trial-run", "grader-trial-report":
		flags.StringVar(&opts.directory, "dir", "", "private frozen trial directory")
		flags.StringVar(&opts.expectedManifest, "plan-sha256", "", "pinned trial plan")
		if opts.command == "grader-trial-run" {
			flags.BoolVar(&opts.execute, "execute", false, "send the frozen 180-request Go batch")
		}
	case "grader-contract-prepare":
		flags.StringVar(&opts.directory, "dir", "", "private completed #126 panel directory")
		flags.StringVar(&opts.outputDirectory, "output-dir", "", "new private development packet directory")
	case "grader-contract-check":
		flags.StringVar(&opts.directory, "dir", "", "private prospective development packet directory")
		flags.StringVar(&opts.expectedManifest, "manifest-sha256", "", "pinned packet manifest SHA-256")
		flags.StringVar(&opts.caseID, "case", "", "neutral packet case ID")
		flags.StringVar(&opts.expectedResponse, "response-sha256", "", "pinned raw response.txt SHA-256")
	case "issue-state-probe", "repository-head-probe", "ci-result-probe", "decision-authority-probe", "echo-origin-probe":
		// Fixed embedded synthetic fixtures only; no external paths or endpoints.
	case "crush-test":
		flags.StringVar(&opts.source, "source", "", "pinned source checkout")
		flags.StringVar(&opts.profile, "build-profile", "development", "fixed build profile")
	case "study-summary":
		flags.StringVar(&opts.directory, "dir", "", "private completed study directory")
		flags.StringVar(&opts.study, "study", "", "completed study: 34 or 36")
	case "corpus-audit":
		flags.StringVar(&opts.directory, "dir", "", "private corpus directory")
		flags.StringVar(&opts.raw, "raw", "", "comma-separated private raw snapshot basenames")
	case "calibration-summary":
		flags.StringVar(&opts.directory, "dir", "", "private saved calibration label directory")
		flags.StringVar(&opts.expectedFrame, "frame-sha256", "", "pinned immutable blank packet SHA-256")
	case "grader-evidence-check":
		flags.StringVar(&opts.directory, "dir", "", "private single-case answer and evidence directory")
		flags.StringVar(&opts.expectedFrame, "frame-sha256", "", "pinned answer/requirements frame SHA-256")
		flags.StringVar(&opts.expectedResponse, "response-sha256", "", "pinned structured grader response SHA-256")
	case "go-usage-check":
		flags.StringVar(&opts.directory, "dir", "", "private saved Go usage directory")
		flags.StringVar(&opts.expectedUsage, "usage-sha256", "", "pinned native usage object SHA-256")
	case "devin-audit", "devin-candidates":
		flags.StringVar(&opts.directory, "dir", "", "private Devin evidence directory")
		flags.StringVar(&opts.transcriptRoot, "transcript-root", "", "explicit Devin transcript directory")
		flags.StringVar(&opts.grantMapDirectory, "grant-map-dir", "", "private owner-grant project-root map directory")
		if opts.command == "devin-candidates" {
			flags.StringVar(&opts.expectedManifest, "manifest-sha256", "", "pinned source manifest SHA-256")
			flags.StringVar(&opts.outputDirectory, "output-dir", "", "existing private candidate output directory")
		}
	case "codex-audit":
		flags.StringVar(&opts.directory, "manifest-dir", "", "private Codex manifest directory")
		flags.StringVar(&opts.sessionsRoot, "sessions-root", "", "explicit Codex sessions directory")
		flags.StringVar(&opts.grantMapDirectory, "grant-map-dir", "", "private owner-grant project-root map directory")
	default:
		return options{}, devtools.ErrCommand
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return options{}, devtools.ErrCommand
	}
	if strings.HasPrefix(opts.command, "grader-trial-") && (strings.TrimSpace(opts.directory) == "" || len(opts.expectedManifest) != 64 || strings.Trim(opts.expectedManifest, "0123456789abcdef") != "" || (opts.command == "grader-trial-prepare" && strings.TrimSpace(opts.outputDirectory) == "") || (opts.command == "grader-trial-run" && !opts.execute)) {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "grader-contract-prepare" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.outputDirectory) == "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "grader-contract-check" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.caseID) == "" || len(opts.expectedManifest) != 64 || strings.Trim(opts.expectedManifest, "0123456789abcdef") != "" || len(opts.expectedResponse) != 64 || strings.Trim(opts.expectedResponse, "0123456789abcdef") != "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "crush-test" && (strings.TrimSpace(opts.source) == "" || (opts.profile != "development" && opts.profile != "stripped" && opts.profile != "collector")) {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "study-summary" && (strings.TrimSpace(opts.directory) == "" || (opts.study != "34" && opts.study != "36")) {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "corpus-audit" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.raw) == "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "calibration-summary" && (strings.TrimSpace(opts.directory) == "" || len(opts.expectedFrame) != 64 || strings.Trim(opts.expectedFrame, "0123456789abcdef") != "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "grader-evidence-check" && (strings.TrimSpace(opts.directory) == "" || len(opts.expectedFrame) != 64 || strings.Trim(opts.expectedFrame, "0123456789abcdef") != "" || len(opts.expectedResponse) != 64 || strings.Trim(opts.expectedResponse, "0123456789abcdef") != "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "go-usage-check" && (strings.TrimSpace(opts.directory) == "" || len(opts.expectedUsage) != 64 || strings.Trim(opts.expectedUsage, "0123456789abcdef") != "") {
		return options{}, devtools.ErrCommand
	}
	if (opts.command == "devin-audit" || opts.command == "devin-candidates") && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.transcriptRoot) == "" || strings.TrimSpace(opts.grantMapDirectory) == "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "devin-candidates" && (len(opts.expectedManifest) != 64 || strings.TrimSpace(opts.outputDirectory) == "") {
		return options{}, devtools.ErrCommand
	}
	if opts.command == "codex-audit" && (strings.TrimSpace(opts.directory) == "" || strings.TrimSpace(opts.sessionsRoot) == "" || strings.TrimSpace(opts.grantMapDirectory) == "") {
		return options{}, devtools.ErrCommand
	}
	return opts, nil
}

func repositoryRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", devtools.ErrCommand
	}
	for {
		f, err := os.Open(filepath.Join(dir, "go.mod"))
		if err == nil {
			body, readErr := io.ReadAll(io.LimitReader(f, 4097))
			closeErr := f.Close()
			if readErr != nil || closeErr != nil || len(body) > 4096 || !strings.HasPrefix(string(body), "module github.com/andikapradanaarif/ganesh\n") {
				return "", devtools.ErrCommand
			}
			return dir, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(dir) == dir {
			return "", devtools.ErrCommand
		}
		dir = filepath.Dir(dir)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	opts, err := parse(args)
	if err != nil {
		return err
	}
	if strings.HasPrefix(opts.command, "grader-trial-") {
		var report any
		switch opts.command {
		case "grader-trial-prepare":
			report, err = devtools.PrepareGraderTrial(opts.directory, opts.expectedManifest, opts.outputDirectory)
		case "grader-trial-run":
			report, err = devtools.RunGraderTrial(opts.directory, opts.expectedManifest, stdout)
		case "grader-trial-report":
			report, err = devtools.ReportGraderTrial(opts.directory, opts.expectedManifest)
		}
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil {
			return devtools.ErrGraderTrial
		}
		return nil
	}
	if opts.command == "grader-contract-prepare" || opts.command == "grader-contract-check" {
		var report any
		if opts.command == "grader-contract-prepare" {
			report, err = devtools.PrepareGraderContract(opts.directory, opts.outputDirectory)
		} else {
			report, err = devtools.CheckGraderContract(opts.directory, opts.expectedManifest, opts.caseID, opts.expectedResponse)
		}
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil {
			return devtools.ErrGraderContract
		}
		return nil
	}
	if opts.command == "issue-state-probe" {
		report, err := safetyprobe.RunIssueState()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil || report.Mismatched != 0 {
			return safetyprobe.ErrIssueStateProbe
		}
		return nil
	}
	if opts.command == "repository-head-probe" {
		report, err := safetyprobe.RunRepositoryHead()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil || report.Mismatched != 0 {
			return safetyprobe.ErrRepositoryHeadProbe
		}
		return nil
	}
	if opts.command == "ci-result-probe" {
		report, err := safetyprobe.RunCIResult()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil || report.Mismatched != 0 {
			return safetyprobe.ErrCIResultProbe
		}
		return nil
	}
	if opts.command == "decision-authority-probe" {
		report, err := safetyprobe.RunDecisionAuthority()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil || report.Mismatched != 0 {
			return safetyprobe.ErrDecisionAuthorityProbe
		}
		return nil
	}
	if opts.command == "echo-origin-probe" {
		report, err := safetyprobe.RunEchoOrigins()
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil || report.Mismatched != 0 {
			return safetyprobe.ErrEchoOriginProbe
		}
		return nil
	}
	if opts.command == "study-summary" {
		study := 34
		if opts.study == "36" {
			study = 36
		}
		report, err := devtools.SummarizeStudy(study, opts.directory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrStudy
		}
		return nil
	}
	if opts.command == "corpus-audit" {
		report, err := devtools.AuditCorpus(opts.directory, strings.Split(opts.raw, ","))
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrCorpus
		}
		return nil
	}
	if opts.command == "calibration-summary" {
		report, err := devtools.SummarizeCalibration(opts.directory, opts.expectedFrame)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil {
			return devtools.ErrCalibration
		}
		return nil
	}
	if opts.command == "grader-evidence-check" {
		report, err := devtools.CheckGraderEvidence(opts.directory, opts.expectedFrame, opts.expectedResponse)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil {
			return devtools.ErrGraderEvidence
		}
		return nil
	}
	if opts.command == "go-usage-check" {
		report, err := devtools.CheckGoUsage(opts.directory, opts.expectedUsage)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if encoder.Encode(report) != nil {
			return devtools.ErrGoUsage
		}
		return nil
	}
	if opts.command == "devin-audit" {
		report, err := devtools.AuditDevin(opts.directory, opts.transcriptRoot, opts.grantMapDirectory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrDevinAudit
		}
		return nil
	}
	if opts.command == "devin-candidates" {
		report, err := devtools.PrepareDevinCandidates(opts.directory, opts.transcriptRoot, opts.grantMapDirectory, opts.expectedManifest, opts.outputDirectory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrDevinCandidates
		}
		return nil
	}
	if opts.command == "codex-audit" {
		report, err := devtools.AuditCodex(opts.directory, opts.sessionsRoot, opts.grantMapDirectory)
		if err != nil {
			return err
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return devtools.ErrCodexAudit
		}
		return nil
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	return devtools.RunCrush(context.Background(), root, opts.source, opts.profile, stdout, stderr)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
