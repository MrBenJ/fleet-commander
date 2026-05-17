import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AIGeneratePanel } from "./AIGeneratePanel";

vi.mock("../../api", () => ({
  generateAgents: vi.fn(),
  getAvailableDrivers: vi.fn(),
}));

import { generateAgents, getAvailableDrivers } from "../../api";

const mockGenerateAgents = vi.mocked(generateAgents);
const mockGetAvailableDrivers = vi.mocked(getAvailableDrivers);

describe("AIGeneratePanel", () => {
  const defaultProps = {
    squadronName: "test-squad",
    onAgentsGenerated: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAvailableDrivers.mockResolvedValue([
      { name: "claude-code", available: true },
      { name: "codex", available: false },
    ]);
  });

  it("renders the heading, driver dropdown, and textarea", async () => {
    render(<AIGeneratePanel {...defaultProps} />);
    expect(screen.getByText("AI Generate from Description")).toBeInTheDocument();
    expect(screen.getByLabelText("Driver")).toHaveValue("claude-code");
    expect(screen.getByRole("option", { name: /codex not installed/i })).toBeDisabled();
    expect(screen.getByLabelText("Task description for AI generation")).toBeInTheDocument();
  });

  it("disables the generate button when textarea is empty", () => {
    render(<AIGeneratePanel {...defaultProps} />);
    const button = screen.getByRole("button", { name: /generate agent breakdown/i });
    expect(button).toBeDisabled();
  });

  it("enables the generate button when textarea has content", async () => {
    const user = userEvent.setup();
    render(<AIGeneratePanel {...defaultProps} />);
    await user.type(screen.getByLabelText("Task description for AI generation"), "Build a login page");
    const button = screen.getByRole("button", { name: /generate agent breakdown/i });
    expect(button).not.toBeDisabled();
  });

  it("calls generateAgents and onAgentsGenerated on success", async () => {
    const user = userEvent.setup();
    const onAgentsGenerated = vi.fn();
    mockGenerateAgents.mockResolvedValue({
      agents: [
        { name: "auth-agent", branch: "", prompt: "Build auth", driver: "", persona: "" },
      ],
    });

    render(<AIGeneratePanel {...defaultProps} onAgentsGenerated={onAgentsGenerated} />);
    await user.type(screen.getByLabelText("Task description for AI generation"), "Build auth");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    expect(mockGenerateAgents).toHaveBeenCalledWith("Build auth", "claude-code");
    await vi.waitFor(() => {
      expect(onAgentsGenerated).toHaveBeenCalledWith([
        { name: "auth-agent", branch: "squadron/test-squad/auth-agent", prompt: "Build auth", driver: "claude-code", persona: "" },
      ]);
    });
  });

  it("sends codex when codex is available and selected", async () => {
    const user = userEvent.setup();
    mockGetAvailableDrivers.mockResolvedValue([
      { name: "claude-code", available: true },
      { name: "codex", available: true },
    ]);
    mockGenerateAgents.mockResolvedValue({
      agents: [
        { name: "codex-agent", branch: "", prompt: "Build with codex", driver: "codex", persona: "" },
      ],
    });

    render(<AIGeneratePanel {...defaultProps} />);
    await vi.waitFor(() => {
      expect(screen.getByRole("option", { name: "Codex" })).not.toBeDisabled();
    });
    await user.selectOptions(screen.getByLabelText("Driver"), "codex");
    await user.type(screen.getByLabelText("Task description for AI generation"), "Build codex things");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    expect(mockGenerateAgents).toHaveBeenCalledWith("Build codex things", "codex");
  });

  it("fills in default branch and driver when missing from API response", async () => {
    const user = userEvent.setup();
    const onAgentsGenerated = vi.fn();
    mockGenerateAgents.mockResolvedValue({
      agents: [
        { name: "my-agent", branch: "custom-branch", prompt: "Do stuff", driver: "aider", persona: "zen-master" },
      ],
    });

    render(<AIGeneratePanel {...defaultProps} onAgentsGenerated={onAgentsGenerated} />);
    await user.type(screen.getByLabelText("Task description for AI generation"), "tasks");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    await vi.waitFor(() => {
      expect(onAgentsGenerated).toHaveBeenCalledWith([
        { name: "my-agent", branch: "custom-branch", prompt: "Do stuff", driver: "aider", persona: "zen-master" },
      ]);
    });
  });

  it("displays an error message on generation failure", async () => {
    const user = userEvent.setup();
    mockGenerateAgents.mockRejectedValue(new Error("API down"));

    render(<AIGeneratePanel {...defaultProps} />);
    await user.type(screen.getByLabelText("Task description for AI generation"), "tasks");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    await vi.waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("API down");
    });
  });

  it("displays fallback error for non-Error throws", async () => {
    const user = userEvent.setup();
    mockGenerateAgents.mockRejectedValue("something bad");

    render(<AIGeneratePanel {...defaultProps} />);
    await user.type(screen.getByLabelText("Task description for AI generation"), "tasks");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    await vi.waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Generation failed");
    });
  });
});
