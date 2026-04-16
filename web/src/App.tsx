import { useState, useEffect } from "react";
import { useFleet } from "./hooks/useFleet";
import { WizardLayout } from "./components/wizard/WizardLayout";
import { MissionControl } from "./components/mission/MissionControl";
import { TerminalPage } from "./components/terminal/TerminalPage";
import { ThemeToggle } from "./components/ThemeToggle";
import { getSquadronInfo } from "./api";
import type { SquadronAgent } from "./types";
import type { SquadronInfoResponse } from "./api";

type View = "wizard" | "mission";

function getSquadronParam(): string | null {
  const params = new URLSearchParams(window.location.search);
  return params.get("squadron");
}

export function App() {
  // If we're on a /terminal/ path, render the terminal directly
  if (window.location.pathname.startsWith("/terminal/")) {
    const inIframe = window.self !== window.top;
    return (
      <>
        {!inIframe && <ThemeToggle />}
        <TerminalPage />
      </>
    );
  }

  const squadronParam = getSquadronParam();
  const { fleet, personas, drivers, loading, error } = useFleet();
  const [view, setView] = useState<View>(squadronParam ? "mission" : "wizard");
  const [activeSquadron, setActiveSquadron] = useState<string | null>(squadronParam);
  const [launchedAgents, setLaunchedAgents] = useState<SquadronAgent[]>([]);
  const [launchConfig, setLaunchConfig] = useState<{
    consensus: string;
    autoMerge: boolean;
    mergeMaster?: string;
  }>({ consensus: "universal", autoMerge: true });
  const [controlLoading, setControlLoading] = useState(!!squadronParam);
  const [controlError, setControlError] = useState<string | null>(null);

  useEffect(() => {
    if (!squadronParam) return;
    getSquadronInfo(squadronParam)
      .then((info: SquadronInfoResponse) => {
        setActiveSquadron(info.name);
        setLaunchedAgents(info.agents);
        setLaunchConfig({
          consensus: info.consensus,
          autoMerge: info.autoMerge,
        });
        setView("mission");
        setControlLoading(false);
      })
      .catch((err: Error) => {
        setControlError(err.message);
        setControlLoading(false);
      });
  }, [squadronParam]);

  if (controlLoading || loading) {
    return (
      <main style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100vh" }}>
        <div role="status" aria-live="polite" style={{ color: "var(--text-secondary)" }}>
          {controlLoading ? `Loading squadron "${squadronParam}"...` : "Loading fleet..."}
        </div>
      </main>
    );
  }

  if (controlError) {
    return (
      <main style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "100vh" }}>
        <div role="alert" style={{ color: "var(--red)", textAlign: "center", maxWidth: 500 }}>
          <h2 style={{ margin: "0 0 0.5rem" }}>Squadron not found</h2>
          <p>Could not load squadron "{squadronParam}": {controlError}</p>
          <p style={{ color: "var(--text-secondary)", fontSize: "0.85rem" }}>
            Make sure the squadron has been launched previously.
          </p>
        </div>
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
    config: { consensus: string; autoMerge: boolean; mergeMaster?: string }
  ) => {
    setActiveSquadron(name);
    setLaunchedAgents(agents);
    setLaunchConfig(config);
    setView("mission");
  };

  return (
    <div style={{ minHeight: "100vh" }}>
      <ThemeToggle />
      <a href="#main-content" className="skip-nav">Skip to main content</a>
      <main id="main-content">
        {view === "wizard" && fleet && (
          <WizardLayout
            personas={personas}
            drivers={drivers}
            currentBranch={fleet.currentBranch}
            ghAvailable={fleet.ghAvailable ?? false}
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
            mergeMaster={launchConfig.mergeMaster}
          />
        )}
      </main>
    </div>
  );
}
