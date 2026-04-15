import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentListItem } from "./AgentListItem";
import type { Persona } from "../../types";

describe("AgentListItem", () => {
  const personas: Persona[] = [
    { name: "zen-master", displayName: "Zen Master", preamble: "" },
    { name: "paranoid-perfectionist", displayName: "Paranoid Perfectionist", preamble: "" },
  ];

  const baseAgent = {
    name: "auth-agent",
    branch: "squadron/squad/auth-agent",
    prompt: "Build the authentication system",
    driver: "claude-code",
    persona: "",
  };

  it("renders agent name and driver badge", () => {
    render(
      <AgentListItem agent={baseAgent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    expect(screen.getByText("auth-agent")).toBeInTheDocument();
    expect(screen.getByText("claude-code")).toBeInTheDocument();
  });

  it("truncates long prompts at 80 characters", () => {
    const longPrompt = "A".repeat(100);
    const agent = { ...baseAgent, prompt: longPrompt };
    render(
      <AgentListItem agent={agent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    expect(screen.getByText(`${"A".repeat(80)}...`)).toBeInTheDocument();
  });

  it("does not truncate short prompts", () => {
    render(
      <AgentListItem agent={baseAgent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    expect(screen.getByText("Build the authentication system")).toBeInTheDocument();
  });

  it("shows 'No persona selected' when persona is empty", () => {
    render(
      <AgentListItem agent={baseAgent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    expect(screen.getByLabelText("Select persona for auth-agent")).toHaveTextContent("No persona selected");
  });

  it("shows persona display name and icon when persona is set", () => {
    const agent = { ...baseAgent, persona: "paranoid-perfectionist" };
    render(
      <AgentListItem agent={agent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    const button = screen.getByLabelText("Select persona for auth-agent");
    expect(button.textContent).toContain("Paranoid Perfectionist");
  });

  it("calls onRemove when remove button is clicked", async () => {
    const user = userEvent.setup();
    const onRemove = vi.fn();
    render(
      <AgentListItem agent={baseAgent} onRemove={onRemove} onPickPersona={vi.fn()} personas={personas} />,
    );
    await user.click(screen.getByLabelText("Remove agent auth-agent"));
    expect(onRemove).toHaveBeenCalledOnce();
  });

  it("calls onPickPersona when persona button is clicked", async () => {
    const user = userEvent.setup();
    const onPickPersona = vi.fn();
    render(
      <AgentListItem agent={baseAgent} onRemove={vi.fn()} onPickPersona={onPickPersona} personas={personas} />,
    );
    await user.click(screen.getByLabelText("Select persona for auth-agent"));
    expect(onPickPersona).toHaveBeenCalledOnce();
  });

  it("handles unknown persona gracefully", () => {
    const agent = { ...baseAgent, persona: "unknown-persona" };
    render(
      <AgentListItem agent={agent} onRemove={vi.fn()} onPickPersona={vi.fn()} personas={personas} />,
    );
    const button = screen.getByLabelText("Select persona for auth-agent");
    expect(button.textContent).toContain("unknown-persona");
  });
});
