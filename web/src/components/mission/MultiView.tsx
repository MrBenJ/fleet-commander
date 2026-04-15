import type { SquadronAgent } from "../../types";

interface MultiViewProps {
  agents: SquadronAgent[];
}

function getGridColumns(count: number): string {
  if (count <= 1) return "1fr";
  if (count <= 4) return "1fr 1fr";
  if (count <= 9) return "1fr 1fr 1fr";
  return "1fr 1fr 1fr 1fr";
}

export function MultiView({ agents }: MultiViewProps) {
  const cols = getGridColumns(agents.length);

  return (
    <div
      data-testid="multi-view-grid"
      style={{
        display: "grid",
        gridTemplateColumns: cols,
        gap: "2px",
        flex: 1,
        minHeight: 0,
        background: "var(--border)",
      }}
    >
      {agents.map((agent) => (
        <div
          key={agent.name}
          style={{
            display: "flex",
            flexDirection: "column",
            minHeight: 0,
            background: "#0d1117",
          }}
        >
          <div
            style={{
              padding: "0.25rem 0.5rem",
              fontSize: "0.7rem",
              fontWeight: 600,
              color: "var(--text-secondary)",
              background: "var(--bg-secondary)",
              borderBottom: "1px solid var(--border)",
              textTransform: "uppercase",
              letterSpacing: "0.05em",
            }}
          >
            {agent.name}
          </div>
          <iframe
            src={`/terminal/${agent.name}`}
            title={`Terminal: ${agent.name}`}
            style={{
              flex: 1,
              width: "100%",
              border: "none",
              minHeight: 0,
            }}
          />
        </div>
      ))}
    </div>
  );
}
