package main

import (
	"io"

	"github.com/andikapradanaarif/ganesh/internal/s0"
)

func freezeRecheckCommand(input, expectedNeeds, outputPath string, output io.Writer) error {
	if expectedNeeds == "" || outputPath == "" {
		return s0.ErrRecheck
	}
	d, err := s0.ReadRecheckDefinition(input)
	if err != nil {
		return err
	}
	s, err := s0.FreezeRecheck(d, expectedNeeds)
	if err != nil {
		return err
	}
	if err := s0.SaveRecheckSnapshot(outputPath, s); err != nil {
		return err
	}
	return emit(output, struct {
		State                         string `json:"state"`
		SnapshotDigest                string `json:"snapshot_digest"`
		NeedDigest                    string `json:"frozen_need_digest"`
		EvidenceIndependentlyVerified bool   `json:"evidence_independently_verified"`
		ExperimentAccepted            bool   `json:"experiment_accepted"`
		ModelRunsAuthorized           bool   `json:"model_runs_authorized"`
		ProductionNarrativeEnabled    bool   `json:"production_narrative_enabled"`
	}{State: "recheck_snapshot_frozen", SnapshotDigest: s.Digest, NeedDigest: s.NeedDigest})
}

func recheckCommand(plan, expected, input string, output io.Writer) error {
	if expected == "" {
		return s0.ErrRecheck
	}
	s, err := s0.ReadRecheckSnapshot(plan)
	if err != nil {
		return err
	}
	// Reject replacement baselines before opening final assessment metadata.
	if err := s0.CheckRecheckAgainst(s, expected); err != nil {
		return err
	}
	in, err := s0.ReadRecheckInput(input)
	if err != nil {
		return err
	}
	r, err := s0.ReportRecheck(s, expected, in)
	if err != nil {
		return err
	}
	if err := emit(output, r); err != nil {
		return s0.ErrRecheck
	}
	if r.State != "declared_criteria_met" {
		return s0.ErrRecheckIncomplete
	}
	return nil
}
