import { useState } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { CodeEditor } from "../common/CodeEditor";
import { HelpTooltip } from "../common/HelpTooltip";

interface ManualAddFormProps {
  squadronName: string;
  drivers: string[];
  personas: Persona[];
  onAgentAdded: (agent: SquadronAgent) => void;
}

const inputStyle: React.CSSProperties = {
  background: "var(--bg-primary)",
  border: "1px solid var(--border)",
  borderRadius: 4,
  padding: "0.5rem",
  color: "var(--text-primary)",
  width: "100%",
  fontSize: "0.85rem",
};

const labelStyle: React.CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.7rem",
  textTransform: "uppercase" as const,
  marginBottom: "0.25rem",
  display: "block",
};

export function ManualAddForm({ squadronName, drivers, personas, onAgentAdded }: ManualAddFormProps) {
  const [name, setName] = useState("");
  const [branch, setBranch] = useState("");
  const [prompt, setPrompt] = useState("");
  const [selectedDriver, setSelectedDriver] = useState(drivers[0] || "claude-code");
  const [manualPersona, setManualPersona] = useState("");

  const handleAddManual = () => {
    if (!name || !prompt) return;
    const agentBranch = branch || `squadron/${squadronName}/${name}`;
    onAgentAdded({ name, branch: agentBranch, prompt, driver: selectedDriver, persona: manualPersona });
    setName("");
    setBranch("");
    setPrompt("");
    setManualPersona("");
  };

  return (
    <section
      aria-labelledby="manual-add-heading"
      style={{
        flex: 1,
        border: "1px solid var(--border)",
        borderRadius: 8,
        padding: "1.5rem",
        background: "var(--bg-secondary)",
      }}
    >
      <div id="manual-add-heading" style={{ fontWeight: 600, color: "var(--blue)", marginBottom: "0.75rem" }}>
        Add Manually
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
        <div>
          <label htmlFor="manual-agent-name" style={labelStyle}>
            Agent Name
            <HelpTooltip text="A short identifier for this agent. Used in branch names and tmux session names." />
          </label>
          <input id="manual-agent-name" style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} placeholder="auth-agent" />
        </div>
        <div>
          <label htmlFor="manual-branch" style={labelStyle}>
            Branch
            <HelpTooltip text="The git branch name for this agent's worktree. Defaults to squadron/<name>/<agent> if left empty." />
          </label>
          <input
            id="manual-branch"
            style={inputStyle}
            value={branch}
            onChange={(e) => setBranch(e.target.value)}
            placeholder={`squadron/${squadronName}/${name || "agent"}`}
          />
        </div>
        <div>
          <label id="manual-prompt-label" style={labelStyle}>Prompt</label>
          <CodeEditor
            labelId="manual-prompt-label"
            value={prompt}
            onChange={setPrompt}
            placeholder="What should this agent do?"
            minHeight={120}
          />
        </div>
        <div>
          <label htmlFor="manual-harness" style={labelStyle}>
            Harness
            <HelpTooltip text="The harness configures how the agent runs — including permissions, tools, and execution constraints." />
          </label>
          <select
            id="manual-harness"
            style={inputStyle}
            value={selectedDriver}
            onChange={(e) => setSelectedDriver(e.target.value)}
          >
            {drivers.map((d) => (
              <option key={d} value={d}>{d}</option>
            ))}
          </select>
        </div>
        <div>
          <label htmlFor="manual-persona" style={labelStyle}>
            Persona
            <HelpTooltip text="A persona defines the agent's coding style, expertise areas, and approach to problem-solving." />
          </label>
          <select
            id="manual-persona"
            style={inputStyle}
            value={manualPersona}
            onChange={(e) => setManualPersona(e.target.value)}
          >
            <option value="">No persona</option>
            {personas.map((p) => (
              <option key={p.name} value={p.name}>{p.displayName}</option>
            ))}
          </select>
        </div>
        <button
          onClick={handleAddManual}
          disabled={!name || !prompt}
          aria-disabled={!name || !prompt}
          style={{
            background: "var(--bg-tertiary)",
            color: "var(--text-primary)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "0.5rem 1rem",
            cursor: !name || !prompt ? "default" : "pointer",
            opacity: !name || !prompt ? 0.5 : 1,
          }}
        >
          + Add Agent
        </button>
      </div>
    </section>
  );
}
