import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConsensusSelector } from "./ConsensusSelector";
import type { SquadronAgent } from "../../types";

const agents: SquadronAgent[] = [
  { name: "agent-a", branch: "feat/a", prompt: "do a", driver: "claude-code", persona: "" },
  { name: "agent-b", branch: "feat/b", prompt: "do b", driver: "codex", persona: "" },
];

const noop = () => {};

describe("ConsensusSelector", () => {
  it("renders all three consensus buttons", () => {
    render(
      <ConsensusSelector consensus="universal" reviewMaster="" agents={agents} onChange={noop} onReviewMasterChange={noop} />
    );
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(3);
    expect(buttons[0]).toHaveTextContent("Universal Consensus");
    expect(buttons[1]).toHaveTextContent("Single Reviewer");
    expect(buttons[2]).toHaveTextContent("None");
  });

  it("shows the description for the selected consensus type", () => {
    render(
      <ConsensusSelector consensus="universal" reviewMaster="" agents={agents} onChange={noop} onReviewMasterChange={noop} />
    );
    expect(screen.getByText(/Every agent reviews every other/)).toBeInTheDocument();
  });

  it("calls onChange when a consensus button is clicked", async () => {
    const onChange = vi.fn();
    render(
      <ConsensusSelector consensus="universal" reviewMaster="" agents={agents} onChange={onChange} onReviewMasterChange={noop} />
    );
    await userEvent.click(screen.getByText("None"));
    expect(onChange).toHaveBeenCalledWith("none");
  });

  it("does not show reviewer dropdown when consensus is not review_master", () => {
    render(
      <ConsensusSelector consensus="universal" reviewMaster="" agents={agents} onChange={noop} onReviewMasterChange={noop} />
    );
    expect(screen.queryByText("Designated Reviewer")).not.toBeInTheDocument();
  });

  it("shows reviewer dropdown when consensus is review_master", () => {
    render(
      <ConsensusSelector consensus="review_master" reviewMaster="" agents={agents} onChange={noop} onReviewMasterChange={noop} />
    );
    expect(screen.getByText("Designated Reviewer")).toBeInTheDocument();
    expect(screen.getByText("agent-a")).toBeInTheDocument();
    expect(screen.getByText("agent-b")).toBeInTheDocument();
  });

  it("calls onReviewMasterChange when reviewer is selected", async () => {
    const onReviewMasterChange = vi.fn();
    render(
      <ConsensusSelector consensus="review_master" reviewMaster="" agents={agents} onChange={noop} onReviewMasterChange={onReviewMasterChange} />
    );
    await userEvent.selectOptions(screen.getByRole("combobox"), "agent-a");
    expect(onReviewMasterChange).toHaveBeenCalledWith("agent-a");
  });
});
