import { useState, useCallback, useMemo } from "react";
import type { ContextMessage, SquadronAgent, Persona, WSEvent } from "../../types";
import { useWebSocket } from "../../hooks/useWebSocket";
import { AgentPill } from "./AgentPill";
import { AgentTooltip } from "./AgentTooltip";
import { ContextLog } from "./ContextLog";

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

  const selectedAgentData = agents.find((a) => a.name === selectedAgent);
  const selectedPersona = selectedAgentData
    ? personas.find((p) => p.name === selectedAgentData.persona)
    : undefined;

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100vh" }}>
      {/* Top bar */}
      <div
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
          <span style={{ fontWeight: 700, fontSize: "1.1rem" }}>
            {squadronName}
          </span>
          <span
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
      </div>

      {/* Agent pills */}
      <div
        style={{
          borderBottom: "1px solid var(--border)",
          padding: "0.6rem 1.5rem",
          display: "flex",
          gap: "0.75rem",
          alignItems: "center",
        }}
      >
        <span style={{ fontSize: "0.75rem", color: "var(--text-secondary)" }}>
          Agents:
        </span>
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

      {/* Context log */}
      <ContextLog
        messages={messages}
        agentColors={agentColors}
        onAgentClick={(name) => setSelectedAgent(name)}
      />

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
