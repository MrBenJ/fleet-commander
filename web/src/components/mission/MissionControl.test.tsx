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

  it("opens agent tooltip when an agent pill is clicked", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.click(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByRole("dialog", { name: /alpha/ })).toBeInTheDocument();
  });

  it("closes agent tooltip when backdrop is clicked", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.click(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();

    // Click the backdrop
    const backdrop = screen.getByRole("presentation");
    await user.click(backdrop);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
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

  it("shows agent tooltip with persona info", async () => {
    const user = userEvent.setup();
    renderMission();
    await user.click(screen.getByRole("button", { name: /alpha/ }));
    expect(screen.getByText("Zen Master")).toBeInTheDocument();
  });

  it("renders the agents navigation section", () => {
    renderMission();
    expect(screen.getByRole("navigation", { name: /squadron agents/i })).toBeInTheDocument();
  });
});
