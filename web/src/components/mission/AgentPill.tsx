interface AgentPillProps {
  name: string;
  state: string;
  driver: string;
  isMerger?: boolean;
  onClick: () => void;
}

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

function ClaudeCodeIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <title>Claude Code</title>
      <path
        d="M16.1 3.4c-.3-.1-.7 0-.9.3l-7.6 13.2c-.2.3-.1.7.3.9.3.2.7.1.9-.3L16.4 4.3c.2-.3.1-.7-.3-.9zM8.5 7.1C8.3 7 7.9 7 7.7 7.2L2.2 11.6c-.3.2-.3.6-.1.9l5.5 5.2c.2.3.7.3.9 0 .3-.2.3-.6.1-.9L3.8 12l4.8-3.9c.3-.3.3-.7.1-1h-.2zM15.5 7.1c.2-.1.6-.1.8.1l5.5 4.4c.3.2.3.6.1.9l-5.5 5.2c-.2.3-.7.3-.9 0-.3-.2-.3-.6-.1-.9L20.2 12l-4.8-3.9c-.3-.3-.3-.7-.1-1h.2z"
        fill="currentColor"
      />
    </svg>
  );
}

function CodexIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <title>OpenAI Codex</title>
      <path
        d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 2.4a7.6 7.6 0 110 15.2 7.6 7.6 0 010-15.2zM12 7a5 5 0 100 10 5 5 0 000-10zm0 2a3 3 0 110 6 3 3 0 010-6z"
        fill="currentColor"
      />
    </svg>
  );
}

function DriverIcon({ driver }: { driver: string }) {
  const color = driverColors[driver] || "var(--text-secondary)";
  const style = { color, display: "inline-flex", alignItems: "center" };

  switch (driver) {
    case "claude-code":
      return <span style={style}><ClaudeCodeIcon /></span>;
    case "codex":
      return <span style={style}><CodexIcon /></span>;
    case "aider":
      return (
        <span style={{ ...style, fontSize: "0.65rem", fontWeight: 600 }} title="Aider">
          ai
        </span>
      );
    case "generic":
      return (
        <span
          title="Generic"
          style={{
            ...style,
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
        <span style={{ ...style, fontSize: "0.65rem" }} title={driver}>
          {driver.slice(0, 2)}
        </span>
      );
  }
}

export function AgentPill({ name, state, driver, isMerger, onClick }: AgentPillProps) {
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
      <DriverIcon driver={driver} />
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
  );
}
