import type { SquadronAgent, Persona } from "../../types";
import { driverColors, driverTextColors, personaIcons, inputStyle, labelStyle } from "./review-constants";
import { CodeEditor } from "../common/CodeEditor";
import { HelpTooltip } from "../common/HelpTooltip";

interface AgentCardProps {
  agent: SquadronAgent;
  isEditing: boolean;
  editDraft: SquadronAgent | null;
  drivers: string[];
  personas: Persona[];
  onEdit: () => void;
  onSave: () => void;
  onCancel: () => void;
  onRemove: () => void;
  onDraftChange: (draft: SquadronAgent) => void;
}

export function AgentCard({
  agent,
  isEditing,
  editDraft,
  drivers,
  personas,
  onEdit,
  onSave,
  onCancel,
  onRemove,
  onDraftChange,
}: AgentCardProps) {
  const getPersonaIcon = (key: string) => personaIcons[key] || "";
  const getPersonaName = (key: string) => {
    if (!key) return "";
    const p = personas.find((x) => x.name === key);
    return p?.displayName || key;
  };

  if (isEditing && editDraft) {
    return (
      <li
        style={{
          background: "var(--bg-secondary)",
          border: "2px solid var(--blue)",
          borderRadius: 8,
          padding: "1rem",
        }}
      >
        <div style={{ display: "flex", flexDirection: "column", gap: "0.75rem" }}>
          <div style={{ display: "flex", gap: "0.75rem" }}>
            <div style={{ flex: 1 }}>
              <label htmlFor={`edit-name-${agent.name}`} style={labelStyle}>
                Agent Name
                <HelpTooltip text="A short identifier for this agent. Used in branch names and tmux session names." />
              </label>
              <input
                id={`edit-name-${agent.name}`}
                style={inputStyle}
                value={editDraft.name}
                onChange={(e) => onDraftChange({ ...editDraft, name: e.target.value })}
              />
            </div>
            <div style={{ flex: 1 }}>
              <label htmlFor={`edit-branch-${agent.name}`} style={labelStyle}>
                Branch
                <HelpTooltip text="The git branch name for this agent's worktree. Each agent works in its own isolated branch." />
              </label>
              <input
                id={`edit-branch-${agent.name}`}
                style={inputStyle}
                value={editDraft.branch}
                onChange={(e) => onDraftChange({ ...editDraft, branch: e.target.value })}
              />
            </div>
            <div style={{ width: 150 }}>
              <label htmlFor={`edit-harness-${agent.name}`} style={labelStyle}>
                Harness
                <HelpTooltip text="The harness configures how the agent runs — including permissions, tools, and execution constraints." />
              </label>
              <select
                id={`edit-harness-${agent.name}`}
                style={inputStyle}
                value={editDraft.driver}
                onChange={(e) => onDraftChange({ ...editDraft, driver: e.target.value })}
              >
                {drivers.map((d) => (
                  <option key={d} value={d}>{d}</option>
                ))}
              </select>
            </div>
            <div style={{ width: 180 }}>
              <label htmlFor={`edit-persona-${agent.name}`} style={labelStyle}>
                Persona
                <HelpTooltip text="A persona defines the agent's coding style, expertise areas, and approach to problem-solving." />
              </label>
              <select
                id={`edit-persona-${agent.name}`}
                style={inputStyle}
                value={editDraft.persona}
                onChange={(e) => onDraftChange({ ...editDraft, persona: e.target.value })}
              >
                <option value="">none</option>
                {personas.map((p) => (
                  <option key={p.name} value={p.name}>{p.displayName}</option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label id={`edit-prompt-label-${agent.name}`} style={labelStyle}>Prompt</label>
            <CodeEditor
              labelId={`edit-prompt-label-${agent.name}`}
              value={editDraft.prompt}
              onChange={(val) => onDraftChange({ ...editDraft, prompt: val })}
              minHeight={200}
            />
          </div>
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <button
              onClick={onSave}
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
              onClick={onCancel}
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
      </li>
    );
  }

  return (
    <li
      style={{
        background: "var(--bg-secondary)",
        border: "1px solid var(--border)",
        borderRadius: 8,
        padding: "1rem",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
        <div style={{ flex: 1 }}>
          <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
            <strong>{agent.name}</strong>
            <span
              style={{
                fontSize: "0.7rem",
                background: driverColors[agent.driver] || driverColors.generic,
                color: driverTextColors[agent.driver] || driverTextColors.generic,
                padding: "0.15rem 0.5rem",
                borderRadius: 10,
              }}
            >
              {agent.driver}
            </span>
            <span style={{ fontSize: "0.7rem", color: "var(--text-secondary)" }}>
              {agent.branch}
            </span>
            {agent.persona && (
              <span style={{ fontSize: "0.7rem", color: "var(--purple, #a855f7)" }}>
                <span aria-hidden="true">{getPersonaIcon(agent.persona)}</span> {getPersonaName(agent.persona)}
              </span>
            )}
          </div>
          <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)", marginTop: "0.25rem" }}>
            {(agent.prompt || "").slice(0, 120)}{(agent.prompt || "").length > 120 ? "..." : ""}
          </div>
        </div>
        <button
          onClick={onEdit}
          aria-label={`Edit agent ${agent.name}`}
          style={{ background: "none", border: "none", color: "var(--text-secondary)", cursor: "pointer" }}
        >
          <span aria-hidden="true">{"\u270F\uFE0F"}</span>
        </button>
        <button
          onClick={onRemove}
          aria-label={`Remove agent ${agent.name}`}
          style={{ background: "none", border: "none", color: "var(--red)", cursor: "pointer", fontSize: "1rem" }}
        >
          X
        </button>
      </div>
    </li>
  );
}
