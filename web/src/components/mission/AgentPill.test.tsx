import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentPill } from "./AgentPill";

describe("AgentPill", () => {
  const defaultProps = {
    name: "test-agent",
    state: "working",
    driver: "claude-code",
    onClick: vi.fn(),
  };

  it("renders agent name", () => {
    render(<AgentPill {...defaultProps} />);
    expect(screen.getByText("test-agent")).toBeInTheDocument();
  });

  it("shows driver abbreviation for claude-code", () => {
    render(<AgentPill {...defaultProps} driver="claude-code" />);
    expect(screen.getByText("cc")).toBeInTheDocument();
  });

  it("shows driver abbreviation for aider", () => {
    render(<AgentPill {...defaultProps} driver="aider" />);
    expect(screen.getByText("ai")).toBeInTheDocument();
  });

  it("shows driver abbreviation for codex", () => {
    render(<AgentPill {...defaultProps} driver="codex" />);
    expect(screen.getByText("cx")).toBeInTheDocument();
  });

  it("truncates unknown driver to first 2 chars", () => {
    render(<AgentPill {...defaultProps} driver="custom-driver" />);
    expect(screen.getByText("cu")).toBeInTheDocument();
  });

  it("has accessible label with name, state, and driver", () => {
    render(<AgentPill {...defaultProps} />);
    expect(screen.getByRole("button")).toHaveAccessibleName(
      "test-agent, status: working, harness: claude-code"
    );
  });

  it("calls onClick when clicked", async () => {
    const onClick = vi.fn();
    const user = userEvent.setup();
    render(<AgentPill {...defaultProps} onClick={onClick} />);

    await user.click(screen.getByRole("button"));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("applies orange border when state is waiting", () => {
    render(<AgentPill {...defaultProps} state="waiting" />);
    const button = screen.getByRole("button");
    expect(button.style.border).toContain("var(--orange)");
  });

  it("applies default border when state is not waiting", () => {
    render(<AgentPill {...defaultProps} state="working" />);
    const button = screen.getByRole("button");
    expect(button.style.border).toContain("var(--border)");
  });
});
