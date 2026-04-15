interface AgentPillProps {
  name: string;
  state: string;
  driver: string;
  onClick: () => void;
}

const driverAbbrev: Record<string, string> = {
  "claude-code": "cc",
  codex: "cx",
  aider: "ai",
  generic: "gn",
};

const driverColors: Record<string, string> = {
  "claude-code": "var(--blue)",
  codex: "var(--green)",
  aider: "var(--orange)",
  generic: "var(--text-secondary)",
};

const stateColors: Record<string, string> = {
  working: "var(--green)",
  waiting: "var(--orange)",
  stopped: "var(--red)",
  starting: "var(--text-muted)",
};

export function AgentPill({ name, state, driver, onClick }: AgentPillProps) {
  const dotColor = stateColors[state] || stateColors.starting;
  const isWaiting = state === "waiting";

  return (
    <button
      onClick={onClick}
      aria-label={`${name}, status: ${state}, harness: ${driver}`}
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
          animation: isWaiting ? "pulse 2s infinite" : undefined,
        }}
      />
      <span>{name}</span>
      <span style={{ fontSize: "0.65rem", color: driverColors[driver] || "var(--text-secondary)" }} aria-hidden="true">
        {driverAbbrev[driver] || driver.slice(0, 2)}
      </span>
    </button>
  );
}
