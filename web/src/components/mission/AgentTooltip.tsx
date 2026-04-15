import type { Persona, SquadronAgent } from "../../types";
import { stopAgent } from "../../api";
import { useState } from "react";

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
        top: "calc(100% + 8px)",
        left: "50%",
        transform: "translateX(-50%)",
        background: "var(--bg-tertiary)",
        border: "1px solid var(--border)",
        borderRadius: 12,
        padding: "1.25rem",
        width: 340,
        boxShadow: "0 8px 24px rgba(0,0,0,0.4)",
        zIndex: 200,
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
        <div style={{ fontSize: "1.75rem" }} aria-hidden="true">
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
          marginBottom: "1rem",
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

      {/* Task preview */}
      <div
        style={{
          fontSize: "0.75rem",
          color: "var(--text-secondary)",
          marginBottom: "1rem",
          padding: "0.65rem",
          background: "var(--bg-secondary)",
          borderRadius: 8,
          lineHeight: 1.5,
          border: "1px solid var(--border)",
        }}
      >
        <div style={{ color: "var(--text-primary)", fontWeight: 600, marginBottom: "0.3rem" }}>
          Task
        </div>
        {agent.prompt.slice(0, 200)}{agent.prompt.length > 200 ? "..." : ""}
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
  );
}
