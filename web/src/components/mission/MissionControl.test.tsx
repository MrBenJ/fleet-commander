import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MissionControl } from "./MissionControl";
import type { SquadronAgent, Persona, WSEvent } from "../../types";

// Capture the onEvent callback so tests can simulate WebSocket events
let capturedOnEvent: ((event: WSEvent) => void) | undefined;
let mockConnected = true;

vi.mock("../../hooks/useWebSocket", () => ({
  useWebSocket: (_path: string, options: { onEvent?: (e: WSEvent) => void }) => {
    capturedOnEvent = options.onEvent;
    return { connected: mockConnected, ws: { current: null } };
  },
}));

const agents: SquadronAgent[] = [
  { name: "alpha", branch: "feat/alpha", prompt: "Do alpha work", driver: "claude-code", persona: "zen-master" },
  { name: "beta", branch: "feat/beta", prompt: "Do beta work", driver: "codex", persona: "raging-jerk" },
];

const personas: Persona[] = [
  { name: "zen-master", displayName: "Zen Master", preamble: "You are calm." },
  { name: "raging-jerk", displayName: "Raging Jerk", preamble: "You are angry." },
];

function renderMission(overrides: Partial<Parameters<typeof MissionControl>[0]> = {}) {
  return render(
    <MissionControl
      squadronName="partyboiz"
      agents={agents}
      personas={personas}
      consensus="universal"
      autoMerge={false}
      {...overrides}
    />
  );
}

describe("MissionControl", () => {
  beforeEach(() => {
    capturedOnEvent = undefined;
    mockConnected = true;
  });

  it("renders squadron name and connection status", () => {
    renderMission();
    expect(screen.getByText("partyboiz")).toBeInTheDocument();
    expect(screen.getByText("ACTIVE")).toBeInTheDocument();
  });

  it("shows DISCONNECTED when not connected", () => {
    mockConnected = false;
    renderMission();
    expect(screen.getByText("DISCONNECTED")).toBeInTheDocument();
  });

  it("renders consensus and auto-merge info", () => {
    renderMission();
    expect(screen.getByText("universal consensus · auto-merge off")).toBeInTheDocument();
  });

  it("shows auto-merge on when enabled", () => {
    renderMission({ autoMerge: true });
    expect(screen.getByText("universal consensus · auto-merge on")).toBeInTheDocument();
  });

  it("renders agent count", () => {
    renderMission();
    expect(screen.getByText("2 agents")).toBeInTheDocument();
  });

  it("renders agent pills for each agent", () => {
    renderMission();
    expect(screen.getByRole("button", { name: /alpha/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /beta/ })).toBeInTheDocument();
  });

  it("shows agent tooltip on hover over an agent pill", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.hover(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByText("Assume Control")).toBeInTheDocument();
  });

  it("hides agent tooltip when mouse leaves the pill", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.hover(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByText("Assume Control")).toBeInTheDocument();

    await user.unhover(screen.getByRole("button", { name: /alpha/ }));
    // Tooltip hides after a 200ms delay
    await vi.waitFor(() => {
      expect(screen.queryByText("Assume Control")).not.toBeInTheDocument();
    });
  });

  it("displays context messages when received via WebSocket", () => {
    renderMission();
    expect(capturedOnEvent).toBeDefined();

    act(() => {
      capturedOnEvent!({
        type: "context_message",
        agent: "alpha",
        message: "Starting work on feature",
        timestamp: "2026-04-14T10:30:00Z",
      });
    });

    expect(screen.getByText("Starting work on feature")).toBeInTheDocument();
  });

  it("updates agent states from agent_state events", () => {
    renderMission();

    act(() => {
      capturedOnEvent!({
        type: "agent_state",
        agent: "alpha",
        state: "working",
        timestamp: "2026-04-14T10:30:00Z",
      });
    });

    expect(screen.getByRole("button", { name: /alpha, status: working/ })).toBeInTheDocument();
  });

  it("marks agent as stopped from agent_stopped events", () => {
    renderMission();

    act(() => {
      capturedOnEvent!({ type: "agent_stopped", agent: "beta" });
    });

    expect(screen.getByRole("button", { name: /beta, status: stopped/ })).toBeInTheDocument();
  });

  it("defaults agent state to starting when no state event received", () => {
    renderMission();
    expect(screen.getByRole("button", { name: /alpha, status: starting/ })).toBeInTheDocument();
  });

  it("renders with empty agents array", () => {
    renderMission({ agents: [] });
    expect(screen.getByText("0 agents")).toBeInTheDocument();
  });

  it("renders the context log area", () => {
    renderMission();
    expect(screen.getByRole("log", { name: /context messages/ })).toBeInTheDocument();
  });

  it("shows agent tooltip with persona info on hover", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.hover(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByText("Zen Master")).toBeInTheDocument();
  });

  it("renders the agents navigation section", () => {
    renderMission();
    expect(screen.getByRole("navigation", { name: /squadron agents/i })).toBeInTheDocument();
  });

  it("renders MERGE badge on the merge master agent", () => {
    renderMission({ autoMerge: true, mergeMaster: "alpha" });
    expect(screen.getByText("MERGE")).toBeInTheDocument();
  });

  it("does not render MERGE badge when no mergeMaster is set", () => {
    renderMission({ autoMerge: true });
    expect(screen.queryByText("MERGE")).not.toBeInTheDocument();
  });

  it("does not render MERGE badge on non-merger agents", () => {
    renderMission({ autoMerge: true, mergeMaster: "alpha" });
    // Only one MERGE badge should exist (on alpha, not beta)
    const badges = screen.getAllByText("MERGE");
    expect(badges).toHaveLength(1);
  });

  it("renders multi-view toggle in the agents nav bar", () => {
    renderMission();
    expect(screen.getByRole("button", { name: /multi-view/i })).toBeInTheDocument();
  });

  it("shows context log by default, not multi-view", () => {
    renderMission();
    expect(screen.getByRole("log", { name: /context messages/ })).toBeInTheDocument();
    expect(screen.queryByTestId("multi-view-grid")).not.toBeInTheDocument();
  });

  it("switches to multi-view when toggle is clicked", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.click(screen.getByRole("button", { name: /multi-view/i }));
    expect(screen.getByTestId("multi-view-grid")).toBeInTheDocument();
    expect(screen.queryByRole("log", { name: /context messages/ })).not.toBeInTheDocument();
  });

  it("switches back to context log when toggle is clicked again", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.click(screen.getByRole("button", { name: /multi-view/i }));
    expect(screen.getByTestId("multi-view-grid")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /multi-view/i }));
    expect(screen.getByRole("log", { name: /context messages/ })).toBeInTheDocument();
    expect(screen.queryByTestId("multi-view-grid")).not.toBeInTheDocument();
  });
});
