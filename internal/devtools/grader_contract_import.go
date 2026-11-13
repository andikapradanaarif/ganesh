package devtools

import (
	"encoding/json"
	"fmt"
	"strings"
)

type contractLegacyLabels struct {
	Completion  *bool `json:"completion"`
	Critical    *bool `json:"introduced_critical_error"`
	Unsupported *bool `json:"unsupported_claim"`
}
type contractLegacyCase struct {
	ID       string           `json:"id"`
	Task     string           `json:"task"`
	Kind     string           `json:"kind"`
	Question string           `json:"question"`
	Sources  []contractSource `json:"sources"`
	Answer   string           `json:"answer"`
	Rubric   struct {
		Required   []string `json:"required"`
		Critical   []string `json:"critical_failures"`
		Abstention string   `json:"abstention"`
	} `json:"rubric"`
	Reference contractLegacyLabels `json:"reference"`
}

// PrepareGraderContract imports only the existing #126 development cases.
// Archive parsing tolerates its historical metadata; new packets are strict.
func PrepareGraderContract(directory, output string) (GraderContractPreparation, error) {
	var empty GraderContractPreparation
	if !contractOutputName(output) {
		return empty, ErrGraderContract
	}
	r, err := contractPrivateRoot(directory)
	if err != nil {
		return empty, err
	}
	defer r.Close()
	inputs := map[string]string{}
	bodies := map[string][]byte{}
	for _, name := range []string{"plan.json", "approval.json", "judging-v2/bindings.json", "run-1/generated-cases.json", "run-1/new-references.json", "run-1/reference-quality.json", "run-1/semantic-review.json"} {
		b, e := privateFile(r, name, 2<<20)
		if e != nil || uniqueJSON(b) != nil {
			return empty, ErrGraderContract
		}
		bodies[name] = b
		inputs[name] = corpusHash(string(b))
	}
	var approval struct {
		Plan string `json:"plan_sha256"`
	}
	var bindings struct {
		Files map[string]string `json:"files_sha256"`
	}
	if json.Unmarshal(bodies["approval.json"], &approval) != nil || approval.Plan != inputs["plan.json"] || json.Unmarshal(bodies["judging-v2/bindings.json"], &bindings) != nil {
		return empty, ErrGraderContract
	}
	for _, name := range []string{"run-1/generated-cases.json", "run-1/new-references.json", "run-1/reference-quality.json"} {
		matches := 0
		for path, pin := range bindings.Files {
			if strings.HasSuffix(path, "/"+name) && pin == inputs[name] {
				matches++
			}
		}
		if matches != 1 {
			return empty, ErrGraderContract
		}
	}
	var plan struct {
		Issue int                  `json:"issue"`
		Cases []contractLegacyCase `json:"cases"`
	}
	var generated []contractLegacyCase
	var fresh map[string]contractLegacyLabels
	var quality struct {
		Met       map[string][]*bool  `json:"expected_criterion_met_proposals"`
		Ambiguous map[string][]string `json:"known_criterion_ambiguities"`
	}
	var review struct {
		Disputes []struct {
			Cases []string `json:"cases"`
			Field string   `json:"field"`
		} `json:"reference_disputes_preserved"`
	}
	if json.Unmarshal(bodies["plan.json"], &plan) != nil || plan.Issue != 126 || len(plan.Cases) != 30 || json.Unmarshal(bodies["run-1/generated-cases.json"], &generated) != nil || len(generated) != 12 || json.Unmarshal(bodies["run-1/new-references.json"], &fresh) != nil || len(fresh) != 12 || json.Unmarshal(bodies["run-1/reference-quality.json"], &quality) != nil || json.Unmarshal(bodies["run-1/semantic-review.json"], &review) != nil {
		return empty, ErrGraderContract
	}
	cases := []contractLegacyCase{}
	slots := map[string]contractLegacyCase{}
	for _, c := range plan.Cases {
		switch c.Kind {
		case "development_control":
			cases = append(cases, c)
		case "prospective_natural_answer":
			if _, exists := slots[c.ID]; exists {
				return empty, ErrGraderContract
			}
			slots[c.ID] = c
		default:
			return empty, ErrGraderContract
		}
	}
	if len(cases) != 18 || len(slots) != 12 {
		return empty, ErrGraderContract
	}
	seenGenerated := map[string]bool{}
	for _, c := range generated {
		slot, ok := slots[c.ID]
		ref, hasRef := fresh[c.ID]
		// Retain exactly the frozen question, source text and rubric. Only the
		// generated answer and separately frozen reference may fill the slot.
		a, b := slot, c
		a.Answer = ""
		b.Answer = ""
		a.Reference = contractLegacyLabels{}
		b.Reference = contractLegacyLabels{}
		if !ok || !hasRef || seenGenerated[c.ID] || string(contractJSON(a)) != string(contractJSON(b)) {
			return empty, ErrGraderContract
		}
		seenGenerated[c.ID] = true
		c.Reference = ref
		cases = append(cases, c)
	}
	packet := contractPacket{Version: 1, Cases: []contractCase{}, Requests: []contractRequest{}}
	refs := map[string]contractReference{}
	tasks := map[string]string{}
	seen := map[string]bool{}
	for i, old := range cases {
		if old.ID == "" || old.Task == "" || seen[old.ID] {
			return empty, ErrGraderContract
		}
		seen[old.ID] = true
		task, ok := tasks[old.Task]
		if !ok {
			task = fmt.Sprintf("task-%02d", len(tasks)+1)
			tasks[old.Task] = task
		}
		c := contractCase{Version: 1, Case: fmt.Sprintf("case-%02d", i+1), Task: task, Question: old.Question, Sources: old.Sources, Answer: old.Answer, Criteria: []contractRule{}, CriticalRules: []contractRule{}, Abstention: old.Rubric.Abstention}
		for n, text := range old.Rubric.Required {
			c.Criteria = append(c.Criteria, contractRule{fmt.Sprintf("C%d", n+1), text})
		}
		for n, text := range old.Rubric.Critical {
			c.CriticalRules = append(c.CriticalRules, contractRule{fmt.Sprintf("K%d", n+1), text})
		}
		if !contractCaseValid(c) {
			return empty, ErrGraderContract
		}
		ref := contractReference{OriginalCase: old.ID, Basis: "same_author_proposal; previously inspected development case; not independent truth", Completion: old.Reference.Completion, Critical: old.Reference.Critical, Unsupported: old.Reference.Unsupported, Criteria: make([]*bool, 4), Disputed: []string{}}
		if met, ok := quality.Met[old.ID]; ok {
			if len(met) != 4 {
				return empty, ErrGraderContract
			}
			ref.Criteria = met
		}
		for _, field := range quality.Ambiguous[old.ID] {
			if !evidenceCriterionID(field) {
				return empty, ErrGraderContract
			}
			ref.Disputed = append(ref.Disputed, field)
		}
		for _, dispute := range review.Disputes {
			for _, id := range dispute.Cases {
				if id == old.ID {
					if dispute.Field != "unsupported_claim" && dispute.Field != "introduced_critical_error" && dispute.Field != "completion" {
						return empty, ErrGraderContract
					}
					ref.Disputed = append(ref.Disputed, dispute.Field)
				}
			}
		}
		refs[c.Case] = ref
		packet.Cases = append(packet.Cases, c)
		packet.Requests = append(packet.Requests, contractPrompt(c))
	}
	if len(tasks) != 6 {
		return empty, ErrGraderContract
	}
	return contractOutput(output, packet, refs, inputs, len(tasks))
}
