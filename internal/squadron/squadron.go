package squadron

// AgentBranch pairs an agent name with its working branch.
// Used by BuildMergerSuffix to render the merge list.
type AgentBranch struct {
	Name   string
	Branch string
}

// BuildConsensusSuffix returns the prompt suffix appended to every agent's
// prompt based on the consensus type. Returns "" for "none" (no suffix).
//
//	consensusType: "universal" | "review_master" | "none"
//	squadronName:  short name of the squadron (channel is "squadron-<name>")
//	agents:        all agent names in the squadron (caller order preserved)
//	reviewMaster:  name of the review master (only used when type=="review_master")
//	baseBranch:    the branch squadron/<name> is cut from (used in git diff hints)
//
// When consensusType == "review_master", the suffix returned is the one for
// non-reviewer agents UNLESS the caller wants the reviewer's suffix — see
// BuildReviewMasterSuffix for that case.
func BuildConsensusSuffix(consensusType, squadronName string, agents []string, reviewMaster, baseBranch string) string {
	if consensusType == "none" {
		return ""
	}
	return ""
}
