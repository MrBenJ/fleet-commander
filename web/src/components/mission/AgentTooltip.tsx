import type { Persona, SquadronAgent } from "../../types";
import { stopAgent } from "../../api";
import { useState } from "react";
import Markdown from "react-markdown";

interface AgentTooltipProps {
  agent: SquadronAgent;
  state: string;
  persona?: Persona;
}

const personaIcons: Record<string, string> = {
  "overconfident-engineer": "\u{1F680}",
  "zen-master": "\u{1F9D8}",
  "paranoid-perfectionist": "\u{1F50D}",
  "raging-jerk": "\u{1F624}",
  "peter-molyneux": "\u{1F3A9}",
};

const stateColors: Record<string, string> = {
  working: "var(--green)",
  waiting: "var(--orange)",
  stopped: "var(--red)",
  starting: "var(--text-muted)",
};

export function AgentTooltip({
  agent,
  state,
  persona,
}: AgentTooltipProps) {
  const [stopping, setStopping] = useState(false);
  const [stopError, setStopError] = useState<string | null>(null);

  const handleStop = async (e: React.MouseEvent) => {
    e.stopPropagation();
    if (!confirm(`Stop ${agent.name}?`)) return;
    setStopping(true);
    setStopError(null);
    try {
      await stopAgent(agent.name);
    } catch (err) {
      setStopError(err instanceof Error ? err.message : "Failed to stop agent");
    } finally {
      setStopping(false);
    }
  };

  const handleAssumeControl = (e: React.MouseEvent) => {
    e.stopPropagation();
    window.open(`/terminal/${agent.name}`, "_blank", "width=900,height=600");
  };

  return (
    <div
      style={{
        position: "absolute",
        top: "100%",
        left: "50%",
        transform: "translateX(-50%)",
        paddingTop: 8,
        zIndex: 200,
      }}
    >
    <div
      style={{
        background: "var(--bg-tertiary)",
        border: "1px solid var(--border)",
        borderRadius: 12,
        padding: "1.25rem",
        width: 480,
        maxHeight: "80vh",
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        boxShadow: "0 8px 24px rgba(0,0,0,0.4)",
      }}
      onClick={(e) => e.stopPropagation()}
    >
      {/* Header */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "0.75rem",
          marginBottom: "1rem",
          paddingBottom: "0.75rem",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <div style={{ fontSize: "1.75rem", lineHeight: 1 }} aria-hidden="true">
          {personaIcons[agent.persona] || "\u{1F916}"}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 700, fontSize: "1.05rem" }}>{agent.name}</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>
            {persona?.displayName || "No Persona"}
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "0.4rem" }}>
          <span
            aria-hidden="true"
            style={{
              width: 9,
              height: 9,
              borderRadius: "50%",
              background: stateColors[state] || stateColors.starting,
              display: "inline-block",
              flexShrink: 0,
            }}
          />
          <span
            style={{
              fontSize: "0.8rem",
              color: stateColors[state] || stateColors.starting,
              fontWeight: 600,
            }}
          >
            {state}
          </span>
        </div>
      </div>

      {/* Details */}
      <dl
        style={{
          display: "flex",
          flexDirection: "column",
          gap: "0.5rem",
          fontSize: "0.8rem",
          margin: 0,
          paddingBottom: "1rem",
        }}
      >
        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <dt style={{ color: "var(--text-secondary)" }}>Branch</dt>
          <dd style={{ margin: 0 }}><code>{agent.branch}</code></dd>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between" }}>
          <dt style={{ color: "var(--text-secondary)" }}>Harness</dt>
          <dd style={{ margin: 0, color: "var(--blue)" }}>{agent.driver}</dd>
        </div>
      </dl>

      {/* Task */}
      <div style={{ marginBottom: "1rem" }}>
        <div style={{ color: "var(--text-primary)", fontWeight: 600, fontSize: "0.8rem", marginBottom: "0.4rem" }}>
          Task
        </div>
        <div
          className="tooltip-markdown"
          style={{
            margin: 0,
            padding: "0.5rem 0.75rem",
            background: "var(--bg-primary)",
            border: "1px solid var(--border)",
            borderRadius: 8,
            fontSize: "0.8rem",
            lineHeight: 1.5,
            wordBreak: "break-word",
            maxHeight: "40vh",
            overflowY: "auto",
            color: "var(--text-primary)",
          }}
        >
          <Markdown>{agent.prompt.replace(/<!--/g, "\\<!--")}</Markdown>
        </div>
      </div>

      {/* Error */}
      {stopError && (
        <div role="alert" style={{ color: "var(--red)", fontSize: "0.75rem", marginBottom: "0.5rem" }}>
          {stopError}
        </div>
      )}

      {/* Actions */}
      <div style={{ display: "flex", gap: "0.5rem" }}>
        <button
          onClick={handleAssumeControl}
          style={{
            flex: 1,
            background: "#1f6feb",
            color: "#fff",
            border: "none",
            borderRadius: 8,
            padding: "0.55rem",
            fontSize: "0.8rem",
            fontWeight: 600,
            cursor: "pointer",
          }}
        >
          Assume Control
        </button>
        <button
          onClick={handleStop}
          disabled={stopping}
          aria-disabled={stopping}
          style={{
            background: "var(--bg-tertiary)",
            border: "1px solid var(--border)",
            color: "var(--red)",
            borderRadius: 8,
            padding: "0.55rem 0.85rem",
            fontSize: "0.8rem",
            cursor: stopping ? "wait" : "pointer",
          }}
        >
          {stopping ? "Stopping..." : "Stop"}
        </button>
      </div>
    </div>
    </div>
  );
}
