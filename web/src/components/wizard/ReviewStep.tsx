import { useReducer } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { launchSquadron, ApiError } from "../../api";
import type { ConsensusType } from "./review-constants";
import { AgentCard } from "./AgentCard";
import { ConsensusSelector } from "./ConsensusSelector";
import { HelpTooltip } from "../common/HelpTooltip";

interface ReviewConfig {
  name: string;
  baseBranch: string;
}

interface ReviewStepProps {
  config: ReviewConfig;
  agents: SquadronAgent[];
  drivers: string[];
  personas: Persona[];
  ghAvailable: boolean;
  onLaunched: (name: string, agents: SquadronAgent[], config: { consensus: string; autoMerge: boolean; mergeMaster?: string }) => void;
  onEdit: () => void;
  onAddMore: () => void;
  onAgentsChanged: (agents: SquadronAgent[]) => void;
}

// --- State machine ---

type ReviewState = {
  launching: boolean;
  error: string | null;
  errorDetails: string[];
  editingIdx: number | null;
  editDraft: SquadronAgent | null;
  consensus: ConsensusType;
  reviewMaster: string;
  autoMerge: boolean;
  autoPR: boolean;
};

type ReviewAction =
  | { type: "LAUNCH_START" }
  | { type: "LAUNCH_SUCCESS" }
  | { type: "LAUNCH_ERROR"; error: string; details?: string[] }
  | { type: "START_EDITING"; idx: number; agent: SquadronAgent }
  | { type: "UPDATE_DRAFT"; draft: SquadronAgent }
  | { type: "SAVE_EDIT" }
  | { type: "CANCEL_EDIT" }
  | { type: "SET_CONSENSUS"; consensus: ConsensusType }
  | { type: "SET_REVIEW_MASTER"; name: string }
  | { type: "SET_AUTO_MERGE"; enabled: boolean }
  | { type: "SET_AUTO_PR"; enabled: boolean };

const initialState: ReviewState = {
  launching: false,
  error: null,
  errorDetails: [],
  editingIdx: null,
  editDraft: null,
  consensus: "universal",
  reviewMaster: "",
  autoMerge: true,
  autoPR: false,
};

function reviewReducer(state: ReviewState, action: ReviewAction): ReviewState {
  switch (action.type) {
    case "LAUNCH_START":
      return { ...state, launching: true, error: null, errorDetails: [] };
    case "LAUNCH_SUCCESS":
      return { ...state, launching: false };
    case "LAUNCH_ERROR":
      return { ...state, launching: false, error: action.error, errorDetails: action.details ?? [] };
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
      return { ...state, autoMerge: action.enabled, autoPR: action.enabled ? state.autoPR : false };
    case "SET_AUTO_PR":
      return { ...state, autoPR: action.enabled };
  }
}

// --- Component ---

export function ReviewStep({
  config,
  agents,
  drivers,
  personas,
  ghAvailable,
  onLaunched,
  onAddMore,
  onAgentsChanged,
}: ReviewStepProps) {
  const [state, dispatch] = useReducer(reviewReducer, initialState);

  const handleLaunch = async () => {
    dispatch({ type: "LAUNCH_START" });
    try {
      const result = await launchSquadron({
        name: config.name,
        consensus: state.consensus,
        reviewMaster: state.consensus === "review_master" ? state.reviewMaster || undefined : undefined,
        baseBranch: config.baseBranch || undefined,
        autoMerge: state.autoMerge,
        autoPR: state.autoMerge && state.autoPR ? true : undefined,
        agents: agents,
      });
      dispatch({ type: "LAUNCH_SUCCESS" });
      onLaunched(config.name, agents, {
        consensus: state.consensus,
        autoMerge: state.autoMerge,
        mergeMaster: result.mergeMaster || undefined,
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : "Launch failed";
      const details = err instanceof ApiError ? err.details : [];
      dispatch({ type: "LAUNCH_ERROR", error: message, details });
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
      <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: state.autoMerge ? "0.5rem" : "1.5rem" }}>
        <input
          type="checkbox"
          id="auto-merge"
          checked={state.autoMerge}
          onChange={(e) => dispatch({ type: "SET_AUTO_MERGE", enabled: e.target.checked })}
          style={{ width: 18, height: 18 }}
        />
        <label htmlFor="auto-merge">
          Auto-merge all agent branches into <span style={{ color: "var(--blue)", fontWeight: 500 }}>'{config.name}-merged'</span>
          <HelpTooltip text="When enabled, all agent branches will be automatically merged into a single combined branch after the squadron completes." />
        </label>
      </div>

      {/* Auto PR checkbox (only visible when auto-merge is enabled) */}
      {state.autoMerge && (
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.5rem", marginLeft: "2rem" }}>
          <input
            type="checkbox"
            id="auto-pr"
            checked={state.autoPR}
            disabled={!ghAvailable}
            onChange={(e) => dispatch({ type: "SET_AUTO_PR", enabled: e.target.checked })}
            style={{ width: 18, height: 18, opacity: ghAvailable ? 1 : 0.5 }}
          />
          <label htmlFor="auto-pr" style={{ opacity: ghAvailable ? 1 : 0.6 }}>
            Create pull request after merge
            <HelpTooltip text={
              ghAvailable
                ? "When enabled, the merge master will push the merged branch, create a GitHub PR, and monitor CI status until checks pass. This requires the gh CLI tool to be installed and authenticated."
                : "Requires the gh CLI tool (https://cli.github.com). Install it and run `gh auth login` to enable this option."
            } />
          </label>
        </div>
      )}

      <div role="alert" aria-live="assertive">
        {state.error && (
          <div style={{ color: "var(--red)", marginBottom: "1rem", fontSize: "0.9rem" }}>
            <div style={{ fontWeight: 600 }}>{state.error}</div>
            {state.errorDetails.length > 0 && (
              <ul style={{ margin: "0.4rem 0 0", paddingLeft: "1.25rem", fontSize: "0.85rem" }}>
                {state.errorDetails.map((d, i) => (
                  <li key={i}>{d}</li>
                ))}
              </ul>
            )}
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
