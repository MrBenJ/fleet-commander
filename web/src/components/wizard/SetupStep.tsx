import { useState } from "react";

interface SetupConfig {
  name: string;
  baseBranch: string;
}

interface SetupStepProps {
  initial: SetupConfig;
  currentBranch: string;
  onDone: (config: SetupConfig) => void;
}

const inputStyle: React.CSSProperties = {
  background: "var(--bg-secondary)",
  border: "1px solid var(--border)",
  borderRadius: 6,
  padding: "0.75rem 1rem",
  color: "var(--text-primary)",
  width: "100%",
  fontSize: "0.9rem",
  outline: "none",
};

const labelStyle: React.CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.75rem",
  textTransform: "uppercase" as const,
  letterSpacing: "0.1em",
  marginBottom: "0.5rem",
  display: "block",
};

export function SetupStep({ initial, currentBranch, onDone }: SetupStepProps) {
  const [config, setConfig] = useState<SetupConfig>(initial);

  const canContinue = config.name.trim().length > 0;

  return (
    <div>
      <h2 style={{ marginBottom: "0.5rem" }}>Squadron Setup</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Tell us about your squadron
      </p>

      <div style={{ display: "flex", flexDirection: "column", gap: "1.5rem", maxWidth: 500 }}>
        <div>
          <label style={labelStyle}>Squadron Name</label>
          <input
            style={inputStyle}
            value={config.name}
            onChange={(e) => setConfig({ ...config, name: e.target.value })}
            placeholder="alpha-strike"
          />
        </div>

        <div>
          <label style={labelStyle}>Base Branch</label>
          <input
            style={inputStyle}
            value={config.baseBranch}
            onChange={(e) => setConfig({ ...config, baseBranch: e.target.value })}
            placeholder={currentBranch}
          />
        </div>

        <button
          onClick={() => onDone(config)}
          disabled={!canContinue}
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
