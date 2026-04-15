interface MultiViewToggleProps {
  active: boolean;
  onToggle: () => void;
}

export function MultiViewToggle({ active, onToggle }: MultiViewToggleProps) {
  return (
    <button
      onClick={onToggle}
      aria-label="Multi-view"
      aria-pressed={active}
      title="Multi-view: show all agent terminals"
      style={{
        display: "inline-flex",
        alignItems: "center",
        gap: "0.35rem",
        padding: "0.3rem 0.6rem",
        borderRadius: 8,
        border: `1px solid ${active ? "var(--blue)" : "var(--border)"}`,
        background: active ? "var(--blue)" : "transparent",
        color: active ? "#fff" : "var(--text-secondary)",
        cursor: "pointer",
        fontSize: "0.75rem",
        fontFamily: "inherit",
        transition: "all 0.15s ease",
      }}
    >
      {/* 2x2 grid icon */}
      <svg
        width="14"
        height="14"
        viewBox="0 0 14 14"
        fill="none"
        aria-hidden="true"
      >
        <rect x="1" y="1" width="5" height="5" rx="1" fill="currentColor" />
        <rect x="8" y="1" width="5" height="5" rx="1" fill="currentColor" />
        <rect x="1" y="8" width="5" height="5" rx="1" fill="currentColor" />
        <rect x="8" y="8" width="5" height="5" rx="1" fill="currentColor" />
      </svg>
      Multi-view
    </button>
  );
}
