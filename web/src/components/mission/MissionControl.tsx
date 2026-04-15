import { useState, useCallback, useMemo, useEffect } from "react";
import type { ContextMessage, SquadronAgent, Persona, WSEvent } from "../../types";
import { useWebSocket } from "../../hooks/useWebSocket";
import { getFleet } from "../../api";
import { AgentPill } from "./AgentPill";
import { AgentTooltip } from "./AgentTooltip";
import { ContextLog } from "./ContextLog";
import { MultiViewToggle } from "./MultiViewToggle";
import { MultiView } from "./MultiView";

interface MissionControlProps {
  squadronName: string;
  agents: SquadronAgent[];
  personas: Persona[];
  consensus: string;
  autoMerge: boolean;
}

const agentColorPalette = [
  "var(--blue)",
  "var(--green)",
  "var(--purple)",
  "var(--orange)",
  "#f778ba",
  "#79c0ff",
];

export function MissionControl({
  squadronName,
  agents,
  personas,
  consensus,
  autoMerge,
}: MissionControlProps) {
  const [messages, setMessages] = useState<ContextMessage[]>([]);
  const [agentStates, setAgentStates] = useState<Record<string, string>>({});
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);
  const [multiView, setMultiView] = useState(false);

  const agentColors = useMemo(() => {
    const colors: Record<string, string> = {};
    agents.forEach((a, i) => {
      colors[a.name] = agentColorPalette[i % agentColorPalette.length];
    });
    return colors;
  }, [agents]);

  const handleEvent = useCallback((event: WSEvent) => {
    switch (event.type) {
      case "context_message":
        setMessages((prev) => [
          ...prev,
          { agent: event.agent, message: event.message, timestamp: event.timestamp },
        ]);
        break;
      case "agent_state":
        setAgentStates((prev) => ({ ...prev, [event.agent]: event.state }));
        break;
      case "agent_stopped":
        setAgentStates((prev) => ({ ...prev, [event.agent]: "stopped" }));
        break;
    }
  }, []);

  const { connected } = useWebSocket("/ws/events", { onEvent: handleEvent });

  // Seed initial agent states from the REST API so dots aren't gray on load.
  useEffect(() => {
    getFleet().then((info) => {
      const states: Record<string, string> = {};
      for (const a of info.agents) {
        if (a.status) states[a.name] = a.status;
      }
      setAgentStates((prev) => ({ ...states, ...prev }));
    }).catch(() => {});
  }, []);

  const selectedAgentData = agents.find((a) => a.name === selectedAgent);
  const selectedPersona = selectedAgentData
    ? personas.find((p) => p.name === selectedAgentData.persona)
    : undefined;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh" }}>
      {/* Top bar */}
      <header
        style={{
          background: "var(--bg-secondary)",
          borderBottom: "1px solid var(--border)",
          padding: "0.75rem 1.5rem",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          <h1 style={{ fontWeight: 700, fontSize: "1.1rem", margin: 0 }}>
            {squadronName}
          </h1>
          <span
            role="status"
            aria-live="polite"
            style={{
              fontSize: "0.75rem",
              background: connected ? "var(--green)" : "var(--red)",
              color: "#fff",
              padding: "0.2rem 0.6rem",
              borderRadius: 10,
            }}
          >
            {connected ? "ACTIVE" : "DISCONNECTED"}
          </span>
          <span style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>
            {consensus} consensus · auto-merge {autoMerge ? "on" : "off"}
          </span>
        </div>
        <div style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>
          {agents.length} agents
        </div>
      </header>

      {/* Agent pills */}
      <nav
        aria-label="Squadron agents"
        style={{
          borderBottom: "1px solid var(--border)",
          padding: "0.6rem 1.5rem",
          display: "flex",
          gap: "0.75rem",
          alignItems: "center",
        }}
      >
        <MultiViewToggle
          active={multiView}
          onToggle={() => setMultiView((v) => !v)}
        />
        <span
          style={{
            width: 1,
            height: 16,
            background: "var(--border)",
          }}
        />
        <span style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }} id="agents-label">
          Agents:
        </span>
        <div role="group" aria-labelledby="agents-label" style={{ display: "flex", gap: "0.75rem" }}>
          {agents.map((a) => (
            <AgentPill
              key={a.name}
              name={a.name}
              state={agentStates[a.name] || "starting"}
              driver={a.driver}
              onClick={() => setSelectedAgent(a.name)}
            />
          ))}
        </div>
      </nav>

      {/* Main content: context log or multi-view */}
      {multiView ? (
        <MultiView agents={agents} />
      ) : (
        <ContextLog
          messages={messages}
          agentColors={agentColors}
          onAgentClick={(name) => setSelectedAgent(name)}
        />
      )}

      {/* Agent tooltip modal */}
      {selectedAgent && selectedAgentData && (
        <AgentTooltip
          agent={selectedAgentData}
          state={agentStates[selectedAgent] || "starting"}
          persona={selectedPersona}
          onClose={() => setSelectedAgent(null)}
        />
      )}

      {/* Inject pulse animation */}
      <style>{`@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }`}</style>
    </div>
  );
}
