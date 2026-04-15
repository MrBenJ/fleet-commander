package squadron

import (
	"fmt"
	"strings"
)

const universalTemplate = `---

## Squadron Consensus Protocol (UNIVERSAL)

You are a member of squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.

After completing your primary task, you MUST participate in the squadron review process:

1. Announce completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary of what you did>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-%s

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff %s...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-%s "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-%s "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If changes are requested on YOUR work, address them and re-announce:
   fleet context channel-send squadron-%s "REVISED: <summary of changes>"

6. Your work is ONLY complete when:
   - You have approved ALL other squadron members
   - ALL other squadron members have approved you

CRITICAL: Do NOT stop or exit after completing your review. You MUST continue polling the squadron channel every 30 seconds until ALL reviews are complete and ALL agents have been approved. If you have completed your work and reviewed all other agents but not everyone has approved everyone yet, KEEP POLLING. Your session must remain active.

If auto-merge is enabled, after all approvals are in, continue polling until the merge master posts MERGE_COMPLETE or MERGE_FAILED. Only then may you stop.

Squadron members: %s
`

// AgentBranch pairs an agent name with its working branch.
// Used by BuildMergerSuffix to render the merge list.
type AgentBranch struct {
	Name   string
	Branch string
}

const reviewMasterReviewerTemplate = `---

## Squadron Consensus Protocol (REVIEW MASTER)

You are the REVIEW MASTER for squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.

After completing your own primary task:

1. Announce your own completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-%s

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff %s...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-%s "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-%s "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If you requested changes, wait for their REVISED message, then re-review.

6. Once all agents are approved, post:
   fleet context channel-send squadron-%s "ALL_APPROVED: Squadron review complete"

CRITICAL: After posting ALL_APPROVED, if auto-merge is enabled, you MUST continue polling the channel every 30 seconds to monitor merge progress. Only stop when you see MERGE_COMPLETE or MERGE_FAILED. Do NOT exit your session prematurely.

Squadron members: %s
`

const reviewMasterNonReviewerTemplate = `---

## Squadron Consensus Protocol (REVIEW MASTER)

You are a member of squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.
Agent "%s" is the designated review master.

After completing your primary task:

1. Announce completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary of what you did>"

2. Poll for the review master's feedback (every 30 seconds):
   fleet context channel-read squadron-%s

3. If changes are requested on your work, address them and re-announce:
   fleet context channel-send squadron-%s "REVISED: <summary of changes>"

4. Your work is complete when the review master posts APPROVED for you.

CRITICAL: Do NOT stop after receiving approval. If auto-merge is enabled, continue polling the squadron channel every 30 seconds until the merge master posts MERGE_COMPLETE or MERGE_FAILED. Only then may you stop. If you are idle waiting, periodically run: fleet context channel-read squadron-%s

Squadron members: %s
Review master: %s
`

// BuildReviewMasterReviewerSuffix returns the suffix for the designated reviewer.
func BuildReviewMasterReviewerSuffix(squadronName string, agents []string, baseBranch string) string {
	return fmt.Sprintf(
		reviewMasterReviewerTemplate,
		squadronName, squadronName, squadronName, squadronName,
		baseBranch,
		squadronName, squadronName, squadronName,
		strings.Join(agents, ", "),
	)
}

const mergerTemplate = `---

## Squadron Merge Duties

You are also the MERGE MASTER for squadron "%s". After the squadron reaches consensus (all APPROVED for review modes, or all COMPLETED for none mode), you must merge everyone's work into a single squadron branch.

CRITICAL: Before starting the merge, verify ALL agents have reached consensus by reading the squadron channel. Do not begin merging until the review process is fully complete.

1. Create the squadron branch from the base:
   git checkout %s
   git checkout -b squadron/%s

2. Merge each agent's working branch in sequentially (in the order listed below):
   git merge --no-ff <agent-branch>

3. If a merge produces conflicts, resolve them yourself. Use each agent's original prompt (available in the squadron channel) as context for what they were trying to accomplish. Prefer preserving all agents' intent.

4. After all merges succeed, announce:
   fleet context channel-send squadron-%s "MERGE_COMPLETE: squadron/%s"

5. If a merge fails and you cannot resolve it safely, announce:
   fleet context channel-send squadron-%s "MERGE_FAILED: <agent-name> - <reason>"
   and stop. Do not force-merge or discard changes.

Agent branches to merge (in order):
%s
`

const noConsensusAutoMergeTemplate = `---

## Squadron Merge Monitoring

You are a member of squadron "%s". Auto-merge is enabled for this squadron.

After completing your primary task, announce completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary of what you did>"

CRITICAL: Do NOT stop after completing your work. The merge master will merge all agents' branches after everyone is done. You MUST continue polling the squadron channel every 30 seconds until the merge master posts MERGE_COMPLETE or MERGE_FAILED. Only then may you stop. If you are idle waiting, periodically run: fleet context channel-read squadron-%s
`

// BuildNoConsensusAutoMergeSuffix returns a minimal polling suffix for non-merger
// agents when consensus is "none" but auto-merge is enabled. These agents need
// to stay alive to monitor for MERGE_COMPLETE/MERGE_FAILED.
func BuildNoConsensusAutoMergeSuffix(squadronName string) string {
	return fmt.Sprintf(noConsensusAutoMergeTemplate, squadronName, squadronName, squadronName)
}

// BuildMergerSuffix returns the merger-duties suffix appended to the merge
// master's prompt. Pass every agent in the squadron (including the merger
// itself) in the order they should be merged.
func BuildMergerSuffix(squadronName, baseBranch string, agents []AgentBranch) string {
	var lines []string
	for _, ab := range agents {
		lines = append(lines, fmt.Sprintf("%s -> %s", ab.Name, ab.Branch))
	}
	list := strings.Join(lines, "\n")

	return fmt.Sprintf(
		mergerTemplate,
		squadronName,
		baseBranch, squadronName,
		squadronName, squadronName,
		squadronName,
		list,
	)
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
// BuildReviewMasterReviewerSuffix for that case.
func BuildConsensusSuffix(consensusType, squadronName string, agents []string, reviewMaster, baseBranch string) string {
	switch consensusType {
	case "none":
		return ""
	case "universal":
		return fmt.Sprintf(
			universalTemplate,
			squadronName, squadronName, squadronName, squadronName,
			baseBranch,
			squadronName, squadronName, squadronName,
			strings.Join(agents, ", "),
		)
	case "review_master":
		return fmt.Sprintf(
			reviewMasterNonReviewerTemplate,
			squadronName, squadronName, reviewMaster,
			squadronName, squadronName, squadronName,
			squadronName,
			strings.Join(agents, ", "),
			reviewMaster,
		)
	}
	return ""
}
