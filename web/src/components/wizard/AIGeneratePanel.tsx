import { useState } from "react";
import type { SquadronAgent } from "../../types";
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

interface AIGeneratePanelProps {
  squadronName: string;
  onAgentsGenerated: (agents: SquadronAgent[]) => void;
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

export function AIGeneratePanel({ squadronName, onAgentsGenerated }: AIGeneratePanelProps) {
  const [description, setDescription] = useState("");
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);

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
      onAgentsGenerated(newAgents);
    } catch (err) {
      setGenError(err instanceof Error ? err.message : "Generation failed");
    } finally {
      setGenerating(false);
    }
  };

  return (
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
      <style>{spinnerKeyframes}</style>
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
  );
}
