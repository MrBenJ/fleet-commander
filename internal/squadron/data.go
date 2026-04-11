package squadron

import (
	"bytes"
	"encoding/json"
	"fmt"
)

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

	return data, nil
}
