import type { SquadronAgent } from "../../types";
import { type ConsensusType, consensusInfo, inputStyle, labelStyle } from "./review-constants";
import { HelpTooltip } from "../common/HelpTooltip";

interface ConsensusSelectorProps {
  consensus: ConsensusType;
  reviewMaster: string;
  agents: SquadronAgent[];
  onChange: (consensus: ConsensusType) => void;
  onReviewMasterChange: (name: string) => void;
}

export function ConsensusSelector({
  consensus,
  reviewMaster,
  agents,
  onChange,
  onReviewMasterChange,
}: ConsensusSelectorProps) {
  const info = consensusInfo[consensus];

  return (
    <div>
      {/* Consensus description box */}
      <div
        style={{
          background: "var(--bg-secondary)",
          border: "1px solid var(--border)",
          borderRadius: 8,
          padding: "1.5rem",
          marginBottom: "0.75rem",
          textAlign: "center",
        }}
      >
        <div style={{ fontSize: "2rem", marginBottom: "0.5rem" }}>{info.icon}</div>
        <div style={{ fontWeight: 600, fontSize: "1.1rem", marginBottom: "0.5rem" }}>{info.label}</div>
        <div style={{ color: "var(--text-secondary)", fontSize: "0.9rem", maxWidth: 500, margin: "0 auto", lineHeight: 1.5 }}>
          {info.description}
        </div>
      </div>

      {/* Consensus selector buttons */}
      <div style={{ display: "flex", gap: "0.75rem", marginBottom: "1.5rem" }}>
        {(Object.keys(consensusInfo) as ConsensusType[]).map((type) => (
          <button
            key={type}
            onClick={() => onChange(type)}
            style={{
              flex: 1,
              background: "var(--bg-secondary)",
              border: consensus === type ? "2px solid var(--blue)" : "1px solid var(--border)",
              borderRadius: 8,
              padding: "0.75rem 1rem",
              color: consensus === type ? "var(--blue)" : "var(--text-primary)",
              fontWeight: consensus === type ? 600 : 400,
              cursor: "pointer",
              textAlign: "center",
              fontSize: "0.85rem",
            }}
          >
            {consensusInfo[type].label}
          </button>
        ))}
      </div>

      {/* Single reviewer agent dropdown */}
      {consensus === "review_master" && (
        <div style={{ marginBottom: "1.5rem" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: "0.5rem" }}>
            <label style={labelStyle}>Designated Reviewer</label>
            <HelpTooltip text="The agent responsible for reviewing all other agents' work. This agent will approve or request changes before merging." />
          </div>
          <select
            style={{
              ...inputStyle,
              borderRadius: 6,
              padding: "0.75rem 1rem",
              fontSize: "0.9rem",
            }}
            value={reviewMaster}
            onChange={(e) => onReviewMasterChange(e.target.value)}
          >
            <option value="">Select an agent...</option>
            {agents.map((a) => (
              <option key={a.name} value={a.name}>{a.name}</option>
            ))}
          </select>
        </div>
      )}
    </div>
  );
}
