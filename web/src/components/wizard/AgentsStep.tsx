import { useState } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { AIGeneratePanel } from "./AIGeneratePanel";
import { ManualAddForm } from "./ManualAddForm";
import { AgentListItem } from "./AgentListItem";
import { CSVUpload } from "./CSVUpload";
import { HelpTooltip } from "../common/HelpTooltip";

interface AgentsStepProps {
  squadronName: string;
  agents: SquadronAgent[];
  drivers: string[];
  personas: Persona[];
  onDone: (agents: SquadronAgent[]) => void;
  onPickPersona: (idx: number, currentAgents: SquadronAgent[]) => void;
  onChange?: (agents: SquadronAgent[]) => void;
}

export function AgentsStep({
  squadronName,
  agents: initialAgents,
  drivers,
  personas,
  onDone,
  onPickPersona,
  onChange,
}: AgentsStepProps) {
  const [agents, setAgents] = useState<SquadronAgent[]>(initialAgents);
  const [squadronFightMode, setSquadronFightMode] = useState<boolean>(
    initialAgents.length > 0 && initialAgents.every((a) => a.fightMode),
  );

  const applyFightMode = (a: SquadronAgent): SquadronAgent =>
    squadronFightMode ? { ...a, fightMode: true } : a;

  const updateAgents = (updater: (prev: SquadronAgent[]) => SquadronAgent[]) => {
    setAgents((prev) => {
      const next = updater(prev);
      onChange?.(next);
      return next;
    });
  };

  const handleAgentsGenerated = (newAgents: SquadronAgent[]) => {
    updateAgents((prev) => [...prev, ...newAgents.map(applyFightMode)]);
  };

  const handleAgentAdded = (agent: SquadronAgent) => {
    updateAgents((prev) => [...prev, applyFightMode(agent)]);
  };

  const handleCSVAgents = (newAgents: SquadronAgent[]) => {
    updateAgents((prev) => [...prev, ...newAgents.map(applyFightMode)]);
  };

  const handleRemove = (idx: number) => {
    updateAgents((prev) => prev.filter((_, i) => i !== idx));
  };

  const handleFightModeToggle = (checked: boolean) => {
    setSquadronFightMode(checked);
    updateAgents((prev) => prev.map((a) => ({ ...a, fightMode: checked })));
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

      <div style={{ display: "flex", alignItems: "center", gap: "0.75rem", marginBottom: "1.5rem" }}>
        <input
          type="checkbox"
          id="squadron-fight-mode"
          checked={squadronFightMode}
          onChange={(e) => handleFightModeToggle(e.target.checked)}
          style={{ width: 18, height: 18 }}
        />
        <label htmlFor="squadron-fight-mode">
          <span aria-hidden="true">🥊</span> Fight mode for the whole squadron
          <HelpTooltip text="If checked, every agent in this squadron will humorously roast and fight each other while working and speak in their persona's voice." />
        </label>
      </div>

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
