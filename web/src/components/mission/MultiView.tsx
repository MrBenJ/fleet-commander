import type { SquadronAgent } from "../../types";
import { DriverIcon } from "./AgentPill";

interface MultiViewProps {
  agents: SquadronAgent[];
  mergeMaster?: string;
}

function getGridColumns(count: number): string {
  if (count <= 1) return "1fr";
  if (count <= 4) return "1fr 1fr";
  if (count <= 9) return "1fr 1fr 1fr";
  return "1fr 1fr 1fr 1fr";
}

export function MultiView({ agents, mergeMaster }: MultiViewProps) {
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
              display: "flex",
              alignItems: "center",
              gap: "0.4rem",
            }}
          >
            <DriverIcon driver={agent.driver} size={12} />
            <span>{agent.name}</span>
            {mergeMaster && agent.name === mergeMaster && (
              <span
                style={{
                  fontSize: "0.5rem",
                  fontWeight: 700,
                  background: "#a855f7",
                  color: "#fff",
                  padding: "0.05rem 0.3rem",
                  borderRadius: 6,
                  letterSpacing: "0.03em",
                  lineHeight: 1,
                  textTransform: "uppercase",
                }}
              >
                MERGE
              </span>
            )}
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
