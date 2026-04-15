import { useState } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { generateAgents } from "../../api";

const spinnerKeyframes = `@keyframes fc-spin { to { transform: rotate(360deg); } }`;

function Spinner() {
  return (
    <span
      role="status"
      aria-label="Generating agents"
      style={{
        display: "inline-block",
        width: 14,
        height: 14,
        border: "2px solid rgba(255,255,255,0.3)",
        borderTopColor: "#fff",
        borderRadius: "50%",
        animation: "fc-spin 0.6s linear infinite",
        verticalAlign: "middle",
        marginRight: 6,
      }}
    />
  );
}

interface AgentsStepProps {
  squadronName: string;
  agents: SquadronAgent[];
  drivers: string[];
  personas: Persona[];
  onDone: (agents: SquadronAgent[]) => void;
  onPickPersona: (idx: number, currentAgents: SquadronAgent[]) => void;
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

export function AgentsStep({
  squadronName,
  agents: initialAgents,
  drivers,
  personas,
  onDone,
  onPickPersona,
}: AgentsStepProps) {
  const [agents, setAgents] = useState<SquadronAgent[]>(initialAgents);
  const [description, setDescription] = useState("");
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);

  // Manual add form
  const [name, setName] = useState("");
  const [branch, setBranch] = useState("");
  const [prompt, setPrompt] = useState("");
  const [selectedDriver, setSelectedDriver] = useState(drivers[0] || "claude-code");
  const [manualPersona, setManualPersona] = useState("");

  const handleGenerate = async () => {
    setGenerating(true);
    setGenError(null);
    try {
      const result = await generateAgents(description);
      const newAgents = result.agents.map((a) => ({
        ...a,
        branch: a.branch || `squadron/${squadronName}/${a.name}`,
        driver: a.driver || "claude-code",
        persona: a.persona || "",
      }));
      setAgents((prev) => [...prev, ...newAgents]);
    } catch (err) {
      setGenError(err instanceof Error ? err.message : "Generation failed");
    } finally {
      setGenerating(false);
    }
  };

  const handleAddManual = () => {
    if (!name || !prompt) return;
    const agentBranch = branch || `squadron/${squadronName}/${name}`;
    setAgents((prev) => [
      ...prev,
      { name, branch: agentBranch, prompt, driver: selectedDriver, persona: manualPersona },
    ]);
    setName("");
    setBranch("");
    setPrompt("");
    setManualPersona("");
  };

  const handleRemove = (idx: number) => {
    setAgents((prev) => prev.filter((_, i) => i !== idx));
  };

  const personaIcons: Record<string, string> = {
    "overconfident-engineer": "\u{1F680}",
    "zen-master": "\u{1F9D8}",
    "paranoid-perfectionist": "\u{1F50D}",
    "raging-jerk": "\u{1F624}",
    "peter-molyneux": "\u{1F3A9}",
  };

  const getPersonaDisplay = (personaKey: string) => {
    if (!personaKey) return "No persona selected";
    const p = personas.find((x) => x.name === personaKey);
    const name = p?.displayName || personaKey;
    const icon = personaIcons[personaKey] || "";
    return `\u270F\uFE0F ${name} ${icon}`;
  };

  return (
    <div>
      <style>{spinnerKeyframes}</style>
      <h2 style={{ marginBottom: "0.5rem" }}>Add Agents</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Assemble your squad's agents
      </p>

      <div style={{ display: "flex", gap: "1.5rem", marginBottom: "2rem" }}>
        {/* AI Generate panel */}
        <section
          aria-labelledby="ai-generate-heading"
          style={{
            flex: 1,
            border: "1px solid var(--border)",
            borderRadius: 8,
            padding: "1.5rem",
            background: "var(--bg-secondary)",
          }}
        >
          <div id="ai-generate-heading" style={{ fontWeight: 600, color: "var(--purple)", marginBottom: "0.75rem" }}>
            AI Generate from Description
          </div>
          <label htmlFor="ai-description" className="sr-only">Task description for AI generation</label>
          <textarea
            id="ai-description"
            style={{
              ...inputStyle,
              minHeight: "75%",
              resize: "vertical",
              marginBottom: "1rem",
            }}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={"List out all of the tasks you need to get done.\n\nEach task will be given their own agent. 1 agent, 1 task\n\nUse a numbered list for best results"}
          />
          <button
            onClick={handleGenerate}
            disabled={generating || !description.trim()}
            aria-disabled={generating || !description.trim()}
            style={{
              background: "var(--green)",
              color: "#fff",
              border: "none",
              borderRadius: 6,
              padding: "0.5rem 1rem",
              width: "100%",
              cursor: generating ? "wait" : "pointer",
              opacity: generating || !description.trim() ? 0.6 : 1,
            }}
          >
            {generating ? <><Spinner />Generating...</> : "Generate Agent Breakdown"}
          </button>
          {genError && (
            <div role="alert" style={{ color: "var(--red)", fontSize: "0.8rem", marginTop: "0.5rem" }}>
              {genError}
            </div>
          )}
        </section>

        {/* Manual panel */}
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
              <label htmlFor="manual-agent-name" style={labelStyle}>Agent Name</label>
              <input id="manual-agent-name" style={inputStyle} value={name} onChange={(e) => setName(e.target.value)} placeholder="auth-agent" />
            </div>
            <div>
              <label htmlFor="manual-branch" style={labelStyle}>Branch</label>
              <input
                id="manual-branch"
                style={inputStyle}
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                placeholder={`squadron/${squadronName}/${name || "agent"}`}
              />
            </div>
            <div>
              <label htmlFor="manual-prompt" style={labelStyle}>Prompt</label>
              <textarea
                id="manual-prompt"
                style={{ ...inputStyle, minHeight: 60, resize: "vertical" }}
                value={prompt}
                onChange={(e) => setPrompt(e.target.value)}
                placeholder="What should this agent do?"
              />
            </div>
            <div>
              <label htmlFor="manual-harness" style={labelStyle}>Harness</label>
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
              <label htmlFor="manual-persona" style={labelStyle}>Persona</label>
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
      </div>

      {/* Agent list */}
      {agents.length > 0 && (
        <div style={{ marginBottom: "1.5rem" }}>
          <h3 style={{ marginBottom: "0.75rem" }}>Agents ({agents.length})</h3>
          <ul style={{ display: "flex", flexDirection: "column", gap: "0.5rem", listStyle: "none", padding: 0 }} aria-label="Agent list">
            {agents.map((a, idx) => (
              <li
                key={idx}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "1rem",
                  background: "var(--bg-secondary)",
                  border: "1px solid var(--border)",
                  borderRadius: 8,
                  padding: "0.75rem 1rem",
                }}
              >
                <div style={{ flex: 1 }}>
                  <strong>{a.name}</strong>
                  <span
                    style={{
                      fontSize: "0.7rem",
                      background: "rgba(31,111,235,0.2)",
                      color: "var(--blue)",
                      padding: "0.15rem 0.5rem",
                      borderRadius: 10,
                      marginLeft: "0.5rem",
                    }}
                  >
                    {a.driver}
                  </span>
                  <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 2 }}>
                    {(a.prompt || "").slice(0, 80)}{(a.prompt || "").length > 80 ? "..." : ""}
                  </div>
                </div>
                <button
                  onClick={() => onPickPersona(idx, agents)}
                  aria-label={`Select persona for ${a.name}`}
                  style={{
                    background: "none",
                    border: "1px solid var(--border)",
                    borderRadius: 6,
                    padding: "0.3rem 0.6rem",
                    color: "var(--text-secondary)",
                    fontSize: "0.75rem",
                    cursor: "pointer",
                  }}
                >
                  {getPersonaDisplay(a.persona)}
                </button>
                <button
                  onClick={() => handleRemove(idx)}
                  aria-label={`Remove agent ${a.name}`}
                  style={{
                    background: "none",
                    border: "none",
                    color: "var(--red)",
                    cursor: "pointer",
                    fontSize: "1rem",
                  }}
                >
                  X
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}

      <button
        onClick={() => onDone(agents)}
        disabled={agents.length === 0}
        aria-disabled={agents.length === 0}
        style={{
          background: agents.length > 0 ? "var(--green)" : "var(--bg-secondary)",
          color: agents.length > 0 ? "#fff" : "var(--text-muted)",
          border: "none",
          borderRadius: 8,
          padding: "0.75rem 2rem",
          fontSize: "1rem",
          fontWeight: 600,
          cursor: agents.length > 0 ? "pointer" : "default",
        }}
      >
        Continue to Review →
      </button>
    </div>
  );
}
