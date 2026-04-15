import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentTooltip } from "./AgentTooltip";
import * as api from "../../api";
import type { SquadronAgent, Persona } from "../../types";

vi.mock("../../api", () => ({
  stopAgent: vi.fn(),
}));

const mockStopAgent = vi.mocked(api.stopAgent);

const agent: SquadronAgent = {
  name: "test-agent",
  branch: "feat/test",
  prompt: "Do some testing work for the project",
  driver: "claude-code",
  persona: "zen-master",
};

const persona: Persona = {
  name: "zen-master",
  displayName: "Zen Master",
  preamble: "Be calm",
};

const defaultProps = {
  agent,
  state: "working",
  persona,
};

beforeEach(() => {
  vi.resetAllMocks();
});

describe("AgentTooltip", () => {
  it("renders agent name and persona display name", () => {
    render(<AgentTooltip {...defaultProps} />);
    expect(screen.getByText("test-agent")).toBeInTheDocument();
    expect(screen.getByText("Zen Master")).toBeInTheDocument();
  });

  it("renders agent details: branch and driver", () => {
    render(<AgentTooltip {...defaultProps} />);
    expect(screen.getByText("feat/test")).toBeInTheDocument();
    expect(screen.getByText("claude-code")).toBeInTheDocument();
  });

  it("renders state text", () => {
    render(<AgentTooltip {...defaultProps} state="waiting" />);
    expect(screen.getByText("waiting")).toBeInTheDocument();
  });

  it("renders task prompt text", () => {
    render(<AgentTooltip {...defaultProps} />);
    expect(screen.getByText("Do some testing work for the project")).toBeInTheDocument();
  });

  it("shows full prompt without truncation", () => {
    const longPrompt = "x".repeat(250);
    render(
      <AgentTooltip
        {...defaultProps}
        agent={{ ...agent, prompt: longPrompt }}
      />
    );
    expect(screen.getByText(longPrompt)).toBeInTheDocument();
  });

  it("shows 'No Persona' when persona is undefined", () => {
    render(<AgentTooltip {...defaultProps} persona={undefined} />);
    expect(screen.getByText("No Persona")).toBeInTheDocument();
  });

  it("renders Assume Control and Stop buttons", () => {
    render(<AgentTooltip {...defaultProps} />);
    expect(screen.getByText("Assume Control")).toBeInTheDocument();
    expect(screen.getByText("Stop")).toBeInTheDocument();
  });

  it("opens terminal window on Assume Control click", async () => {
    const windowOpen = vi.spyOn(window, "open").mockImplementation(() => null);
    const user = userEvent.setup();
    render(<AgentTooltip {...defaultProps} />);

    await user.click(screen.getByText("Assume Control"));
    expect(windowOpen).toHaveBeenCalledWith(
      "/terminal/test-agent",
      "_blank",
      "width=900,height=600"
    );
    windowOpen.mockRestore();
  });

  it("calls stopAgent on Stop click after confirm", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockStopAgent.mockResolvedValue({ status: "stopped", agent: "test-agent" });
    const user = userEvent.setup();
    render(<AgentTooltip {...defaultProps} />);

    await user.click(screen.getByText("Stop"));
    expect(mockStopAgent).toHaveBeenCalledWith("test-agent");
    vi.mocked(window.confirm).mockRestore();
  });

  it("does not stop when confirm is cancelled", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const user = userEvent.setup();
    render(<AgentTooltip {...defaultProps} />);

    await user.click(screen.getByText("Stop"));
    expect(mockStopAgent).not.toHaveBeenCalled();
    vi.mocked(window.confirm).mockRestore();
  });

  it("shows error when stop fails", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    mockStopAgent.mockRejectedValue(new Error("connection refused"));
    const user = userEvent.setup();
    render(<AgentTooltip {...defaultProps} />);

    await user.click(screen.getByText("Stop"));
    expect(await screen.findByRole("alert")).toHaveTextContent("connection refused");
    vi.mocked(window.confirm).mockRestore();
  });
});
