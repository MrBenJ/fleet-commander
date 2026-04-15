import { useEffect, useRef } from "react";
import type { ContextMessage } from "../../types";

interface ContextLogProps {
  messages: ContextMessage[];
  agentColors: Record<string, string>;
  onAgentClick: (name: string) => void;
}

const defaultColors = [
  "var(--blue)",
  "var(--green)",
  "var(--purple)",
  "var(--orange)",
  "#f778ba",
  "#79c0ff",
];

export function ContextLog({ messages, agentColors, onAgentClick }: ContextLogProps) {
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  const getColor = (agent: string) =>
    agentColors[agent] || defaultColors[Math.abs(hashCode(agent)) % defaultColors.length];

  const handleAgentKeyDown = (e: React.KeyboardEvent, name: string) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onAgentClick(name);
    }
  };

  return (
    <div
      role="log"
      aria-label="Squadron context messages"
      aria-live="polite"
      style={{
        flex: 1,
        overflowY: "auto",
        padding: "1rem 1.5rem",
        fontFamily: "'SF Mono', 'Fira Code', monospace",
        fontSize: "0.82rem",
        lineHeight: 1.9,
      }}
    >
      {messages.map((msg, idx) => {
        const time = new Date(msg.timestamp);
        const timeStr = `${time.getHours().toString().padStart(2, "0")}:${time.getMinutes().toString().padStart(2, "0")}`;
        const color = getColor(msg.agent);

        return (
          <div key={idx} style={{
            display: "flex",
            gap: "0.75rem",
            padding: "0.25rem 0.5rem",
            background: idx % 2 === 0 ? "rgba(255,255,255,0.03)" : "transparent",
            borderRadius: idx % 2 === 0 ? 4 : 0,
          }}>
            <span style={{ color: "var(--text-muted)", minWidth: 55 }}><time>{timeStr}</time></span>
            <span
              role="button"
              tabIndex={0}
              onClick={() => onAgentClick(msg.agent)}
              onKeyDown={(e) => handleAgentKeyDown(e, msg.agent)}
              style={{
                color,
                fontWeight: 600,
                cursor: "pointer",
                minWidth: 100,
              }}
            >
              {msg.agent}
            </span>
            <span style={{ color: "var(--text-secondary)" }}>{msg.message}</span>
          </div>
        );
      })}

      {messages.length > 0 && (
        <div style={{ display: "flex", justifyContent: "center", padding: "1rem 0 0.5rem" }}>
          <span
            aria-hidden="true"
            style={{
              fontSize: "0.75rem",
              color: "var(--text-muted)",
              background: "var(--bg-secondary)",
              padding: "0.3rem 0.8rem",
              borderRadius: 10,
            }}
          >
            ● live
          </span>
        </div>
      )}

      <div ref={bottomRef} />
    </div>
  );
}

function hashCode(s: string): number {
  let hash = 0;
  for (let i = 0; i < s.length; i++) {
    hash = (hash << 5) - hash + s.charCodeAt(i);
    hash |= 0;
  }
  return hash;
}
