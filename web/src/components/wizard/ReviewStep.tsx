import { useState } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { launchSquadron } from "../../api";

interface ReviewConfig {
  name: string;
  baseBranch: string;
}

type ConsensusType = "universal" | "review_master" | "none";

const consensusInfo: Record<ConsensusType, { icon: string; label: string; description: string }> = {
  universal: {
    icon: "\u{1F91D}",
    label: "Universal Consensus",
    description:
      "Every agent reviews every other agent's work. All agents must approve before merging. Best for high-stakes changes where multiple perspectives catch more issues.",
  },
  review_master: {
    icon: "\u{1F464}",
    label: "Single Reviewer",
    description:
      "One designated agent reviews all other agents' work. Streamlined review flow with a single point of approval. Best when one agent has the broadest context.",
  },
  none: {
    icon: "\u{26A1}",
    label: "None",
    description:
      "No review step. Agents complete their tasks and stop. Best for independent tasks that don't need cross-review.",
  },
};

interface ReviewStepProps {
  config: ReviewConfig;
  agents: SquadronAgent[];
  drivers: string[];
  personas: Persona[];
  onLaunched: (name: string, agents: SquadronAgent[], config: { consensus: string; autoMerge: boolean }) => void;
  onEdit: () => void;
  onAddMore: () => void;
  onAgentsChanged: (agents: SquadronAgent[]) => void;
}

const personaIcons: Record<string, string> = {
  "overconfident-engineer": "\u{1F680}",
  "zen-master": "\u{1F9D8}",
  "paranoid-perfectionist": "\u{1F50D}",
  "raging-jerk": "\u{1F624}",
  "peter-molyneux": "\u{1F3A9}",
};

const driverColors: Record<string, string> = {
  "claude-code": "rgba(31,111,235,0.2)",
  codex: "rgba(46,160,67,0.2)",
  aider: "rgba(240,136,62,0.2)",
  generic: "rgba(139,148,158,0.2)",
};

const driverTextColors: Record<string, string> = {
  "claude-code": "var(--blue)",
  codex: "var(--green)",
  aider: "var(--orange)",
  generic: "var(--text-secondary)",
};

const inputStyle: React.CSSProperties = {
  background: "var(--bg-primary)",
  border: "1px solid var(--border)",
  borderRadius: 4,
  padding: "0.5rem",
  color: "var(--text-primary)",
  width: "100%",
  fontSize: "0.85rem",
  outline: "none",
};

const labelStyle: React.CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.7rem",
  textTransform: "uppercase" as const,
  marginBottom: "0.25rem",
  display: "block",
};

export function ReviewStep({
  config,
  agents,
  drivers,
  personas,
  onLaunched,
  onAddMore,
  onAgentsChanged,
}: ReviewStepProps) {
  const [launching, setLaunching] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [editDraft, setEditDraft] = useState<SquadronAgent | null>(null);
  const [consensus, setConsensus] = useState<ConsensusType>("universal");
  const [reviewMaster, setReviewMaster] = useState("");
  const [autoMerge, setAutoMerge] = useState(true);

  const handleLaunch = async () => {
    setLaunching(true);
    setError(null);
    try {
      await launchSquadron({
        name: config.name,
        consensus,
        reviewMaster: consensus === "review_master" ? reviewMaster || undefined : undefined,
        baseBranch: config.baseBranch || undefined,
        autoMerge,
        agents: agents,
      });
      onLaunched(config.name, agents, { consensus, autoMerge });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Launch failed");
    } finally {
      setLaunching(false);
    }
  };

  const startEditing = (idx: number) => {
    setEditingIdx(idx);
    setEditDraft({ ...agents[idx] });
  };

  const saveEdit = () => {
    if (editingIdx === null || !editDraft) return;
    const updated = [...agents];
    updated[editingIdx] = editDraft;
    onAgentsChanged(updated);
    setEditingIdx(null);
    setEditDraft(null);
  };

  const cancelEdit = () => {
    setEditingIdx(null);
    setEditDraft(null);
  };

  const removeAgent = (idx: number) => {
    onAgentsChanged(agents.filter((_, i) => i !== idx));
  };

  const getPersonaIcon = (key: string) => personaIcons[key] || "";
  const getPersonaName = (key: string) => {
    if (!key) return "";
    const p = personas.find((x) => x.name === key);
    return p?.displayName || key;
  };

  const info = consensusInfo[consensus];

  return (
    <div>
      <div style={{ marginBottom: "1.5rem" }}>
        <span style={{ fontWeight: 600, fontSize: "1.1rem" }}>Squadron: </span>
        <span style={{ color: "var(--blue)", fontSize: "1.1rem" }}>{config.name}</span>
      </div>

      {/* Agent cards */}
      <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginBottom: "1.5rem" }}>
        {agents.map((a, idx) => (
          <div
            key={idx}
            style={{
              background: "var(--bg-secondary)",
              border: editingIdx === idx ? "2px solid var(--blue)" : "1px solid var(--border)",
              borderRadius: 8,
              padding: "1rem",
            }}
          >
            {editingIdx === idx && editDraft ? (
              <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
                <div style={{ display: "flex", gap: "0.75rem" }}>
                  <div style={{ flex: 1 }}>
                    <label style={labelStyle}>Agent Name</label>
                    <input
                      style={inputStyle}
                      value={editDraft.name}
                      onChange={(e) => setEditDraft({ ...editDraft, name: e.target.value })}
                    />
                  </div>
                  <div style={{ flex: 1 }}>
                    <label style={labelStyle}>Branch</label>
                    <input
                      style={inputStyle}
                      value={editDraft.branch}
                      onChange={(e) => setEditDraft({ ...editDraft, branch: e.target.value })}
                    />
                  </div>
                  <div style={{ width: 150 }}>
                    <label style={labelStyle}>Harness</label>
                    <select
                      style={inputStyle}
                      value={editDraft.driver}
                      onChange={(e) => setEditDraft({ ...editDraft, driver: e.target.value })}
                    >
                      {drivers.map((d) => (
                        <option key={d} value={d}>{d}</option>
                      ))}
                    </select>
                  </div>
                  <div style={{ width: 180 }}>
                    <label style={labelStyle}>Persona</label>
                    <select
                      style={inputStyle}
                      value={editDraft.persona}
                      onChange={(e) => setEditDraft({ ...editDraft, persona: e.target.value })}
                    >
                      <option value="">none</option>
                      {personas.map((p) => (
                        <option key={p.name} value={p.name}>{p.displayName}</option>
                      ))}
                    </select>
                  </div>
                </div>
                <div>
                  <label style={labelStyle}>Prompt</label>
                  <textarea
                    style={{ ...inputStyle, minHeight: 80, resize: "vertical" }}
                    value={editDraft.prompt}
                    onChange={(e) => setEditDraft({ ...editDraft, prompt: e.target.value })}
                  />
                </div>
                <div style={{ display: "flex", gap: "0.5rem" }}>
                  <button
                    onClick={saveEdit}
                    style={{
                      background: "var(--green)",
                      color: "#fff",
                      border: "none",
                      borderRadius: 6,
                      padding: "0.4rem 1rem",
                      fontSize: "0.85rem",
                      cursor: "pointer",
                    }}
                  >
                    Save
                  </button>
                  <button
                    onClick={cancelEdit}
                    style={{
                      background: "none",
                      border: "1px solid var(--border)",
                      borderRadius: 6,
                      padding: "0.4rem 1rem",
                      fontSize: "0.85rem",
                      color: "var(--text-secondary)",
                      cursor: "pointer",
                    }}
                  >
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
                <div style={{ flex: 1 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
                    <strong>{a.name}</strong>
                    <span
                      style={{
                        fontSize: "0.7rem",
                        background: driverColors[a.driver] || driverColors.generic,
                        color: driverTextColors[a.driver] || driverTextColors.generic,
                        padding: "0.15rem 0.5rem",
                        borderRadius: 10,
                      }}
                    >
                      {a.driver}
                    </span>
                    <span style={{ fontSize: "0.7rem", color: "var(--text-secondary)" }}>
                      {a.branch}
                    </span>
                    {a.persona && (
                      <span style={{ fontSize: "0.7rem", color: "var(--purple, #a855f7)" }}>
                        {getPersonaIcon(a.persona)} {getPersonaName(a.persona)}
                      </span>
                    )}
                  </div>
                  <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: "0.25rem" }}>
                    {(a.prompt || "").slice(0, 120)}{(a.prompt || "").length > 120 ? "..." : ""}
                  </div>
                </div>
                <button
                  onClick={() => startEditing(idx)}
                  style={{ background: "none", border: "none", color: "var(--text-secondary)", cursor: "pointer" }}
                >
                  ✏️
                </button>
                <button
                  onClick={() => removeAgent(idx)}
                  style={{ background: "none", border: "none", color: "var(--red)", cursor: "pointer", fontSize: "1rem" }}
                >
                  X
                </button>
              </div>
            )}
          </div>
        ))}
      </div>

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
            onClick={() => {
              setConsensus(type);
              if (type !== "review_master") setReviewMaster("");
            }}
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
          <label style={{ ...labelStyle, marginBottom: "0.5rem" }}>Designated Reviewer</label>
          <select
            style={{
              ...inputStyle,
              borderRadius: 6,
              padding: "0.75rem 1rem",
              fontSize: "0.9rem",
            }}
            value={reviewMaster}
            onChange={(e) => setReviewMaster(e.target.value)}
          >
            <option value="">Select an agent...</option>
            {agents.map((a) => (
              <option key={a.name} value={a.name}>{a.name}</option>
            ))}
          </select>
        </div>
      )}

      {/* Auto-merge checkbox */}
      <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.5rem" }}>
        <input
          type="checkbox"
          checked={autoMerge}
          onChange={(e) => setAutoMerge(e.target.checked)}
          style={{ width: 18, height: 18 }}
        />
        <span>
          Auto-merge all agent branches into <span style={{ color: "var(--blue)", fontWeight: 500 }}>'{config.name}-merged'</span>
        </span>
      </div>

      {error && (
        <div style={{ color: "var(--red)", marginBottom: "1rem", fontSize: "0.9rem" }}>
          {error}
        </div>
      )}

      <div style={{ display: "flex", gap: "1rem" }}>
        <button
          onClick={handleLaunch}
          disabled={launching || agents.length === 0}
          style={{
            flex: 1,
            background: "var(--green)",
            color: "#fff",
            border: "none",
            borderRadius: 8,
            padding: "0.75rem",
            fontSize: "1rem",
            fontWeight: 600,
            cursor: launching ? "wait" : "pointer",
            opacity: launching ? 0.6 : 1,
          }}
        >
          {launching ? "Launching..." : "Launch Squadron"}
        </button>
        <button
          onClick={onAddMore}
          style={{
            background: "var(--bg-tertiary)",
            color: "var(--text-primary)",
            border: "1px solid var(--border)",
            borderRadius: 8,
            padding: "0.75rem 1.5rem",
            cursor: "pointer",
          }}
        >
          + Add More
        </button>
      </div>
    </div>
  );
}
