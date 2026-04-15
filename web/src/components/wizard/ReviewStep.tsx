import { useReducer } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { launchSquadron } from "../../api";
import type { ConsensusType } from "./review-constants";
import { AgentCard } from "./AgentCard";
import { ConsensusSelector } from "./ConsensusSelector";

interface ReviewConfig {
  name: string;
  baseBranch: string;
}

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

// --- State machine ---

type ReviewState = {
  launching: boolean;
  error: string | null;
  editingIdx: number | null;
  editDraft: SquadronAgent | null;
  consensus: ConsensusType;
  reviewMaster: string;
  autoMerge: boolean;
};

type ReviewAction =
  | { type: "LAUNCH_START" }
  | { type: "LAUNCH_SUCCESS" }
  | { type: "LAUNCH_ERROR"; error: string }
  | { type: "START_EDITING"; idx: number; agent: SquadronAgent }
  | { type: "UPDATE_DRAFT"; draft: SquadronAgent }
  | { type: "SAVE_EDIT" }
  | { type: "CANCEL_EDIT" }
  | { type: "SET_CONSENSUS"; consensus: ConsensusType }
  | { type: "SET_REVIEW_MASTER"; name: string }
  | { type: "SET_AUTO_MERGE"; enabled: boolean };

const initialState: ReviewState = {
  launching: false,
  error: null,
  editingIdx: null,
  editDraft: null,
  consensus: "universal",
  reviewMaster: "",
  autoMerge: true,
};

function reviewReducer(state: ReviewState, action: ReviewAction): ReviewState {
  switch (action.type) {
    case "LAUNCH_START":
      return { ...state, launching: true, error: null };
    case "LAUNCH_SUCCESS":
      return { ...state, launching: false };
    case "LAUNCH_ERROR":
      return { ...state, launching: false, error: action.error };
    case "START_EDITING":
      return { ...state, editingIdx: action.idx, editDraft: { ...action.agent } };
    case "UPDATE_DRAFT":
      return { ...state, editDraft: action.draft };
    case "SAVE_EDIT":
      return { ...state, editingIdx: null, editDraft: null };
    case "CANCEL_EDIT":
      return { ...state, editingIdx: null, editDraft: null };
    case "SET_CONSENSUS":
      return {
        ...state,
        consensus: action.consensus,
        reviewMaster: action.consensus !== "review_master" ? "" : state.reviewMaster,
      };
    case "SET_REVIEW_MASTER":
      return { ...state, reviewMaster: action.name };
    case "SET_AUTO_MERGE":
      return { ...state, autoMerge: action.enabled };
  }
}

// --- Component ---

export function ReviewStep({
  config,
  agents,
  drivers,
  personas,
  onLaunched,
  onAddMore,
  onAgentsChanged,
}: ReviewStepProps) {
  const [state, dispatch] = useReducer(reviewReducer, initialState);

  const handleLaunch = async () => {
    dispatch({ type: "LAUNCH_START" });
    try {
      await launchSquadron({
        name: config.name,
        consensus: state.consensus,
        reviewMaster: state.consensus === "review_master" ? state.reviewMaster || undefined : undefined,
        baseBranch: config.baseBranch || undefined,
        autoMerge: state.autoMerge,
        agents: agents,
      });
      dispatch({ type: "LAUNCH_SUCCESS" });
      onLaunched(config.name, agents, { consensus: state.consensus, autoMerge: state.autoMerge });
    } catch (err) {
      dispatch({ type: "LAUNCH_ERROR", error: err instanceof Error ? err.message : "Launch failed" });
    }
  };

  const handleSaveEdit = () => {
    if (state.editingIdx === null || !state.editDraft) return;
    const updated = [...agents];
    updated[state.editingIdx] = state.editDraft;
    onAgentsChanged(updated);
    dispatch({ type: "SAVE_EDIT" });
  };

  const removeAgent = (idx: number) => {
    onAgentsChanged(agents.filter((_, i) => i !== idx));
  };

  return (
    <div>
      <div style={{ marginBottom: "1.5rem" }}>
        <h2 style={{ display: "inline", fontSize: "1.1rem" }}>
          <span style={{ fontWeight: 600 }}>Squadron: </span>
          <span style={{ color: "var(--blue)" }}>{config.name}</span>
        </h2>
      </div>

      {/* Agent cards */}
      <ul style={{ display: "flex", flexDirection: "column", gap: "0.75rem", marginBottom: "1.5rem", listStyle: "none", padding: 0 }} aria-label="Agents to launch">
        {agents.map((a, idx) => (
          <AgentCard
            key={idx}
            agent={a}
            isEditing={state.editingIdx === idx}
            editDraft={state.editDraft}
            drivers={drivers}
            personas={personas}
            onEdit={() => dispatch({ type: "START_EDITING", idx, agent: a })}
            onSave={handleSaveEdit}
            onCancel={() => dispatch({ type: "CANCEL_EDIT" })}
            onRemove={() => removeAgent(idx)}
            onDraftChange={(draft) => dispatch({ type: "UPDATE_DRAFT", draft })}
          />
        ))}
      </ul>

      <ConsensusSelector
        consensus={state.consensus}
        reviewMaster={state.reviewMaster}
        agents={agents}
        onChange={(consensus) => dispatch({ type: "SET_CONSENSUS", consensus })}
        onReviewMasterChange={(name) => dispatch({ type: "SET_REVIEW_MASTER", name })}
      />

      {/* Auto-merge checkbox */}
      <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.5rem" }}>
        <input
          type="checkbox"
          id="auto-merge"
          checked={state.autoMerge}
          onChange={(e) => dispatch({ type: "SET_AUTO_MERGE", enabled: e.target.checked })}
          style={{ width: 18, height: 18 }}
        />
        <label htmlFor="auto-merge">
          Auto-merge all agent branches into <span style={{ color: "var(--blue)", fontWeight: 500 }}>'{config.name}-merged'</span>
        </label>
      </div>

      <div role="alert" aria-live="assertive">
        {state.error && (
          <div style={{ color: "var(--red)", marginBottom: "1rem", fontSize: "0.9rem" }}>
            {state.error}
          </div>
        )}
      </div>

      <div style={{ display: "flex", gap: "1rem" }}>
        <button
          onClick={handleLaunch}
          disabled={state.launching || agents.length === 0}
          aria-disabled={state.launching || agents.length === 0}
          style={{
            flex: 1,
            background: "var(--green)",
            color: "#fff",
            border: "none",
            borderRadius: 8,
            padding: "0.75rem",
            fontSize: "1rem",
            fontWeight: 600,
            cursor: state.launching ? "wait" : "pointer",
            opacity: state.launching ? 0.6 : 1,
          }}
        >
          {state.launching ? "Launching..." : "Launch Squadron"}
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
