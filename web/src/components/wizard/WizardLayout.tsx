import { useState, useEffect, useRef } from "react";
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
  ghAvailable: boolean;
  onLaunched: (name: string, agents: SquadronAgent[], config: { consensus: string; autoMerge: boolean; mergeMaster?: string }) => void;
}

interface SquadronConfig {
  name: string;
  baseBranch: string;
}

type Step = "setup" | "agents" | "persona" | "review";

export function WizardLayout({
  personas,
  drivers,
  currentBranch,
  ghAvailable,
  onLaunched,
}: WizardProps) {
  const [step, setStep] = useState<Step>("setup");
  const [config, setConfig] = useState<SquadronConfig>({
    name: "",
    baseBranch: currentBranch,
  });
  const [agents, setAgents] = useState<SquadronAgent[]>([]);
  const [editingAgentIdx, setEditingAgentIdx] = useState<number | null>(null);
  const [branches, setBranches] = useState<string[]>([]);
  const stepContentRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    getBranches().then(setBranches).catch(() => {});
  }, []);

  useEffect(() => {
    stepContentRef.current?.focus();
  }, [step]);

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

  const handlePersonaSelected = (personaName: string, fightMode: boolean) => {
    if (editingAgentIdx !== null) {
      const updated = [...agents];
      updated[editingAgentIdx] = {
        ...updated[editingAgentIdx],
        persona: personaName,
        fightMode,
      };
      setAgents(updated);
    }
    setStep("agents");
  };

  const steps: Step[] = ["setup", "agents", "review"];
  const stepLabels = { setup: "Setup", agents: "Agents", persona: "Persona", review: "Review" };
  const currentStepIndex = steps.indexOf(step);

  return (
    <div style={{ maxWidth: 900, margin: "0 auto", padding: "2rem" }}>
      {/* Progress indicator */}
      <nav aria-label="Wizard progress">
        <ol
          style={{
            display: "flex",
            gap: "1rem",
            marginBottom: "2rem",
            justifyContent: "center",
            listStyle: "none",
            padding: 0,
          }}
        >
          {steps.map((s, i) => (
            <li
              key={s}
              aria-current={step === s ? "step" : undefined}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                color: step === s ? "var(--blue)" : "var(--text-muted)",
                fontWeight: step === s ? 600 : 400,
              }}
            >
              <span
                aria-hidden="true"
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
              <span>
                {stepLabels[s]}
                {i < currentStepIndex && <span className="sr-only"> (completed)</span>}
                {step === s && <span className="sr-only"> (current)</span>}
              </span>
              {i < steps.length - 1 && (
                <span style={{ color: "var(--text-muted)", marginLeft: "0.5rem" }} aria-hidden="true">
                  →
                </span>
              )}
            </li>
          ))}
        </ol>
      </nav>

      <div ref={stepContentRef} tabIndex={-1} style={{ outline: "none" }} aria-live="polite">
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
            initialFightMode={editingAgentIdx !== null ? agents[editingAgentIdx]?.fightMode ?? false : false}
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
            ghAvailable={ghAvailable}
            onLaunched={onLaunched}
            onEdit={() => setStep("agents")}
            onAddMore={() => setStep("agents")}
            onAgentsChanged={(a) => setAgents(a)}
          />
        )}
      </div>
    </div>
  );
}
