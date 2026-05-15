import { useState, useEffect, useRef } from "react";
import type { SquadronAgent, Persona } from "../../types";
import { getBranches } from "../../api";
import { SetupStep, isSetupComplete } from "./SetupStep";
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
    setConfig({ ...c, name: c.name.trim() });
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

  type BreadcrumbStep = Exclude<Step, "persona">;
  const steps: BreadcrumbStep[] = ["setup", "agents", "review"];
  const stepLabels: Record<Step, string> = { setup: "Setup", agents: "Agents", persona: "Persona", review: "Review" };
  const setupComplete = isSetupComplete(config);
  const agentsComplete = agents.length > 0;
  const stepEnabled: Record<BreadcrumbStep, boolean> = {
    setup: true,
    agents: setupComplete,
    review: setupComplete && agentsComplete,
  };
  // While editing a persona, treat the underlying step as Agents for breadcrumb highlighting.
  const breadcrumbStep: BreadcrumbStep = step === "persona" ? "agents" : step;
  const currentStepIndex = steps.indexOf(breadcrumbStep);

  const handleStepClick = (target: BreadcrumbStep) => {
    if (!stepEnabled[target]) return;
    if (target === step) return;
    setStep(target);
  };

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
          {steps.map((s, i) => {
            const isCurrent = breadcrumbStep === s;
            const enabled = stepEnabled[s];
            const isCompleted = i < currentStepIndex;
            return (
              <li
                key={s}
                aria-current={isCurrent ? "step" : undefined}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "0.5rem",
                }}
              >
                <StepButton
                  enabled={enabled}
                  isCurrent={isCurrent}
                  onClick={() => handleStepClick(s)}
                  label={stepLabels[s]}
                  index={i}
                  isCompleted={isCompleted}
                />
                {i < steps.length - 1 && (
                  <span style={{ color: "var(--text-muted)", marginLeft: "0.5rem" }} aria-hidden="true">
                    →
                  </span>
                )}
              </li>
            );
          })}
        </ol>
      </nav>

      <div ref={stepContentRef} tabIndex={-1} style={{ outline: "none" }} aria-live="polite">
        {step === "setup" && (
          <SetupStep
            initial={config}
            currentBranch={currentBranch}
            branches={branches}
            onDone={handleSetupDone}
            onChange={setConfig}
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
            onChange={setAgents}
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

interface StepButtonProps {
  enabled: boolean;
  isCurrent: boolean;
  onClick: () => void;
  label: string;
  index: number;
  isCompleted: boolean;
}

function StepButton({ enabled, isCurrent, onClick, label, index, isCompleted }: StepButtonProps) {
  const [hovered, setHovered] = useState(false);
  const interactive = enabled && !isCurrent;
  const cursor = !enabled ? "not-allowed" : isCurrent ? "default" : "pointer";

  let circleBg = "var(--bg-secondary)";
  let circleColor = "var(--text-muted)";
  if (isCurrent) {
    circleBg = "var(--blue)";
    circleColor = "#fff";
  } else if (interactive && hovered) {
    circleBg = "var(--blue)";
    circleColor = "#fff";
  }

  let labelColor: string = "var(--text-muted)";
  if (isCurrent) {
    labelColor = "var(--blue)";
  } else if (interactive && hovered) {
    labelColor = "var(--blue)";
  }

  return (
    <button
      type="button"
      onClick={onClick}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onFocus={() => setHovered(true)}
      onBlur={() => setHovered(false)}
      disabled={!interactive}
      aria-disabled={!enabled}
      aria-label={
        isCurrent
          ? `${label} (current step)`
          : isCompleted
          ? `Go to ${label} (completed)`
          : enabled
          ? `Go to ${label}`
          : `${label} (locked — complete previous step first)`
      }
      style={{
        display: "flex",
        alignItems: "center",
        gap: "0.5rem",
        background: "transparent",
        border: "none",
        padding: "0.25rem 0.5rem",
        borderRadius: 6,
        cursor,
        fontFamily: "inherit",
        fontSize: "inherit",
        fontWeight: isCurrent ? 600 : 400,
        color: labelColor,
        opacity: enabled ? 1 : 0.5,
        transition: "color 120ms ease, background 120ms ease",
        outline: "none",
      }}
    >
      <span
        aria-hidden="true"
        style={{
          width: 24,
          height: 24,
          borderRadius: "50%",
          background: circleBg,
          color: circleColor,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: "0.75rem",
          transition: "background 120ms ease, color 120ms ease",
        }}
      >
        {index + 1}
      </span>
      <span>
        {label}
        {isCompleted && <span className="sr-only"> (completed)</span>}
        {isCurrent && <span className="sr-only"> (current)</span>}
      </span>
    </button>
  );
}
