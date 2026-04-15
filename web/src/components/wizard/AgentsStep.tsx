import { useState } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { AIGeneratePanel } from "./AIGeneratePanel";
import { ManualAddForm } from "./ManualAddForm";
import { AgentListItem } from "./AgentListItem";
import { CSVUpload } from "./CSVUpload";

interface AgentsStepProps {
  squadronName: string;
  agents: SquadronAgent[];
  drivers: string[];
  personas: Persona[];
  onDone: (agents: SquadronAgent[]) => void;
  onPickPersona: (idx: number, currentAgents: SquadronAgent[]) => void;
}

export function AgentsStep({
  squadronName,
  agents: initialAgents,
  drivers,
  personas,
  onDone,
  onPickPersona,
}: AgentsStepProps) {
  const [agents, setAgents] = useState<SquadronAgent[]>(initialAgents);

  const handleAgentsGenerated = (newAgents: SquadronAgent[]) => {
    setAgents((prev) => [...prev, ...newAgents]);
  };

  const handleAgentAdded = (agent: SquadronAgent) => {
    setAgents((prev) => [...prev, agent]);
  };

  const handleCSVAgents = (newAgents: SquadronAgent[]) => {
    setAgents((prev) => [...prev, ...newAgents]);
  };

  const handleRemove = (idx: number) => {
    setAgents((prev) => prev.filter((_, i) => i !== idx));
  };

  return (
    <div>
      <h2 style={{ marginBottom: "0.5rem" }}>Add Agents</h2>
      <p style={{ color: "var(--text-secondary)", marginBottom: "2rem" }}>
        Assemble your squad's agents
      </p>

      <CSVUpload
        squadronName={squadronName}
        onAgentsParsed={handleCSVAgents}
      />

      <div style={{ display: "flex", gap: "1.5rem", marginBottom: "2rem" }}>
        <AIGeneratePanel
          squadronName={squadronName}
          onAgentsGenerated={handleAgentsGenerated}
        />
        <ManualAddForm
          squadronName={squadronName}
          drivers={drivers}
          personas={personas}
          onAgentAdded={handleAgentAdded}
        />
      </div>

      {agents.length > 0 && (
        <div style={{ marginBottom: "1.5rem" }}>
          <h3 style={{ marginBottom: "0.75rem" }}>Agents ({agents.length})</h3>
          <ul style={{ display: "flex", flexDirection: "column", gap: "0.5rem", listStyle: "none", padding: 0 }} aria-label="Agent list">
            {agents.map((a, idx) => (
              <AgentListItem
                key={idx}
                agent={a}
                personas={personas}
                onRemove={() => handleRemove(idx)}
                onPickPersona={() => onPickPersona(idx, agents)}
              />
            ))}
          </ul>
        </div>
      )}

      <button
        onClick={() => onDone(agents)}
        disabled={agents.length === 0}
        aria-disabled={agents.length === 0}
        style={{
          background: agents.length > 0 ? "var(--green)" : "var(--bg-secondary)",
          color: agents.length > 0 ? "#fff" : "var(--text-muted)",
          border: "none",
          borderRadius: 8,
          padding: "0.75rem 2rem",
          fontSize: "1rem",
          fontWeight: 600,
          cursor: agents.length > 0 ? "pointer" : "default",
        }}
      >
        Continue to Review →
      </button>
    </div>
  );
}
