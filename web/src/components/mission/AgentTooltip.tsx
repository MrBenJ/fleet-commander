import type { Persona, SquadronAgent } from "../../types";
import { stopAgent } from "../../api";
import { useState } from "react";

interface AgentTooltipProps {
  agent: SquadronAgent;
  state: string;
  persona?: Persona;
  onClose: () => void;
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
  onClose,
}: AgentTooltipProps) {
  const [stopping, setStopping] = useState(false);
  const [stopError, setStopError] = useState<string | null>(null);

  const handleStop = async () => {
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

  const handleAssumeControl = () => {
    window.open(`/terminal/${agent.name}`, "_blank", "width=900,height=600");
  };

  return (
    <div
      style={{
        position: "fixed",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        zIndex: 100,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      }}
      onClick={onClose}
    >
      <div
        style={{
          background: "var(--bg-tertiary)",
          border: "1px solid var(--border)",
          borderRadius: 12,
          padding: "1.75rem",
          width: 380,
          boxShadow: "0 8px 24px rgba(0,0,0,0.4)",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "1rem",
            marginBottom: "1.25rem",
            paddingBottom: "1rem",
            borderBottom: "1px solid var(--border)",
          }}
        >
          <div style={{ fontSize: "2rem" }}>
            {personaIcons[agent.persona] || "\u{1F916}"}
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontWeight: 700, fontSize: "1.15rem" }}>{agent.name}</div>
            <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>
              {persona?.displayName || "No Persona"}
            </div>
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: "0.5rem" }}>
            <span
              style={{
                width: 10,
                height: 10,
                borderRadius: "50%",
                background: stateColors[state] || stateColors.starting,
                display: "inline-block",
              }}
            />
            <span
              style={{
                fontSize: "0.85rem",
                color: stateColors[state] || stateColors.starting,
                fontWeight: 600,
              }}
            >
              {state}
            </span>
          </div>
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "0.75rem",
            fontSize: "0.85rem",
            marginBottom: "1.25rem",
          }}
        >
          <div style={{ display: "flex", justifyContent: "space-between" }}>
            <span style={{ color: "var(--text-secondary)" }}>Branch</span>
            <code>{agent.branch}</code>
          </div>
          <div style={{ display: "flex", justifyContent: "space-between" }}>
            <span style={{ color: "var(--text-secondary)" }}>Harness</span>
            <span style={{ color: "var(--blue)" }}>{agent.driver}</span>
          </div>
        </div>

        <div
          style={{
            fontSize: "0.8rem",
            color: "var(--text-secondary)",
            marginBottom: "1.25rem",
            padding: "0.85rem",
            background: "var(--bg-secondary)",
            borderRadius: 8,
            lineHeight: 1.5,
            border: "1px solid var(--border)",
          }}
        >
          <div style={{ color: "var(--text-primary)", fontWeight: 600, marginBottom: "0.4rem" }}>
            Task
          </div>
          {agent.prompt.slice(0, 200)}{agent.prompt.length > 200 ? "..." : ""}
        </div>

        {stopError && (
          <div style={{ color: "var(--red)", fontSize: "0.8rem", marginBottom: "0.75rem" }}>
            {stopError}
          </div>
        )}

        <div style={{ display: "flex", gap: "0.6rem" }}>
          <button
            onClick={handleAssumeControl}
            style={{
              flex: 1,
              background: "#1f6feb",
              color: "#fff",
              border: "none",
              borderRadius: 8,
              padding: "0.65rem",
              fontSize: "0.85rem",
              fontWeight: 600,
              cursor: "pointer",
            }}
          >
            Assume Control
          </button>
          <button
            onClick={handleStop}
            disabled={stopping}
            style={{
              background: "var(--bg-tertiary)",
              border: "1px solid var(--border)",
              color: "var(--red)",
              borderRadius: 8,
              padding: "0.65rem 1rem",
              fontSize: "0.85rem",
              cursor: stopping ? "wait" : "pointer",
            }}
          >
            Stop
          </button>
        </div>
      </div>
    </div>
  );
}
