import { useState } from "react";

interface SetupConfig {
  name: string;
  baseBranch: string;
  consensus: "universal" | "review_master" | "none";
  reviewMaster: string;
  autoMerge: boolean;
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

  const consensusOptions = [
    { value: "universal" as const, label: "Universal", desc: "All review all" },
    { value: "review_master" as const, label: "Review Master", desc: "One reviewer" },
    { value: "none" as const, label: "None", desc: "No review" },
  ];

  const canContinue = config.name.trim().length > 0;

  return (
    <div>
      <h2 style={{ marginBottom: "0.5rem" }}>Squadron Setup</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Configure the basics for your squadron
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

        <div>
          <label style={labelStyle}>Consensus Type</label>
          <div style={{ display: "flex", gap: "0.75rem" }}>
            {consensusOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setConfig({ ...config, consensus: opt.value })}
                style={{
                  flex: 1,
                  background: "var(--bg-secondary)",
                  border: config.consensus === opt.value
                    ? "2px solid var(--blue)"
                    : "1px solid var(--border)",
                  borderRadius: 8,
                  padding: "1rem",
                  color:
                    config.consensus === opt.value
                      ? "var(--blue)"
                      : "var(--text-primary)",
                  fontWeight: config.consensus === opt.value ? 600 : 400,
                  cursor: "pointer",
                  textAlign: "center" as const,
                }}
              >
                <div>{opt.label}</div>
                <div style={{ fontSize: "0.75rem", color: "var(--text-secondary)", marginTop: 4 }}>
                  {opt.desc}
                </div>
              </button>
            ))}
          </div>
        </div>

        {config.consensus === "review_master" && (
          <div>
            <label style={labelStyle}>Review Master Agent Name</label>
            <input
              style={inputStyle}
              value={config.reviewMaster}
              onChange={(e) =>
                setConfig({ ...config, reviewMaster: e.target.value })
              }
              placeholder="Must match an agent name added in the next step"
            />
          </div>
        )}

        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
          <input
            type="checkbox"
            checked={config.autoMerge}
            onChange={(e) =>
              setConfig({ ...config, autoMerge: e.target.checked })
            }
            style={{ width: 18, height: 18 }}
          />
          <span>Auto-merge on completion</span>
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
