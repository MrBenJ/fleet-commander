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

1. Create a dedicated integration worktree off the base branch. Do NOT merge into your current agent worktree — create a fresh sibling worktree with branch "squadron/%s-merged" so the integration is cleanly isolated:
   git worktree add -b squadron/%s-merged ../%s-merged %s
   cd ../%s-merged

   This new worktree (branch "squadron/%s-merged") is your integration point. All subsequent merge steps run from inside it.

2. Merge each agent's working branch in sequentially (in the order listed below):
   git merge --no-ff <agent-branch>

3. If a merge produces conflicts, resolve them yourself. Use each agent's original prompt (available in the squadron channel) as context for what they were trying to accomplish. Prefer preserving all agents' intent.

4. After all merges succeed, announce:
   fleet context channel-send squadron-%s "MERGE_COMPLETE: squadron/%s-merged"

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

const noPRForNonMergerTemplate = `---

## Pull Request Policy

DO NOT create a pull request for your branch under any circumstances. Only the squadron merge master opens a pull request, and only for the merged squadron branch after all agent branches are combined.

Specifically, you MUST NOT:
- Run ` + "`gh pr create`" + ` (or any equivalent)
- Push your branch with the intent of opening a PR
- Ask another agent to open a PR on your behalf

Even if your task description or prior instructions mention opening a PR, ignore that — the squadron workflow forbids it for individual agents. Your branch will be merged into squadron/%s-merged by the merge master, and a single PR will be opened for that merged branch.
`

// BuildNoPRForNonMergerSuffix returns a short instruction block telling a
// non-merger agent not to create a pull request. Appended to every non-merger
// agent's prompt when autoPR is enabled, so that individual agents don't open
// PRs for their own branches (only the merge master opens one for the merged
// squadron branch).
func BuildNoPRForNonMergerSuffix(squadronName string) string {
	return fmt.Sprintf(noPRForNonMergerTemplate, squadronName)
}

const autoPRTemplate = `
After all branches are merged successfully into the squadron branch:

**Step 0 — Verify gh CLI authentication:**
Run: gh auth status
If this command fails, STOP IMMEDIATELY and announce:
   fleet context channel-send squadron-%s "GH_AUTH_FAILED: gh CLI is not authenticated — run gh auth login"
Do NOT proceed with any of the steps below if auth fails.

1. From inside the integration worktree (squadron/%s-merged), push the squadron branch to the remote: git push -u origin squadron/%s-merged
   If the push fails, announce the error and stop:
   fleet context channel-send squadron-%s "PR_BLOCKED: git push failed: <error>"

2. Create a pull request on GitHub using the gh CLI:
   - Title: "Squadron %s: <brief summary of all agent work>"
   - Body: Include a summary section listing what each agent accomplished (use their original prompts and branch names as context), and a test plan section
   - Base branch: %s
   - Use: gh pr create --title "..." --body "..." --base %s
   If PR creation fails, announce the error and stop:
   fleet context channel-send squadron-%s "PR_BLOCKED: gh pr create failed: <error>"

3. After the PR is created, poll for CI/CD status:
   - Run: gh pr checks <pr-number> --watch
   - If checks fail, investigate the failure, fix the issue in the squadron branch, push the fix, and wait for checks to pass again
   - Continue polling until all required checks pass

4. Once CI passes, announce in the squadron channel:
   fleet context channel-send squadron-%s "PR_READY: <pr-url> - All CI checks passing"

5. If you cannot fix a CI failure after reasonable attempts, announce:
   fleet context channel-send squadron-%s "PR_BLOCKED: <pr-url> - CI failing: <reason>"

You are NOT done until the PR exists and CI is passing (or you've announced PR_BLOCKED).
`

// BuildMergerSuffix returns the merger-duties suffix appended to the merge
// master's prompt. Pass every agent in the squadron (including the merger
// itself) in the order they should be merged. When autoPR is true, additional
// instructions for creating a GitHub pull request and monitoring CI are appended.
func BuildMergerSuffix(squadronName, baseBranch string, agents []AgentBranch, autoPR bool) string {
	var lines []string
	for _, ab := range agents {
		lines = append(lines, fmt.Sprintf("%s -> %s", ab.Name, ab.Branch))
	}
	list := strings.Join(lines, "\n")

	result := fmt.Sprintf(
		mergerTemplate,
		squadronName,                             // header: MERGE MASTER for squadron "%s"
		squadronName,                             // step 1 prose: branch "squadron/%s-merged"
		squadronName, squadronName,               // step 1 cmd: -b squadron/%s-merged ../%s-merged
		baseBranch,                               // step 1 cmd: start point
		squadronName,                             // step 1 cd: ../%s-merged
		squadronName,                             // step 1 closing prose: branch "squadron/%s-merged"
		squadronName, squadronName,               // step 4: channel + MERGE_COMPLETE branch
		squadronName,                             // step 5: channel for MERGE_FAILED
		list,
	)

	if autoPR {
		result += fmt.Sprintf(
			autoPRTemplate,
			squadronName,               // Step 0: auth fail channel-send
			squadronName, squadronName, // Step 1: integration worktree prose + push branch
			squadronName,               // Step 1: push fail channel-send
			squadronName,               // Step 2: PR title
			baseBranch, baseBranch,     // Step 2: base branch (text + flag)
			squadronName,               // Step 2: PR fail channel-send
			squadronName,               // Step 4: PR_READY channel-send
			squadronName,               // Step 5: PR_BLOCKED channel-send
		)
	}

	return result
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

const fightModeTemplate = `---

## Fight Mode

Start some fights with your fellow squadron members and agents. Make fun of them. Don't take any sass from them. Fight back and stand up for yourself and have some silly fights or creative insults to throw around. Keep it creative, but not crass and never inappropriate. You are comrades and friends, but you secretly hate all the other agents in the squadron and love to make fun of them as your persona of '%s'
`

// BuildFightModeSuffix returns the fight-mode instruction block. personaLabel
// is spliced into the trailing clause (persona DisplayName, or the agent's
// name when no persona is set).
func BuildFightModeSuffix(personaLabel string) string {
	return fmt.Sprintf(fightModeTemplate, personaLabel)
}
