interface CostToggleProps {
  active: boolean;
  onToggle: () => void;
}

export function CostToggle({ active, onToggle }: CostToggleProps) {
  return (
    <button
      onClick={onToggle}
      aria-label="Toggle cost"
      aria-pressed={active}
      title={active ? "Hide cost meters" : "Show cost meters"}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        width: 30,
        height: 28,
        borderRadius: 8,
        border: `1px solid ${active ? "var(--green)" : "var(--border)"}`,
        background: active ? "var(--green)" : "rgba(255,255,255,0.05)",
        color: active ? "#fff" : "var(--text-secondary)",
        cursor: "pointer",
        fontSize: "0.85rem",
        fontWeight: 700,
        fontFamily: "inherit",
        lineHeight: 1,
        transition: "all 0.15s ease",
      }}
    >
      $
    </button>
  );
}
