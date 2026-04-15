import { useState, useRef, useEffect } from "react";

interface HelpTooltipProps {
  text: string;
}

const tooltipKeyframes = `
@keyframes fc-tooltip-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}
`;

export function HelpTooltip({ text }: HelpTooltipProps) {
  const [visible, setVisible] = useState(false);
  const [above, setAbove] = useState(false);
  const iconRef = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    if (visible && iconRef.current) {
      const rect = iconRef.current.getBoundingClientRect();
      // If there's not enough room below, show above
      setAbove(rect.bottom + 120 > window.innerHeight);
    }
  }, [visible]);

  return (
    <span
      ref={iconRef}
      style={{ position: "relative", display: "inline-flex", alignItems: "center", marginLeft: 6 }}
      onMouseEnter={() => setVisible(true)}
      onMouseLeave={() => setVisible(false)}
    >
      <style>{tooltipKeyframes}</style>
      <svg
        width="14"
        height="14"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
        style={{ cursor: "help", opacity: 0.5, transition: "opacity 0.15s" }}
        onMouseEnter={(e) => (e.currentTarget.style.opacity = "0.85")}
        onMouseLeave={(e) => (e.currentTarget.style.opacity = "0.5")}
      >
        <circle cx="8" cy="8" r="7.5" stroke="currentColor" strokeWidth="1" />
        <text
          x="8"
          y="12"
          textAnchor="middle"
          fill="currentColor"
          fontSize="10"
          fontWeight="600"
          fontFamily="inherit"
        >
          ?
        </text>
      </svg>
      {visible && (
        <span
          role="tooltip"
          style={{
            position: "absolute",
            left: "50%",
            transform: "translateX(-50%)",
            ...(above
              ? { bottom: "calc(100% + 8px)" }
              : { top: "calc(100% + 8px)" }),
            background: "var(--bg-tertiary, #1a1a2e)",
            color: "var(--text-primary)",
            border: "1px solid var(--border)",
            borderRadius: 6,
            padding: "0.5rem 0.75rem",
            fontSize: "0.75rem",
            lineHeight: 1.4,
            whiteSpace: "normal",
            width: 220,
            zIndex: 1000,
            boxShadow: "0 4px 12px rgba(0,0,0,0.25)",
            animation: "fc-tooltip-in 0.15s ease-out",
            pointerEvents: "none",
          }}
        >
          {text}
        </span>
      )}
    </span>
  );
}
