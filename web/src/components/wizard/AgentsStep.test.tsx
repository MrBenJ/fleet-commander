import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentsStep } from "./AgentsStep";
import type { Persona, SquadronAgent } from "../../types";

vi.mock("../../api", () => ({
  generateAgents: vi.fn(),
  getAvailableDrivers: vi.fn(() => Promise.resolve([
    { name: "claude-code", available: true },
    { name: "codex", available: false },
  ])),
}));

describe("AgentsStep", () => {
  const personas: Persona[] = [
    { name: "zen-master", displayName: "Zen Master", preamble: "" },
  ];

  const defaultProps = {
    squadronName: "test-squad",
    agents: [] as SquadronAgent[],
    drivers: ["claude-code", "aider"],
    personas,
    onDone: vi.fn(),
    onPickPersona: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the heading and both panels", () => {
    render(<AgentsStep {...defaultProps} />);
    expect(screen.getByText("Add Agents")).toBeInTheDocument();
    expect(screen.getByText("AI Generate from Description")).toBeInTheDocument();
    expect(screen.getByText("Add Manually")).toBeInTheDocument();
  });

  it("disables continue button when no agents", () => {
    render(<AgentsStep {...defaultProps} />);
    const button = screen.getByRole("button", { name: /continue to review/i });
    expect(button).toBeDisabled();
  });

  it("enables continue button when agents exist", () => {
    const agents: SquadronAgent[] = [
      { name: "test", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} />);
    const button = screen.getByRole("button", { name: /continue to review/i });
    expect(button).not.toBeDisabled();
  });

  it("shows agent list when agents are provided", () => {
    const agents: SquadronAgent[] = [
      { name: "auth-agent", branch: "b", prompt: "Build auth", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} />);
    expect(screen.getByText("auth-agent")).toBeInTheDocument();
    expect(screen.getByText("Agents (1)")).toBeInTheDocument();
  });

  it("adds an agent via manual form", async () => {
    const user = userEvent.setup();
    render(<AgentsStep {...defaultProps} />);

    await user.type(screen.getByLabelText("Agent Name"), "new-agent");
    await user.type(screen.getByLabelText("Prompt"), "Do work");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(screen.getByText("new-agent")).toBeInTheDocument();
    expect(screen.getByText("Agents (1)")).toBeInTheDocument();
  });

  it("removes an agent when remove button is clicked", async () => {
    const user = userEvent.setup();
    const agents: SquadronAgent[] = [
      { name: "test-agent", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} />);

    expect(screen.getByText("test-agent")).toBeInTheDocument();
    await user.click(screen.getByLabelText("Remove agent test-agent"));
    expect(screen.queryByText("test-agent")).not.toBeInTheDocument();
  });

  it("calls onDone with agents when continue is clicked", async () => {
    const user = userEvent.setup();
    const onDone = vi.fn();
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} onDone={onDone} />);

    await user.click(screen.getByRole("button", { name: /continue to review/i }));
    expect(onDone).toHaveBeenCalledWith(agents);
  });

  it("calls onPickPersona with index and current agents", async () => {
    const user = userEvent.setup();
    const onPickPersona = vi.fn();
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} onPickPersona={onPickPersona} />);

    await user.click(screen.getByLabelText("Select persona for a1"));
    expect(onPickPersona).toHaveBeenCalledWith(0, agents);
  });

  it("does not show agent list section when empty", () => {
    render(<AgentsStep {...defaultProps} />);
    expect(screen.queryByText(/^Agents \(/)).not.toBeInTheDocument();
  });

  it("renders the squadron fight mode toggle above continue button", () => {
    render(<AgentsStep {...defaultProps} />);
    expect(screen.getByLabelText(/fight mode for the whole squadron/i)).toBeInTheDocument();
  });

  it("squadron fight mode is unchecked by default", () => {
    render(<AgentsStep {...defaultProps} />);
    const toggle = screen.getByLabelText(/fight mode for the whole squadron/i) as HTMLInputElement;
    expect(toggle.checked).toBe(false);
  });

  it("squadron fight mode shows checked when all initial agents have fightMode true", () => {
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "", fightMode: true },
      { name: "a2", branch: "b", prompt: "p", driver: "claude-code", persona: "", fightMode: true },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} />);
    const toggle = screen.getByLabelText(/fight mode for the whole squadron/i) as HTMLInputElement;
    expect(toggle.checked).toBe(true);
  });

  it("squadron fight mode shows unchecked when only some agents have fightMode", () => {
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "", fightMode: true },
      { name: "a2", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} />);
    const toggle = screen.getByLabelText(/fight mode for the whole squadron/i) as HTMLInputElement;
    expect(toggle.checked).toBe(false);
  });

  it("toggling squadron fight mode applies fightMode to all agents on continue", async () => {
    const user = userEvent.setup();
    const onDone = vi.fn();
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
      { name: "a2", branch: "b", prompt: "p", driver: "claude-code", persona: "" },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} onDone={onDone} />);

    await user.click(screen.getByLabelText(/fight mode for the whole squadron/i));
    await user.click(screen.getByRole("button", { name: /continue to review/i }));

    expect(onDone).toHaveBeenCalledTimes(1);
    const passed = onDone.mock.calls[0][0] as SquadronAgent[];
    expect(passed).toHaveLength(2);
    expect(passed.every((a) => a.fightMode === true)).toBe(true);
  });

  it("toggling squadron fight mode off clears fightMode on all agents", async () => {
    const user = userEvent.setup();
    const onDone = vi.fn();
    const agents: SquadronAgent[] = [
      { name: "a1", branch: "b", prompt: "p", driver: "claude-code", persona: "", fightMode: true },
      { name: "a2", branch: "b", prompt: "p", driver: "claude-code", persona: "", fightMode: true },
    ];
    render(<AgentsStep {...defaultProps} agents={agents} onDone={onDone} />);

    await user.click(screen.getByLabelText(/fight mode for the whole squadron/i));
    await user.click(screen.getByRole("button", { name: /continue to review/i }));

    const passed = onDone.mock.calls[0][0] as SquadronAgent[];
    expect(passed.every((a) => a.fightMode === false)).toBe(true);
  });

  it("agents added after enabling squadron fight mode inherit it", async () => {
    const user = userEvent.setup();
    const onDone = vi.fn();
    render(<AgentsStep {...defaultProps} onDone={onDone} />);

    await user.click(screen.getByLabelText(/fight mode for the whole squadron/i));

    await user.type(screen.getByLabelText("Agent Name"), "fresh-agent");
    await user.type(screen.getByLabelText("Prompt"), "Do work");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    await user.click(screen.getByRole("button", { name: /continue to review/i }));
    const passed = onDone.mock.calls[0][0] as SquadronAgent[];
    expect(passed).toHaveLength(1);
    expect(passed[0].fightMode).toBe(true);
  });
});
