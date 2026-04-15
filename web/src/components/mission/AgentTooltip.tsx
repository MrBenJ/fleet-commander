import type { Persona, SquadronAgent } from "../../types";
import { stopAgent } from "../../api";
import { useState, useEffect, useRef, useCallback } from "react";

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
  const dialogRef = useRef<HTMLDivElement>(null);

  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === "Escape") {
      onClose();
      return;
    }

    if (e.key === "Tab" && dialogRef.current) {
      const focusable = dialogRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];

      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
  }, [onClose]);

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown);
    const firstButton = dialogRef.current?.querySelector<HTMLElement>("button");
    firstButton?.focus();

    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);

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
        background: "rgba(0,0,0,0.5)",
      }}
      onClick={onClose}
      role="presentation"
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={`Agent details: ${agent.name}`}
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
          <div style={{ fontSize: "2rem" }} aria-hidden="true">
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
              aria-hidden="true"
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

        <dl
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "0.75rem",
            fontSize: "0.85rem",
            marginBottom: "1.25rem",
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
          <div role="alert" style={{ color: "var(--red)", fontSize: "0.8rem", marginBottom: "0.75rem" }}>
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
            aria-disabled={stopping}
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
            {stopping ? "Stopping..." : "Stop"}
          </button>
        </div>
      </div>
    </div>
  );
}
