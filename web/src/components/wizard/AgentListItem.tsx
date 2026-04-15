import type { SquadronAgent, Persona } from "../../types";

interface AgentListItemProps {
  agent: SquadronAgent;
  onRemove: () => void;
  onPickPersona: () => void;
  personas: Persona[];
}

const personaIcons: Record<string, string> = {
  "overconfident-engineer": "\u{1F680}",
  "zen-master": "\u{1F9D8}",
  "paranoid-perfectionist": "\u{1F50D}",
  "raging-jerk": "\u{1F624}",
  "peter-molyneux": "\u{1F3A9}",
};

function getPersonaDisplay(personaKey: string, personas: Persona[]) {
  if (!personaKey) return "No persona selected";
  const p = personas.find((x) => x.name === personaKey);
  const name = p?.displayName || personaKey;
  const icon = personaIcons[personaKey] || "";
  return `\u270F\uFE0F ${name} ${icon}`;
}

export function AgentListItem({ agent, onRemove, onPickPersona, personas }: AgentListItemProps) {
  return (
    <li
      style={{
        display: "flex",
        alignItems: "center",
        gap: "1rem",
        background: "var(--bg-secondary)",
        border: "1px solid var(--border)",
        borderRadius: 8,
        padding: "0.75rem 1rem",
      }}
    >
      <div style={{ flex: 1 }}>
        <strong>{agent.name}</strong>
        <span
          style={{
            fontSize: "0.7rem",
            background: "rgba(31,111,235,0.2)",
            color: "var(--blue)",
            padding: "0.15rem 0.5rem",
            borderRadius: 10,
            marginLeft: "0.5rem",
          }}
        >
          {agent.driver}
        </span>
        <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: 2 }}>
          {(agent.prompt || "").slice(0, 80)}{(agent.prompt || "").length > 80 ? "..." : ""}
        </div>
      </div>
      <button
        onClick={onPickPersona}
        aria-label={`Select persona for ${agent.name}`}
        style={{
          background: "none",
          border: "1px solid var(--border)",
          borderRadius: 6,
          padding: "0.3rem 0.6rem",
          color: "var(--text-secondary)",
          fontSize: "0.75rem",
          cursor: "pointer",
        }}
      >
        {getPersonaDisplay(agent.persona, personas)}
      </button>
      <button
        onClick={onRemove}
        aria-label={`Remove agent ${agent.name}`}
        style={{
          background: "none",
          border: "none",
          color: "var(--red)",
          cursor: "pointer",
          fontSize: "1rem",
        }}
      >
        X
      </button>
    </li>
  );
}
