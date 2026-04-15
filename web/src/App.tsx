import { useState } from "react";
import { useFleet } from "./hooks/useFleet";
import { WizardLayout } from "./components/wizard/WizardLayout";
import { MissionControl } from "./components/mission/MissionControl";
import { TerminalPage } from "./components/terminal/TerminalPage";
import { ThemeToggle } from "./components/ThemeToggle";
import type { SquadronAgent } from "./types";

type View = "wizard" | "mission";

export function App() {
  // If we're on a /terminal/ path, render the terminal directly
  if (window.location.pathname.startsWith("/terminal/")) {
    return (
      <>
        <ThemeToggle />
        <TerminalPage />
      </>
    );
  }

  const { fleet, personas, drivers, loading, error } = useFleet();
  const [view, setView] = useState<View>("wizard");
  const [activeSquadron, setActiveSquadron] = useState<string | null>(null);
  const [launchedAgents, setLaunchedAgents] = useState<SquadronAgent[]>([]);
  const [launchConfig, setLaunchConfig] = useState<{
    consensus: string;
    autoMerge: boolean;
  }>({ consensus: "universal", autoMerge: true });

  if (loading) {
    return (
      <main style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100vh" }}>
        <div role="status" aria-live="polite" style={{ color: "var(--text-secondary)" }}>Loading fleet...</div>
      </main>
    );
  }

  if (error) {
    return (
      <main style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100vh" }}>
        <div role="alert" style={{ color: "var(--red)" }}>Error: {error}</div>
      </main>
    );
  }

  const handleLaunched = (
    name: string,
    agents: SquadronAgent[],
    config: { consensus: string; autoMerge: boolean }
  ) => {
    setActiveSquadron(name);
    setLaunchedAgents(agents);
    setLaunchConfig(config);
    setView("mission");
  };

  return (
    <div style={{ minHeight: "100vh" }}>
      <a href="#main-content" className="skip-nav">Skip to main content</a>
      <ThemeToggle />
      <main id="main-content">
        {view === "wizard" && fleet && (
          <WizardLayout
            personas={personas}
            drivers={drivers}
            currentBranch={fleet.currentBranch}
            onLaunched={handleLaunched}
          />
        )}
        {view === "mission" && activeSquadron && (
          <MissionControl
            squadronName={activeSquadron}
            agents={launchedAgents}
            personas={personas}
            consensus={launchConfig.consensus}
            autoMerge={launchConfig.autoMerge}
          />
        )}
      </main>
    </div>
  );
}
