import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AgentCard } from "./AgentCard";
import type { SquadronAgent, Persona } from "../../types";

const baseAgent: SquadronAgent = {
  name: "test-agent",
  branch: "feat/test",
  prompt: "Do the thing",
  driver: "claude-code",
  persona: "zen-master",
};

const personas: Persona[] = [
  { name: "zen-master", displayName: "Zen Master", preamble: "" },
  { name: "raging-jerk", displayName: "Raging Jerk", preamble: "" },
];

const drivers = ["claude-code", "codex", "aider"];

const noop = () => {};

function renderCard(overrides: Partial<Parameters<typeof AgentCard>[0]> = {}) {
  const props = {
    agent: baseAgent,
    isEditing: false,
    editDraft: null,
    drivers,
    personas,
    onEdit: noop,
    onSave: noop,
    onCancel: noop,
    onRemove: noop,
    onDraftChange: noop,
    ...overrides,
  };
  return render(<AgentCard {...props} />, { wrapper: ({ children }) => <ul>{children}</ul> });
}

describe("AgentCard", () => {
  describe("view mode", () => {
    it("renders agent name, driver badge, branch, and persona", () => {
      renderCard();
      expect(screen.getByText("test-agent")).toBeInTheDocument();
      expect(screen.getByText("claude-code")).toBeInTheDocument();
      expect(screen.getByText("feat/test")).toBeInTheDocument();
      expect(screen.getByText(/Zen Master/)).toBeInTheDocument();
    });

    it("truncates long prompts at 120 characters", () => {
      const longPrompt = "A".repeat(150);
      renderCard({ agent: { ...baseAgent, prompt: longPrompt } });
      expect(screen.getByText(`${"A".repeat(120)}...`)).toBeInTheDocument();
    });

    it("shows full prompt when under 120 characters", () => {
      renderCard({ agent: { ...baseAgent, prompt: "Short prompt" } });
      expect(screen.getByText("Short prompt")).toBeInTheDocument();
    });

    it("does not show persona when empty", () => {
      renderCard({ agent: { ...baseAgent, persona: "" } });
      expect(screen.queryByText(/Zen Master/)).not.toBeInTheDocument();
    });

    it("calls onEdit when edit button is clicked", async () => {
      const onEdit = vi.fn();
      renderCard({ onEdit });
      await userEvent.click(screen.getByLabelText("Edit agent test-agent"));
      expect(onEdit).toHaveBeenCalledOnce();
    });

    it("calls onRemove when remove button is clicked", async () => {
      const onRemove = vi.fn();
      renderCard({ onRemove });
      await userEvent.click(screen.getByLabelText("Remove agent test-agent"));
      expect(onRemove).toHaveBeenCalledOnce();
    });
  });

  describe("edit mode", () => {
    const editDraft: SquadronAgent = { ...baseAgent };

    it("renders edit form with input fields", () => {
      renderCard({ isEditing: true, editDraft });
      expect(screen.getByLabelText("Agent Name")).toHaveValue("test-agent");
      expect(screen.getByLabelText("Branch")).toHaveValue("feat/test");
      expect(screen.getByLabelText("Harness")).toHaveValue("claude-code");
      expect(screen.getByLabelText("Persona")).toHaveValue("zen-master");
      expect(screen.getByLabelText("Prompt")).toHaveValue("Do the thing");
    });

    it("calls onDraftChange when name input changes", async () => {
      const onDraftChange = vi.fn();
      renderCard({ isEditing: true, editDraft, onDraftChange });
      await userEvent.clear(screen.getByLabelText("Agent Name"));
      await userEvent.type(screen.getByLabelText("Agent Name"), "new-name");
      expect(onDraftChange).toHaveBeenCalled();
    });

    it("calls onSave when Save is clicked", async () => {
      const onSave = vi.fn();
      renderCard({ isEditing: true, editDraft, onSave });
      await userEvent.click(screen.getByText("Save"));
      expect(onSave).toHaveBeenCalledOnce();
    });

    it("calls onCancel when Cancel is clicked", async () => {
      const onCancel = vi.fn();
      renderCard({ isEditing: true, editDraft, onCancel });
      await userEvent.click(screen.getByText("Cancel"));
      expect(onCancel).toHaveBeenCalledOnce();
    });
  });
});
