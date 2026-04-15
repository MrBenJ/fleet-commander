import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ManualAddForm } from "./ManualAddForm";
import type { Persona } from "../../types";

describe("ManualAddForm", () => {
  const personas: Persona[] = [
    { name: "zen-master", displayName: "Zen Master", preamble: "" },
    { name: "raging-jerk", displayName: "Raging Jerk", preamble: "" },
  ];

  const defaultProps = {
    squadronName: "test-squad",
    drivers: ["claude-code", "aider"],
    personas,
    onAgentAdded: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders all form fields", () => {
    render(<ManualAddForm {...defaultProps} />);
    expect(screen.getByText("Add Manually")).toBeInTheDocument();
    expect(screen.getByLabelText("Agent Name")).toBeInTheDocument();
    expect(screen.getByLabelText("Branch")).toBeInTheDocument();
    expect(screen.getByLabelText("Prompt")).toBeInTheDocument();
    expect(screen.getByLabelText("Harness")).toBeInTheDocument();
    expect(screen.getByLabelText("Persona")).toBeInTheDocument();
  });

  it("disables the add button when name or prompt is empty", () => {
    render(<ManualAddForm {...defaultProps} />);
    expect(screen.getByRole("button", { name: /add agent/i })).toBeDisabled();
  });

  it("enables the add button when name and prompt are filled", async () => {
    const user = userEvent.setup();
    render(<ManualAddForm {...defaultProps} />);
    await user.type(screen.getByLabelText("Agent Name"), "my-agent");
    await user.type(screen.getByLabelText("Prompt"), "Do something");
    expect(screen.getByRole("button", { name: /add agent/i })).not.toBeDisabled();
  });

  it("calls onAgentAdded with correct data and resets form", async () => {
    const user = userEvent.setup();
    const onAgentAdded = vi.fn();
    render(<ManualAddForm {...defaultProps} onAgentAdded={onAgentAdded} />);

    await user.type(screen.getByLabelText("Agent Name"), "auth-agent");
    await user.type(screen.getByLabelText("Prompt"), "Build login");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(onAgentAdded).toHaveBeenCalledWith({
      name: "auth-agent",
      branch: "squadron/test-squad/auth-agent",
      prompt: "Build login",
      driver: "claude-code",
      persona: "",
    });

    // Form should reset
    expect(screen.getByLabelText("Agent Name")).toHaveValue("");
    expect(screen.getByLabelText("Prompt")).toHaveValue("");
  });

  it("uses custom branch when provided", async () => {
    const user = userEvent.setup();
    const onAgentAdded = vi.fn();
    render(<ManualAddForm {...defaultProps} onAgentAdded={onAgentAdded} />);

    await user.type(screen.getByLabelText("Agent Name"), "my-agent");
    await user.type(screen.getByLabelText("Branch"), "custom/branch");
    await user.type(screen.getByLabelText("Prompt"), "Do work");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(onAgentAdded).toHaveBeenCalledWith(
      expect.objectContaining({ branch: "custom/branch" }),
    );
  });

  it("allows selecting a driver", async () => {
    const user = userEvent.setup();
    const onAgentAdded = vi.fn();
    render(<ManualAddForm {...defaultProps} onAgentAdded={onAgentAdded} />);

    await user.selectOptions(screen.getByLabelText("Harness"), "aider");
    await user.type(screen.getByLabelText("Agent Name"), "test");
    await user.type(screen.getByLabelText("Prompt"), "task");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(onAgentAdded).toHaveBeenCalledWith(
      expect.objectContaining({ driver: "aider" }),
    );
  });

  it("allows selecting a persona", async () => {
    const user = userEvent.setup();
    const onAgentAdded = vi.fn();
    render(<ManualAddForm {...defaultProps} onAgentAdded={onAgentAdded} />);

    await user.selectOptions(screen.getByLabelText("Persona"), "zen-master");
    await user.type(screen.getByLabelText("Agent Name"), "test");
    await user.type(screen.getByLabelText("Prompt"), "task");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(onAgentAdded).toHaveBeenCalledWith(
      expect.objectContaining({ persona: "zen-master" }),
    );
  });

  it("does nothing when clicking add with empty name", async () => {
    const user = userEvent.setup();
    const onAgentAdded = vi.fn();
    render(<ManualAddForm {...defaultProps} onAgentAdded={onAgentAdded} />);

    await user.type(screen.getByLabelText("Prompt"), "task");
    await user.click(screen.getByRole("button", { name: /add agent/i }));

    expect(onAgentAdded).not.toHaveBeenCalled();
  });
});
