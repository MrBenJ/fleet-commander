package squadron

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

func RunHeadless(f *fleet.Fleet, data *SquadronData) error {
	if len(data.Agents) == 0 {
		return fmt.Errorf("no agents in squadron")
	}

	baseBranch := data.BaseBranch
	if baseBranch == "" {
		cb, err := f.CurrentBranch()
		if err != nil {
			return fmt.Errorf("could not resolve base branch: %w", err)
		}
		baseBranch = cb
	}

	mergeMaster := ""
	if data.AutoMerge {
		if data.MergeMaster != nil && *data.MergeMaster != "" {
			mergeMaster = *data.MergeMaster
		} else {
			mergeMaster = data.Agents[rand.Intn(len(data.Agents))].Name
		}
	}

	agentNames := make([]string, 0, len(data.Agents))
	agentBranches := make([]AgentBranch, 0, len(data.Agents))
	for _, a := range data.Agents {
		agentNames = append(agentNames, a.Name)
		agentBranches = append(agentBranches, AgentBranch{Name: a.Name, Branch: a.Branch})
	}

	channelName := "squadron-" + data.Name
	description := fmt.Sprintf("Squadron %s (%s)", data.Name, data.Consensus)
	if _, err := fleetctx.CreateChannel(f.FleetDir, channelName, description, agentNames); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create squadron channel %q: %w", channelName, err)
		}
	}

	sysPrompt, _ := fleet.LoadSystemPrompt(f.FleetDir)
	if data.UseJumpSh {
		sysPrompt += "\n\nYour workbranch will be able to be accessed via a local dev instance via a tool called 'https://jump.sh/' - Jump SH. Fetch this web URL to see what it does and how it works. Upon initialization, use this locally hosted web server as a way to access a local development environment for yourself"
	}

	promptsDir := filepath.Join(f.FleetDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("create prompts dir: %w", err)
	}

	tm := tmux.NewManager(f.TmuxPrefix())
	statesDir := filepath.Join(f.FleetDir, "states")
	_ = os.MkdirAll(statesDir, 0755)

	var launched []string

	for _, a := range data.Agents {
		agent, err := f.AddAgent(a.Name, a.Branch)
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("add agent %q: %w", a.Name, err)
			}
			agent, err = f.GetAgent(a.Name)
			if err != nil {
				return fmt.Errorf("get agent %q: %w", a.Name, err)
			}
		}

		fullPrompt := buildHeadlessPrompt(sysPrompt, data.Agents, a)

		switch data.Consensus {
		case "universal":
			fullPrompt += "\n" + BuildConsensusSuffix("universal", data.Name, agentNames, "", baseBranch)
		case "review_master":
			if a.Name == data.ReviewMaster {
				fullPrompt += "\n" + BuildReviewMasterReviewerSuffix(data.Name, agentNames, baseBranch)
			} else {
				fullPrompt += "\n" + BuildConsensusSuffix("review_master", data.Name, agentNames, data.ReviewMaster, baseBranch)
			}
		}

		if data.AutoMerge && a.Name == mergeMaster {
			fullPrompt += "\n" + BuildMergerSuffix(data.Name, baseBranch, agentBranches)
		}

		if a.Persona != "" {
			if p, ok := LookupPersona(a.Persona); ok {
				fullPrompt = ApplyPersona(p, fullPrompt)
			}
		}

		promptFile := filepath.Join(promptsDir, agent.Name+".txt")
		if err := os.WriteFile(promptFile, []byte(fullPrompt), 0644); err != nil {
			return fmt.Errorf("write prompt file %s: %w", promptFile, err)
		}

		drv, err := driver.GetForAgent(agent)
		if err != nil {
			drv, _ = driver.Get(a.Driver)
		}
		stateFilePath := filepath.Join(statesDir, agent.Name+".json")
		if err := drv.InjectHooks(agent.WorktreePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: inject hooks for %q: %v\n", agent.Name, err)
			stateFilePath = ""
			f.UpdateAgentHooks(agent.Name, false)
		} else {
			f.UpdateAgentHooks(agent.Name, true)
		}

		launcherFile := filepath.Join(promptsDir, agent.Name+".sh")
		launcherScript := drv.BuildCommand(driver.LaunchOpts{
			YoloMode:   true,
			PromptFile: promptFile,
			AgentName:  agent.Name,
		})
		if err := os.WriteFile(launcherFile, []byte(launcherScript), 0755); err != nil {
			return fmt.Errorf("write launcher %s: %w", launcherFile, err)
		}

		if err := tm.CreateSession(agent.Name, agent.WorktreePath, []string{launcherFile}, stateFilePath); err != nil {
			return fmt.Errorf("create tmux session %q: %w", agent.Name, err)
		}
		f.UpdateAgentStateFile(agent.Name, stateFilePath)
		pid, _ := tm.GetPID(agent.Name)
		f.UpdateAgent(agent.Name, "running", pid)
		if a.Driver != "" && a.Driver != "claude-code" {
			f.UpdateAgentDriver(agent.Name, a.Driver)
		}
		launched = append(launched, agent.Name)
	}

	fmt.Fprintf(os.Stderr, "Squadron %q launched: %s\n", data.Name, strings.Join(launched, ", "))
	return nil
}

func buildHeadlessPrompt(systemPrompt string, all []SquadronAgent, current SquadronAgent) string {
	var b strings.Builder
	if strings.TrimSpace(systemPrompt) != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n")
	}
	b.WriteString("## Active Fleet Agents\n\n")
	b.WriteString(fmt.Sprintf("You are: %s (branch: %s)\n\n", current.Name, current.Branch))
	b.WriteString("| Agent | Branch | Task |\n")
	b.WriteString("|-------|--------|------|\n")
	for _, a := range all {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", a.Name, a.Branch, a.Prompt))
	}
	b.WriteString("\n---\n\n")
	b.WriteString(current.Prompt)
	return b.String()
}
