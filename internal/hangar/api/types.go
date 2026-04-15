package api

type FleetResponse struct {
	RepoPath      string          `json:"repoPath"`
	CurrentBranch string          `json:"currentBranch"`
	Agents        []AgentResponse `json:"agents"`
}

type AgentResponse struct {
	Name          string `json:"name"`
	Branch        string `json:"branch"`
	Status        string `json:"status"`
	Driver        string `json:"driver"`
	HooksOK       bool   `json:"hooksOK"`
	StateFilePath string `json:"stateFilePath,omitempty"`
}

type PersonaResponse struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Preamble    string `json:"preamble"`
}

type DriverResponse struct {
	Name string `json:"name"`
}

type LaunchRequest struct {
	Name         string             `json:"name"`
	Consensus    string             `json:"consensus"`
	ReviewMaster string             `json:"reviewMaster,omitempty"`
	BaseBranch   string             `json:"baseBranch,omitempty"`
	AutoMerge    bool               `json:"autoMerge"`
	AutoPR       bool               `json:"autoPR,omitempty"`
	MergeMaster  *string            `json:"mergeMaster,omitempty"`
	UseJumpSh    bool               `json:"useJumpSh,omitempty"`
	Agents       []LaunchAgentInput `json:"agents"`
}

type LaunchAgentInput struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Prompt  string `json:"prompt"`
	Driver  string `json:"driver,omitempty"`
	Persona string `json:"persona,omitempty"`
}

type LaunchResponse struct {
	MergeMaster string `json:"mergeMaster,omitempty"`
}

type GenerateRequest struct {
	Description string `json:"description"`
}

type GenerateResponse struct {
	Agents []LaunchAgentInput `json:"agents"`
}

type ErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}
