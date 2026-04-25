import { useState, useRef } from "react";
import type { Persona, SquadronAgent } from "../../types";
import { AgentTooltip } from "./AgentTooltip";
import { ClaudeCodeIcon } from "../icons/ClaudeCodeIcon";
import { CodexIcon } from "../icons/CodexIcon";
import { AiderIcon } from "../icons/AiderIcon";

interface AgentPillProps {
  agent: SquadronAgent;
  state: string;
  persona?: Persona;
  isMerger?: boolean;
}

const stateColors: Record<string, string> = {
  working: "var(--green)",
  waiting: "var(--orange)",
  stopped: "var(--red)",
  starting: "var(--text-muted)",
};

export function DriverIcon({ driver, size = 14 }: { driver: string; size?: number }) {
  const style = { display: "inline-flex", alignItems: "center" };

  switch (driver) {
    case "claude-code":
      return <span style={style}><ClaudeCodeIcon size={size} /></span>;
    case "codex":
      return <span style={style}><CodexIcon size={size} /></span>;
    case "aider":
      return <span style={style}><AiderIcon size={size} /></span>;
    case "kimi-code":
      return (
        <span
          title="Kimi Code"
          style={{
            ...style,
            color: "#a78bfa",
            fontSize: "0.6rem",
            fontWeight: 700,
            background: "rgba(167,139,250,0.15)",
            borderRadius: 4,
            width: 16,
            height: 16,
            justifyContent: "center",
          }}
        >
          K
        </span>
      );
    case "generic":
      return (
        <span
          title="Generic"
          style={{
            ...style,
            color: "var(--text-secondary)",
            fontSize: "0.6rem",
            fontWeight: 700,
            background: "var(--bg-tertiary, rgba(255,255,255,0.08))",
            borderRadius: 4,
            width: 16,
            height: 16,
            justifyContent: "center",
          }}
        >
          G
        </span>
      );
    default:
      return (
        <span style={{ ...style, color: "var(--text-secondary)", fontSize: "0.65rem" }} title={driver}>
          {driver.slice(0, 2)}
        </span>
      );
  }
}

export function AgentPill({ agent, state, persona, isMerger }: AgentPillProps) {
  const [showTooltip, setShowTooltip] = useState(false);
  const hideTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);

  const dotColor = stateColors[state] || stateColors.starting;
  const isWaiting = state === "waiting";

  const handleMouseEnter = () => {
    if (hideTimeout.current) {
      clearTimeout(hideTimeout.current);
      hideTimeout.current = null;
    }
    setShowTooltip(true);
  };

  const handleMouseLeave = () => {
    hideTimeout.current = setTimeout(() => {
      setShowTooltip(false);
    }, 200);
  };

  return (
    <div
      style={{ position: "relative", display: "inline-flex" }}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <button
        aria-label={`${agent.name}, status: ${state}, harness: ${agent.driver}`}
        style={{
          fontSize: "0.8rem",
          background: "var(--bg-secondary)",
          border: `1px solid ${isWaiting ? "var(--orange)" : "var(--border)"}`,
          borderRadius: 16,
          padding: "0.3rem 0.75rem",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          gap: "0.4rem",
          color: "var(--text-primary)",
        }}
      >
        <span
          aria-hidden="true"
          style={{
            width: 8,
            height: 8,
            borderRadius: "50%",
            background: dotColor,
            display: "inline-block",
            flexShrink: 0,
            animation: isWaiting ? "pulse 2s infinite" : undefined,
          }}
        />
        <span>{agent.name}</span>
        <DriverIcon driver={agent.driver} />
        {isMerger && (
          <span
            style={{
              fontSize: "0.55rem",
              fontWeight: 700,
              background: "#a855f7",
              color: "#fff",
              padding: "0.1rem 0.35rem",
              borderRadius: 8,
              letterSpacing: "0.03em",
              lineHeight: 1,
            }}
          >
            MERGE
          </span>
        )}
      </button>

      {showTooltip && (
        <AgentTooltip
          agent={agent}
          state={state}
          persona={persona}
        />
      )}
    </div>
  );
}
