import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentPill } from "./AgentPill";
import type { SquadronAgent } from "../../types";

vi.mock("../../api", () => ({
  stopAgent: vi.fn(),
}));

const defaultAgent: SquadronAgent = {
  name: "test-agent",
  branch: "feat/test",
  prompt: "Do some testing",
  driver: "claude-code",
  persona: "zen-master",
};

describe("AgentPill", () => {
  const defaultProps = {
    agent: defaultAgent,
    state: "working",
  };

  it("renders agent name", () => {
    render(<AgentPill {...defaultProps} />);
    expect(screen.getByText("test-agent")).toBeInTheDocument();
  });

  it("renders SVG icon with tooltip for claude-code driver", () => {
    render(<AgentPill {...defaultProps} />);
    const svg = screen.getByTitle("Claude Code");
    expect(svg).toBeInTheDocument();
  });

  it("renders SVG icon with tooltip for codex driver", () => {
    render(<AgentPill {...defaultProps} agent={{ ...defaultAgent, driver: "codex" }} />);
    const svg = screen.getByTitle("Codex");
    expect(svg).toBeInTheDocument();
  });

  it("renders text badge with tooltip for generic driver", () => {
    render(<AgentPill {...defaultProps} agent={{ ...defaultAgent, driver: "generic" }} />);
    expect(screen.getByText("G")).toBeInTheDocument();
    expect(screen.getByTitle("Generic")).toBeInTheDocument();
  });

  it("renders text abbreviation with tooltip for aider driver", () => {
    render(<AgentPill {...defaultProps} agent={{ ...defaultAgent, driver: "aider" }} />);
    expect(screen.getByTitle("Aider")).toBeInTheDocument();
  });

  it("truncates unknown driver to first 2 chars", () => {
    render(<AgentPill {...defaultProps} agent={{ ...defaultAgent, driver: "custom-driver" }} />);
    expect(screen.getByText("cu")).toBeInTheDocument();
  });

  it("has accessible label with name, state, and driver", () => {
    render(<AgentPill {...defaultProps} />);
    expect(screen.getByRole("button")).toHaveAccessibleName(
      "test-agent, status: working, harness: claude-code"
    );
  });

  it("shows tooltip on hover", async () => {
    const user = userEvent.setup();
    render(<AgentPill {...defaultProps} />);

    await user.hover(screen.getByRole("button"));
    expect(screen.getByText("Assume Control")).toBeInTheDocument();
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

  it("renders MERGE badge when isMerger is true", () => {
    render(<AgentPill {...defaultProps} isMerger={true} />);
    expect(screen.getByText("MERGE")).toBeInTheDocument();
  });

  it("does not render MERGE badge when isMerger is false", () => {
    render(<AgentPill {...defaultProps} isMerger={false} />);
    expect(screen.queryByText("MERGE")).not.toBeInTheDocument();
  });

  it("does not render MERGE badge when isMerger is undefined", () => {
    render(<AgentPill {...defaultProps} />);
    expect(screen.queryByText("MERGE")).not.toBeInTheDocument();
  });
});
