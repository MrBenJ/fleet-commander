import type React from "react";

export type ConsensusType = "universal" | "review_master" | "none";

export const consensusInfo: Record<ConsensusType, { icon: string; label: string; description: string }> = {
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

export const personaIcons: Record<string, string> = {
  "overconfident-engineer": "\u{1F680}",
  "zen-master": "\u{1F9D8}",
  "paranoid-perfectionist": "\u{1F50D}",
  "raging-jerk": "\u{1F624}",
  "peter-molyneux": "\u{1F3A9}",
};

export const driverColors: Record<string, string> = {
  "claude-code": "rgba(31,111,235,0.2)",
  codex: "rgba(46,160,67,0.2)",
  aider: "rgba(240,136,62,0.2)",
  "kimi-code": "rgba(167,139,250,0.2)",
  generic: "rgba(139,148,158,0.2)",
};

export const driverTextColors: Record<string, string> = {
  "claude-code": "var(--blue)",
  codex: "var(--green)",
  aider: "var(--orange)",
  "kimi-code": "#a78bfa",
  generic: "var(--text-secondary)",
};

export const inputStyle: React.CSSProperties = {
  background: "var(--bg-primary)",
  border: "1px solid var(--border)",
  borderRadius: 4,
  padding: "0.5rem",
  color: "var(--text-primary)",
  width: "100%",
  fontSize: "0.85rem",
};

export const labelStyle: React.CSSProperties = {
  color: "var(--text-secondary)",
  fontSize: "0.7rem",
  textTransform: "uppercase" as const,
  display: "block",
};
