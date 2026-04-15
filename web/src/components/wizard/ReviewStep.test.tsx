import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ReviewStep } from "./ReviewStep";
import type { SquadronAgent, Persona } from "../../types";

vi.mock("../../api", () => ({
  launchSquadron: vi.fn(),
}));

import { launchSquadron } from "../../api";

const mockLaunchSquadron = vi.mocked(launchSquadron);

const config = { name: "test-squadron", baseBranch: "main" };

const agents: SquadronAgent[] = [
  { name: "agent-alpha", branch: "feat/alpha", prompt: "Build the alpha feature", driver: "claude-code", persona: "zen-master" },
  { name: "agent-beta", branch: "feat/beta", prompt: "Build the beta feature", driver: "codex", persona: "" },
];

const personas: Persona[] = [
  { name: "zen-master", displayName: "Zen Master", preamble: "" },
];

const drivers = ["claude-code", "codex", "aider"];

const defaultProps = {
  config,
  agents,
  drivers,
  personas,
  onLaunched: vi.fn(),
  onEdit: vi.fn(),
  onAddMore: vi.fn(),
  onAgentsChanged: vi.fn(),
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ReviewStep", () => {
  it("renders squadron name", () => {
    render(<ReviewStep {...defaultProps} />);
    expect(screen.getByText("test-squadron")).toBeInTheDocument();
  });

  it("renders all agent cards", () => {
    render(<ReviewStep {...defaultProps} />);
    expect(screen.getByText("agent-alpha")).toBeInTheDocument();
    expect(screen.getByText("agent-beta")).toBeInTheDocument();
  });

  it("renders consensus selector", () => {
    render(<ReviewStep {...defaultProps} />);
    // "Universal Consensus" appears in both the info card and button, so use getAllByText
    expect(screen.getAllByText("Universal Consensus")).toHaveLength(2);
    expect(screen.getByText("Single Reviewer")).toBeInTheDocument();
    // "None" appears as both a consensus button and a persona option, use role
    expect(screen.getByRole("button", { name: "None" })).toBeInTheDocument();
  });

  it("renders auto-merge checkbox (checked by default)", () => {
    render(<ReviewStep {...defaultProps} />);
    const checkbox = screen.getByLabelText(/Auto-merge/);
    expect(checkbox).toBeChecked();
  });

  it("renders auto-PR checkbox when auto-merge is enabled", () => {
    render(<ReviewStep {...defaultProps} />);
    const checkbox = screen.getByLabelText(/Create pull request after merge/);
    expect(checkbox).not.toBeChecked();
  });

  it("hides auto-PR checkbox when auto-merge is disabled", async () => {
    render(<ReviewStep {...defaultProps} />);
    await userEvent.click(screen.getByLabelText(/Auto-merge/));
    expect(screen.queryByLabelText(/Create pull request after merge/)).not.toBeInTheDocument();
  });

  it("unchecks auto-PR when auto-merge is unchecked and re-checked", async () => {
    render(<ReviewStep {...defaultProps} />);
    // Check auto-PR
    await userEvent.click(screen.getByLabelText(/Create pull request after merge/));
    expect(screen.getByLabelText(/Create pull request after merge/)).toBeChecked();
    // Uncheck auto-merge (hides and resets autoPR)
    await userEvent.click(screen.getByLabelText(/Auto-merge/));
    // Re-check auto-merge
    await userEvent.click(screen.getByLabelText(/Auto-merge/));
    // Auto-PR should be unchecked
    expect(screen.getByLabelText(/Create pull request after merge/)).not.toBeChecked();
  });

  it("renders launch and add more buttons", () => {
    render(<ReviewStep {...defaultProps} />);
    expect(screen.getByText("Launch Squadron")).toBeInTheDocument();
    expect(screen.getByText("+ Add More")).toBeInTheDocument();
  });

  it("disables launch button when no agents", () => {
    render(<ReviewStep {...defaultProps} agents={[]} />);
    expect(screen.getByText("Launch Squadron")).toBeDisabled();
  });

  it("calls onAddMore when + Add More is clicked", async () => {
    render(<ReviewStep {...defaultProps} />);
    await userEvent.click(screen.getByText("+ Add More"));
    expect(defaultProps.onAddMore).toHaveBeenCalledOnce();
  });

  describe("editing agents", () => {
    it("enters edit mode when edit button is clicked", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByLabelText("Edit agent agent-alpha"));
      expect(screen.getByLabelText("Agent Name")).toHaveValue("agent-alpha");
    });

    it("saves edits and calls onAgentsChanged", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByLabelText("Edit agent agent-alpha"));
      const nameInput = screen.getByLabelText("Agent Name");
      await userEvent.clear(nameInput);
      await userEvent.type(nameInput, "agent-gamma");
      await userEvent.click(screen.getByText("Save"));
      expect(defaultProps.onAgentsChanged).toHaveBeenCalledWith(
        expect.arrayContaining([
          expect.objectContaining({ name: "agent-gamma" }),
        ])
      );
    });

    it("cancels editing without saving", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByLabelText("Edit agent agent-alpha"));
      await userEvent.click(screen.getByText("Cancel"));
      expect(defaultProps.onAgentsChanged).not.toHaveBeenCalled();
    });
  });

  describe("removing agents", () => {
    it("calls onAgentsChanged without the removed agent", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByLabelText("Remove agent agent-alpha"));
      expect(defaultProps.onAgentsChanged).toHaveBeenCalledWith([agents[1]]);
    });
  });

  describe("consensus selection", () => {
    it("switches consensus type when clicked", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByText("None"));
      expect(screen.getByText(/No review step/)).toBeInTheDocument();
    });

    it("shows reviewer dropdown when Single Reviewer is selected", async () => {
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByText("Single Reviewer"));
      expect(screen.getByText("Designated Reviewer")).toBeInTheDocument();
    });
  });

  describe("launch", () => {
    it("calls launchSquadron and onLaunched on success", async () => {
      mockLaunchSquadron.mockResolvedValueOnce({ mergeMaster: "agent-beta" });
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByText("Launch Squadron"));
      expect(mockLaunchSquadron).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "test-squadron",
          consensus: "universal",
          autoMerge: true,
          autoPR: undefined,
          agents,
        })
      );
      expect(defaultProps.onLaunched).toHaveBeenCalledWith(
        "test-squadron",
        agents,
        { consensus: "universal", autoMerge: true, mergeMaster: "agent-beta" }
      );
    });

    it("sends autoPR=true when auto-PR checkbox is checked", async () => {
      mockLaunchSquadron.mockResolvedValueOnce(undefined);
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByLabelText(/Create pull request after merge/));
      await userEvent.click(screen.getByText("Launch Squadron"));
      expect(mockLaunchSquadron).toHaveBeenCalledWith(
        expect.objectContaining({
          autoMerge: true,
          autoPR: true,
        })
      );
    });

    it("shows error message on launch failure", async () => {
      mockLaunchSquadron.mockRejectedValueOnce(new Error("Network error"));
      render(<ReviewStep {...defaultProps} />);
      await userEvent.click(screen.getByText("Launch Squadron"));
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });
});
