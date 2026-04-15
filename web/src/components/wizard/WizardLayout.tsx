import { useState, useEffect } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { getBranches } from "../../api";
import { SetupStep } from "./SetupStep";
import { AgentsStep } from "./AgentsStep";
import { PersonaStep } from "./PersonaStep";
import { ReviewStep } from "./ReviewStep";

interface WizardProps {
  personas: Persona[];
  drivers: string[];
  currentBranch: string;
  onLaunched: (name: string, agents: SquadronAgent[], config: { consensus: string; autoMerge: boolean }) => void;
}

interface SquadronConfig {
  name: string;
  baseBranch: string;
  consensus: "universal" | "review_master" | "none";
  reviewMaster: string;
  autoMerge: boolean;
}

type Step = "setup" | "agents" | "persona" | "review";

export function WizardLayout({
  personas,
  drivers,
  currentBranch,
  onLaunched,
}: WizardProps) {
  const [step, setStep] = useState<Step>("setup");
  const [config, setConfig] = useState<SquadronConfig>({
    name: "",
    baseBranch: currentBranch,
    consensus: "universal",
    reviewMaster: "",
    autoMerge: true,
  });
  const [agents, setAgents] = useState<SquadronAgent[]>([]);
  const [editingAgentIdx, setEditingAgentIdx] = useState<number | null>(null);
  const [branches, setBranches] = useState<string[]>([]);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => {});
  }, []);

  const handleSetupDone = (c: SquadronConfig) => {
    setConfig(c);
    setStep("agents");
  };

  const handleAgentsDone = (a: SquadronAgent[]) => {
    setAgents(a);
    setStep("review");
  };

  const handlePickPersona = (idx: number, currentAgents: SquadronAgent[]) => {
    setAgents(currentAgents);
    setEditingAgentIdx(idx);
    setStep("persona");
  };

  const handlePersonaSelected = (personaName: string) => {
    if (editingAgentIdx !== null) {
      const updated = [...agents];
      updated[editingAgentIdx] = {
        ...updated[editingAgentIdx],
        persona: personaName,
      };
      setAgents(updated);
    }
    setStep("agents");
  };

  const steps: Step[] = ["setup", "agents", "review"];
  const stepLabels = { setup: "Setup", agents: "Agents", persona: "Persona", review: "Review" };

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "2rem" }}>
      {/* Progress indicator */}
      <div
        style={{
          display: "flex",
          gap: "1rem",
          marginBottom: "2rem",
          justifyContent: "center",
        }}
      >
        {steps.map((s, i) => (
          <div
            key={s}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              color: step === s ? "var(--blue)" : "var(--text-muted)",
              fontWeight: step === s ? 600 : 400,
            }}
          >
            <span
              style={{
                width: 24,
                height: 24,
                borderRadius: "50%",
                background:
                  step === s ? "var(--blue)" : "var(--bg-secondary)",
                color: step === s ? "#fff" : "var(--text-muted)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: "0.75rem",
              }}
            >
              {i + 1}
            </span>
            {stepLabels[s]}
            {i < steps.length - 1 && (
              <span style={{ color: "var(--text-muted)", marginLeft: "0.5rem" }}>
                →
              </span>
            )}
          </div>
        ))}
      </div>

      {step === "setup" && (
        <SetupStep
          initial={config}
          currentBranch={currentBranch}
          branches={branches}
          onDone={handleSetupDone}
        />
      )}
      {step === "agents" && (
        <AgentsStep
          squadronName={config.name}
          agents={agents}
          drivers={drivers}
          personas={personas}
          onDone={handleAgentsDone}
          onPickPersona={handlePickPersona}
        />
      )}
      {step === "persona" && (
        <PersonaStep
          personas={personas}
          onSelect={handlePersonaSelected}
          onCancel={() => setStep("agents")}
        />
      )}
      {step === "review" && (
        <ReviewStep
          config={config}
          agents={agents}
          drivers={drivers}
          personas={personas}
          onLaunched={onLaunched}
          onEdit={() => setStep("agents")}
          onAddMore={() => setStep("agents")}
          onAgentsChanged={(a) => setAgents(a)}
        />
      )}
    </div>
  );
}
