import { useState } from "react";
import { HelpTooltip } from "../common/HelpTooltip";

interface SetupConfig {
  name: string;
  baseBranch: string;
}

interface SetupStepProps {
  initial: SetupConfig;
  currentBranch: string;
  branches: string[];
  onDone: (config: SetupConfig) => void;
}

const SQUADRON_NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9_-]*$/;

function validateSquadronName(name: string): string | null {
  const trimmed = name.trim();
  if (!trimmed) return null;
  if (trimmed.length > 30) return "Max 30 characters";
  if (!SQUADRON_NAME_RE.test(trimmed)) {
    return "Only letters, digits, hyphens, and underscores; must start with a letter or digit";
  }
  return null;
}

const inputStyle: React.CSSProperties = {
  background: "var(--bg-secondary)",
  border: "1px solid var(--border)",
  borderRadius: 6,
  padding: "0.75rem 1rem",
  color: "var(--text-primary)",
  width: "100%",
  fontSize: "0.9rem",
};

const labelStyle: React.CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.75rem",
  textTransform: "uppercase" as const,
  letterSpacing: "0.1em",
  display: "block",
};

export function SetupStep({ initial, currentBranch, branches, onDone }: SetupStepProps) {
  const [config, setConfig] = useState<SetupConfig>(initial);

  const trimmedName = config.name.trim();
  const nameError = validateSquadronName(config.name);
  const canContinue = trimmedName.length > 0 && nameError === null;

  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
      <h2 style={{ marginBottom: "0.5rem" }}>Squadron Setup</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Tell us about your squadron
      </p>

      <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem", maxWidth: 500, width: "100%" }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: "0.5rem" }}>
            <label htmlFor="squadron-name" style={labelStyle}>Squadron Name</label>
            <HelpTooltip text="A unique name for this group of agents. Used to identify the squadron in logs and context channels." />
          </div>
          <input
            id="squadron-name"
            style={{
              ...inputStyle,
              border: nameError ? "1px solid var(--red)" : inputStyle.border,
            }}
            value={config.name}
            onChange={(e) => setConfig({ ...config, name: e.target.value })}
            placeholder="homepage-fixes"
            aria-required="true"
            aria-invalid={nameError !== null}
            aria-describedby={nameError ? "squadron-name-error" : undefined}
          />
          {nameError && (
            <div
              id="squadron-name-error"
              role="alert"
              style={{ color: "var(--red)", fontSize: "0.75rem", marginTop: "0.25rem" }}
            >
              {nameError}
            </div>
          )}
        </div>

        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 4, marginBottom: "0.5rem" }}>
            <label htmlFor="base-branch" style={labelStyle}>Base Branch</label>
            <HelpTooltip text="The git branch that all agent worktrees will be created from. Each agent gets its own branch forked from this one." />
          </div>
          <select
            id="base-branch"
            style={{ ...inputStyle, appearance: "auto" }}
            value={config.baseBranch}
            onChange={(e) => setConfig({ ...config, baseBranch: e.target.value })}
          >
            {branches.length === 0 && (
              <option value={currentBranch}>{currentBranch}</option>
            )}
            {branches.map((b) => (
              <option key={b} value={b}>{b}</option>
            ))}
          </select>
        </div>

        <button
          onClick={() => onDone({ ...config, name: trimmedName })}
          disabled={!canContinue}
          aria-disabled={!canContinue}
          style={{
            background: canContinue ? "var(--green)" : "var(--bg-secondary)",
            color: canContinue ? "#fff" : "var(--text-muted)",
            border: "none",
            borderRadius: 8,
            padding: "0.75rem 2rem",
            fontSize: "1rem",
            fontWeight: 600,
            cursor: canContinue ? "pointer" : "default",
            alignSelf: "flex-start",
          }}
        >
          Continue →
        </button>
      </div>
    </div>
  );
}
