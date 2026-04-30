import { useState, useEffect } from "react";
import type { SquadronAgent } from "../../types";
import { generateAgents } from "../../api";
import { CodeEditor } from "../common/CodeEditor";
import { sanitizeAgentName } from "../../utils/agentName";

const loadingMessages = [
  "Compilerizing",
  "Turning it off and on again",
  "On my way!",
  "Generatin'",
  "Still workin'",
  "Thank you for your patience",
  "Dodging car extended warranty calls",
  "You look lovely today!",
  "Hydrating",
  "Your prompts are being generated n stuff",
];

const loadingKeyframes = `
@keyframes fc-spin { to { transform: rotate(360deg); } }
@keyframes fc-wave {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}
`;

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

function WavyText({ text }: { text: string }) {
  return (
    <span aria-label={text} style={{ display: "inline-flex" }}>
      {text.split("").map((char, i) => (
        <span
          key={`${text}-${i}`}
          style={{
            display: "inline-block",
            animation: "fc-wave 0.8s ease-in-out infinite",
            animationDelay: `${i * 0.04}s`,
            whiteSpace: char === " " ? "pre" : undefined,
          }}
        >
          {char}
        </span>
      ))}
    </span>
  );
}

function RotatingLoadingText() {
  const [index, setIndex] = useState(
    () => Math.floor(Math.random() * loadingMessages.length)
  );

  useEffect(() => {
    const interval = setInterval(() => {
      setIndex((prev) => {
        let next: number;
        do {
          next = Math.floor(Math.random() * loadingMessages.length);
        } while (next === prev && loadingMessages.length > 1);
        return next;
      });
    }, 2500);
    return () => clearInterval(interval);
  }, []);

  return <WavyText text={loadingMessages[index]} />;
}

interface AIGeneratePanelProps {
  squadronName: string;
  onAgentsGenerated: (agents: SquadronAgent[]) => void;
}

export function AIGeneratePanel({ squadronName, onAgentsGenerated }: AIGeneratePanelProps) {
  const [description, setDescription] = useState("");
  const [generating, setGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);

  const handleGenerate = async () => {
    setGenerating(true);
    setGenError(null);
    try {
      const result = await generateAgents(description);
      const newAgents = result.agents.map((a) => {
        const name = sanitizeAgentName(a.name);
        return {
          ...a,
          name,
          branch: a.branch || `squadron/${squadronName}/${name}`,
          driver: a.driver || "claude-code",
          persona: a.persona || "",
        };
      });
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
      <style>{loadingKeyframes}</style>
      <div id="ai-generate-heading" style={{ fontWeight: 600, color: "var(--purple)", marginBottom: "0.75rem" }}>
        AI Generate from Description
      </div>
      <label id="ai-description-label" className="sr-only">Task description for AI generation</label>
      <div style={{ marginBottom: "1rem" }}>
        <CodeEditor
          labelId="ai-description-label"
          value={description}
          onChange={setDescription}
          placeholder={"List out all of the tasks you need to get done.\n\nEach task will be given their own agent. 1 agent, 1 task\n\nUse a numbered list for best results"}
          minHeight={300}
        />
      </div>
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
        {generating ? <><Spinner /><RotatingLoadingText /></> : "Generate Agent Breakdown"}
      </button>
      {genError && (
        <div role="alert" style={{ color: "var(--red)", fontSize: "0.8rem", marginTop: "0.5rem" }}>
          {genError}
        </div>
      )}
    </section>
  );
}
