import type { Persona } from "../../types";

interface PersonaStepProps {
  personas: Persona[];
  onSelect: (name: string) => void;
  onCancel: () => void;
}

const personaIcons: Record<string, string> = {
  "overconfident-engineer": "🚀",
  "zen-master": "🧘",
  "paranoid-perfectionist": "🔍",
  "raging-jerk": "😤",
  "peter-molyneux": "🎩",
};

const personaFlavors: Record<string, string> = {
  "overconfident-engineer": "Ships fast, asks forgiveness later. Bold choices, strong opinions.",
  "zen-master": "Deliberate, minimal, elegant. Every line has purpose.",
  "paranoid-perfectionist": "Tests everything. Edge cases haunt their dreams.",
  "raging-jerk": "Brutally honest code reviews. No sugar coating.",
  "peter-molyneux": "Grand promises, revolutionary vision. The feature will change everything.",
};

export function PersonaStep({ personas, onSelect, onCancel }: PersonaStepProps) {
  return (
    <div>
      <h2 style={{ marginBottom: "0.5rem" }}>Select Persona</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Choose a personality for this agent
      </p>

      <div
        role="list"
        aria-label="Available personas"
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(3, 1fr)",
          gap: "1rem",
          marginBottom: "2rem",
        }}
      >
        {personas.map((p) => (
          <button
            key={p.name}
            role="listitem"
            onClick={() => onSelect(p.name)}
            aria-label={`${p.displayName}: ${personaFlavors[p.name] || p.preamble || ""}`}
            style={{
              background: "var(--bg-secondary)",
              border: "1px solid var(--border)",
              borderRadius: 8,
              padding: "1.25rem",
              textAlign: "center" as const,
              cursor: "pointer",
              transition: "border-color 0.2s",
              color: "var(--text-primary)",
            }}
            onMouseEnter={(e) =>
              (e.currentTarget.style.borderColor = "var(--blue)")
            }
            onMouseLeave={(e) =>
              (e.currentTarget.style.borderColor = "var(--border)")
            }
          >
            <div style={{ fontSize: "1.5rem", marginBottom: "0.5rem" }} aria-hidden="true">
              {personaIcons[p.name] || "🤖"}
            </div>
            <div style={{ fontWeight: 600, marginBottom: "0.25rem" }}>
              {p.displayName}
            </div>
            <div style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>
              {personaFlavors[p.name] || (p.preamble || "").slice(0, 80)}
            </div>
          </button>
        ))}

        {/* No Persona option */}
        <button
          role="listitem"
          onClick={() => onSelect("")}
          aria-label="No Persona: Default behavior, no personality overlay"
          style={{
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
            borderRadius: 8,
            padding: "1.25rem",
            textAlign: "center" as const,
            cursor: "pointer",
            opacity: 0.6,
            color: "var(--text-primary)",
          }}
        >
          <div style={{ fontSize: "1.5rem", marginBottom: "0.5rem" }} aria-hidden="true">—</div>
          <div style={{ fontWeight: 600, marginBottom: "0.25rem" }}>No Persona</div>
          <div style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>
            Default behavior. No personality overlay.
          </div>
        </button>
      </div>

      <button
        onClick={onCancel}
        style={{
          background: "none",
          border: "1px solid var(--border)",
          borderRadius: 6,
          padding: "0.5rem 1rem",
          color: "var(--text-secondary)",
          cursor: "pointer",
        }}
      >
        ← Back
      </button>
    </div>
  );
}
