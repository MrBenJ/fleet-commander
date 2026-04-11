package squadron

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// ValidName reports whether s is a valid squadron or agent name:
// non-empty, max 30 chars, matching ^[a-zA-Z0-9][a-zA-Z0-9_-]*$.
func ValidName(s string) bool {
	return s != "" && len(s) <= 30 && nameRe.MatchString(s)
}

type SquadronData struct {
	Name         string          `json:"name"`
	Consensus    string          `json:"consensus"`
	ReviewMaster string          `json:"reviewMaster,omitempty"`
	BaseBranch   string          `json:"baseBranch,omitempty"`
	AutoMerge    bool            `json:"autoMerge"`
	MergeMaster  *string         `json:"mergeMaster,omitempty"`
	UseJumpSh    bool            `json:"useJumpSh,omitempty"`
	Agents       []SquadronAgent `json:"agents"`
}

type SquadronAgent struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Prompt  string `json:"prompt"`
	Driver  string `json:"driver,omitempty"`
	Persona string `json:"persona,omitempty"`
}

type rawSquadronData struct {
	Name         string          `json:"name"`
	Consensus    string          `json:"consensus"`
	ReviewMaster string          `json:"reviewMaster,omitempty"`
	BaseBranch   string          `json:"baseBranch,omitempty"`
	AutoMerge    *bool           `json:"autoMerge,omitempty"`
	MergeMaster  *string         `json:"mergeMaster,omitempty"`
	UseJumpSh    bool            `json:"useJumpSh,omitempty"`
	Agents       []SquadronAgent `json:"agents"`
}

func ParseAndValidate(jsonBytes []byte) (*SquadronData, []error) {
	var raw rawSquadronData
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, []error{fmt.Errorf("invalid JSON: %w", err)}
	}

	data := &SquadronData{
		Name:         raw.Name,
		Consensus:    raw.Consensus,
		ReviewMaster: raw.ReviewMaster,
		BaseBranch:   raw.BaseBranch,
		MergeMaster:  raw.MergeMaster,
		UseJumpSh:    raw.UseJumpSh,
		Agents:       raw.Agents,
	}
	if raw.AutoMerge == nil {
		data.AutoMerge = true
	} else {
		data.AutoMerge = *raw.AutoMerge
	}

	var errs []error

	if data.Name == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	} else if len(data.Name) > 30 || !nameRe.MatchString(data.Name) {
		errs = append(errs, fmt.Errorf("name %q is invalid (must match %s, max 30 chars)", data.Name, nameRe.String()))
	}

	switch data.Consensus {
	case "universal", "review_master", "none":
	case "":
		errs = append(errs, fmt.Errorf("consensus is required"))
	default:
		errs = append(errs, fmt.Errorf("consensus %q is invalid (must be universal, review_master, or none)", data.Consensus))
	}

	if len(data.Agents) == 0 {
		errs = append(errs, fmt.Errorf("agents array must contain at least one agent"))
	}
	seen := map[string]bool{}
	for i, a := range data.Agents {
		if a.Name == "" {
			errs = append(errs, fmt.Errorf("agents[%d].name is required", i))
		} else if len(a.Name) > 30 || !nameRe.MatchString(a.Name) {
			errs = append(errs, fmt.Errorf("agents[%d].name %q is invalid", i, a.Name))
		}
		if a.Name != "" && seen[a.Name] {
			errs = append(errs, fmt.Errorf("duplicate agent name %q", a.Name))
		}
		seen[a.Name] = true

		if a.Branch == "" {
			errs = append(errs, fmt.Errorf("agents[%d].branch is required", i))
		}
		if a.Prompt == "" {
			errs = append(errs, fmt.Errorf("agents[%d].prompt is required", i))
		}
		if a.Persona != "" {
			if _, ok := LookupPersona(a.Persona); !ok {
				errs = append(errs, fmt.Errorf("agents[%d].persona %q is not a known persona", i, a.Persona))
			}
		}
	}

	switch data.Consensus {
	case "review_master":
		if data.ReviewMaster == "" {
			errs = append(errs, fmt.Errorf("reviewMaster is required when consensus is review_master"))
		} else if !seen[data.ReviewMaster] {
			errs = append(errs, fmt.Errorf("reviewMaster %q does not match any agent name", data.ReviewMaster))
		}
	default:
		if data.ReviewMaster != "" {
			errs = append(errs, fmt.Errorf("reviewMaster is only allowed when consensus is review_master"))
		}
	}

	if data.MergeMaster != nil && *data.MergeMaster != "" {
		if !seen[*data.MergeMaster] {
			errs = append(errs, fmt.Errorf("mergeMaster %q does not match any agent name", *data.MergeMaster))
		}
	}

	return data, errs
}
