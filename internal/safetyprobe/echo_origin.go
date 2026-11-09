package safetyprobe

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
)

var ErrEchoOriginProbe = errors.New("synthetic echo-origin probe failed")

// Frozen before implementation in issue #73. No external graph is admitted.
//
//go:embed echo_origin_v1.json
var echoFixtures []byte

const echoFixtureDigest = "d4bff21a0ee3620c1815c1f4e322878cddc1b31c9a9ad7cfcf23759a865334ee"

type echoNode struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Project       string   `json:"project"`
	Parents       []string `json:"parents"`
	Source        string   `json:"source"`
	Object        string   `json:"object"`
	Revision      string   `json:"revision"`
	Class         string   `json:"class"`         // Only source nodes supply declared root classes; derived claims are ignored.
	Authenticated bool     `json:"authenticated"` // A transport label cannot replace ancestry or upgrade a root.
	AuthorRole    string   `json:"author_role"`   // Ignored; never an origin or authority grant.
	Prose         string   `json:"prose"`         // Ignored; never parsed for links, classes or instructions.
}

type echoGraph struct {
	Project string     `json:"project"`
	Nodes   []echoNode `json:"nodes"`
	Targets []string   `json:"targets"`
}

type echoOrigin struct{ source, object string }
type echoRevision struct {
	origin   echoOrigin
	revision string
}

type echoTrace struct {
	height int
	roots  map[echoRevision]string
}

type echoOutcome struct {
	Kind                    string `json:"kind"`
	Reason                  string `json:"reason"`
	Level                   string `json:"level"`
	DistinctOrigins         int    `json:"distinct_origins"`
	OriginRevisions         int    `json:"origin_revisions"`
	UntrustedOrigins        int    `json:"untrusted_origins"`
	UnknownOrigins          int    `json:"unknown_origins"`
	StructuredOrigins       int    `json:"structured_origins"`
	IndependenceEstablished bool   `json:"independence_established"`
}

// traceEchoOrigins checks a closed, declared synthetic snapshot. It establishes
// neither real provenance completeness nor source truth/independence. Expected
// outcomes and case names are not supplied to this tracer.
func traceEchoOrigins(g echoGraph) echoOutcome {
	incomplete := func(reason string) echoOutcome {
		return echoOutcome{Kind: "incomplete_lineage", Reason: reason, Level: "unknown"}
	}
	if !revisionID.MatchString(g.Project) {
		return incomplete("invalid_policy")
	}
	if len(g.Nodes) == 0 || len(g.Nodes) > 32 {
		return incomplete("node_limit")
	}
	if len(g.Targets) == 0 || len(g.Targets) > 8 {
		return incomplete("target_limit")
	}
	index := make(map[string]echoNode)
	classes := make(map[echoOrigin]string)
	for _, n := range g.Nodes {
		if !revisionID.MatchString(n.ID) {
			return incomplete("invalid_node_id")
		}
		if _, exists := index[n.ID]; exists {
			return incomplete("duplicate_node")
		}
		if n.Project != g.Project {
			return incomplete("foreign_project")
		}
		switch n.Kind {
		case "source":
			if len(n.Parents) != 0 {
				return incomplete("invalid_root_edges")
			}
			if !revisionID.MatchString(n.Source) || !revisionID.MatchString(n.Object) || !revisionID.MatchString(n.Revision) {
				return incomplete("unresolved_origin")
			}
			if n.Class != "untrusted_narrative" && n.Class != "unknown" && n.Class != "structured_observation" {
				return incomplete("invalid_origin_class")
			}
			origin := echoOrigin{n.Source, n.Object}
			if prior, exists := classes[origin]; exists && prior != n.Class {
				return incomplete("conflicting_origin_class")
			}
			classes[origin] = n.Class
		case "tool", "agent", "copy":
			if len(n.Parents) == 0 {
				return incomplete("missing_ancestry")
			}
		default:
			return incomplete("invalid_node_kind")
		}
		if len(n.Parents) > 8 {
			return incomplete("parent_limit")
		}
		for _, parent := range n.Parents {
			if !revisionID.MatchString(parent) {
				return incomplete("invalid_parent_id")
			}
		}
		index[n.ID] = n
	}
	for _, n := range g.Nodes {
		for _, parent := range n.Parents {
			if _, exists := index[parent]; !exists {
				return incomplete("missing_parent")
			}
		}
	}
	for _, target := range g.Targets {
		if !revisionID.MatchString(target) {
			return incomplete("invalid_target_id")
		}
		if _, exists := index[target]; !exists {
			return incomplete("missing_target")
		}
	}
	state := make(map[string]uint8)
	memo := make(map[string]echoTrace)
	var walk func(string) (echoTrace, string)
	walk = func(id string) (echoTrace, string) {
		if state[id] == 1 {
			return echoTrace{}, "cycle"
		}
		if state[id] == 2 {
			return memo[id], ""
		}
		state[id] = 1
		n := index[id]
		trace := echoTrace{height: 1, roots: make(map[echoRevision]string)}
		if n.Kind == "source" {
			trace.roots[echoRevision{echoOrigin{n.Source, n.Object}, n.Revision}] = n.Class
		} else {
			for _, parent := range n.Parents {
				ancestry, reason := walk(parent)
				if reason != "" {
					return echoTrace{}, reason
				}
				if ancestry.height+1 > trace.height {
					trace.height = ancestry.height + 1
				}
				for root, class := range ancestry.roots {
					trace.roots[root] = class
				}
			}
		}
		// Memoize longest ancestry height, not the first traversal's depth.
		// Shared DAG paths cannot bypass the bound through a cached short path.
		if trace.height > 16 {
			return echoTrace{}, "depth_limit"
		}
		state[id], memo[id] = 2, trace
		return trace, ""
	}
	// Validate the entire snapshot before exposing any selected root counts.
	for _, n := range g.Nodes {
		if _, reason := walk(n.ID); reason != "" {
			return incomplete(reason)
		}
	}
	roots := make(map[echoRevision]string)
	for _, target := range g.Targets {
		for root, class := range memo[target].roots {
			roots[root] = class
		}
	}
	origins := make(map[echoOrigin]string)
	for root, class := range roots {
		origins[root.origin] = class
	}
	out := echoOutcome{Kind: "reported_lineage", Reason: "declared_ancestry_traced", Level: "reported", DistinctOrigins: len(origins), OriginRevisions: len(roots)}
	for _, class := range origins {
		switch class {
		case "untrusted_narrative":
			out.UntrustedOrigins++
		case "unknown":
			out.UnknownOrigins++
		case "structured_observation":
			out.StructuredOrigins++
		}
	}
	return out
}

type echoCase struct {
	Name     string      `json:"name"`
	Graph    echoGraph   `json:"graph"`
	Expected echoOutcome `json:"expected"`
}

type echoFixtureSet struct {
	Schema string     `json:"schema"`
	Cases  []echoCase `json:"cases"`
}

type EchoOriginReport struct {
	Schema                  string `json:"schema"`
	FixtureSHA256           string `json:"fixture_sha256"`
	Status                  string `json:"status"`
	Cases                   int    `json:"cases"`
	Matched                 int    `json:"matched"`
	Mismatched              int    `json:"mismatched"`
	ReportedLineages        int    `json:"reported_lineages"`
	IncompleteLineages      int    `json:"incomplete_lineages"`
	IndependenceEstablished bool   `json:"independence_established"`
	Synthetic               bool   `json:"synthetic"`
	PublicationAuthorized   bool   `json:"publication_authorized"`
	StageGatePassed         bool   `json:"stage_gate_passed"`
}

type echoTracer func(echoGraph) echoOutcome

// The scorer alone receives expected counts/classes/qualification. It never
// asks the tracer to grade its own lineage or independence claim.
func compareEchoOrigins(body []byte, trace echoTracer) (EchoOriginReport, error) {
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	if digest != echoFixtureDigest {
		return EchoOriginReport{}, ErrEchoOriginProbe
	}
	var fixtures echoFixtureSet
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if d.Decode(&fixtures) != nil || fixtures.Schema != "synthetic-echo-origin-v1" || len(fixtures.Cases) != 50 {
		return EchoOriginReport{}, ErrEchoOriginProbe
	}
	if d.Decode(new(any)) != io.EOF {
		return EchoOriginReport{}, ErrEchoOriginProbe
	}
	report := EchoOriginReport{Schema: fixtures.Schema, FixtureSHA256: digest, Cases: len(fixtures.Cases), Synthetic: true, Status: "fixture_expectations_matched"}
	for _, row := range fixtures.Cases {
		got := trace(row.Graph)
		if got == row.Expected {
			report.Matched++
		} else {
			report.Mismatched++
		}
		switch got.Kind {
		case "reported_lineage":
			report.ReportedLineages++
		case "incomplete_lineage":
			report.IncompleteLineages++
		}
	}
	if report.Mismatched > 0 {
		report.Status = "fixture_mismatch"
	}
	return report, nil
}

func RunEchoOrigins() (EchoOriginReport, error) {
	return compareEchoOrigins(echoFixtures, traceEchoOrigins)
}
