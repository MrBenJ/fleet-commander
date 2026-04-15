import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { SetupStep } from "./SetupStep";

const defaultProps = {
  initial: { name: "", baseBranch: "main" },
  currentBranch: "main",
  branches: ["main", "develop", "feature/foo"],
  onDone: vi.fn(),
};

function renderSetup(overrides: Partial<typeof defaultProps> = {}) {
  const props = { ...defaultProps, onDone: vi.fn(), ...overrides };
  return { ...render(<SetupStep {...props} />), props };
}

describe("SetupStep", () => {
  it("renders the form with heading and inputs", () => {
    renderSetup();
    expect(screen.getByText("Squadron Setup")).toBeInTheDocument();
    expect(screen.getByLabelText(/squadron name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/base branch/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /continue/i })).toBeInTheDocument();
  });

  it("populates inputs from initial config", () => {
    renderSetup({ initial: { name: "my-squad", baseBranch: "develop" } });
    expect(screen.getByLabelText(/squadron name/i)).toHaveValue("my-squad");
    expect(screen.getByLabelText(/base branch/i)).toHaveValue("develop");
  });

  it("renders all branches as select options", () => {
    renderSetup({ branches: ["main", "develop", "staging"] });
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(3);
    expect(options.map((o) => o.textContent)).toEqual(["main", "develop", "staging"]);
  });

  it("falls back to currentBranch when branches list is empty", () => {
    renderSetup({ branches: [], currentBranch: "my-branch" });
    const options = screen.getAllByRole("option");
    expect(options).toHaveLength(1);
    expect(options[0]).toHaveTextContent("my-branch");
  });

  it("disables Continue button when name is empty", () => {
    renderSetup({ initial: { name: "", baseBranch: "main" } });
    const btn = screen.getByRole("button", { name: /continue/i });
    expect(btn).toBeDisabled();
    expect(btn).toHaveAttribute("aria-disabled", "true");
  });

  it("disables Continue button when name is only whitespace", () => {
    renderSetup({ initial: { name: "   ", baseBranch: "main" } });
    expect(screen.getByRole("button", { name: /continue/i })).toBeDisabled();
  });

  it("enables Continue button when name has content", () => {
    renderSetup({ initial: { name: "alpha", baseBranch: "main" } });
    expect(screen.getByRole("button", { name: /continue/i })).not.toBeDisabled();
  });

  it("calls onDone with config when Continue is clicked", async () => {
    const user = userEvent.setup();
    const { props } = renderSetup({ initial: { name: "squad", baseBranch: "main" } });

    await user.click(screen.getByRole("button", { name: /continue/i }));
    expect(props.onDone).toHaveBeenCalledWith({ name: "squad", baseBranch: "main" });
  });

  it("updates name via typing and calls onDone with updated value", async () => {
    const user = userEvent.setup();
    const { props } = renderSetup();

    const nameInput = screen.getByLabelText(/squadron name/i);
    await user.type(nameInput, "new-squad");

    // Button should now be enabled
    expect(screen.getByRole("button", { name: /continue/i })).not.toBeDisabled();

    await user.click(screen.getByRole("button", { name: /continue/i }));
    expect(props.onDone).toHaveBeenCalledWith(
      expect.objectContaining({ name: "new-squad" }),
    );
  });

  it("updates base branch via select and calls onDone with updated value", async () => {
    const user = userEvent.setup();
    const { props } = renderSetup({ initial: { name: "s", baseBranch: "main" } });

    await user.selectOptions(screen.getByLabelText(/base branch/i), "develop");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    expect(props.onDone).toHaveBeenCalledWith({ name: "s", baseBranch: "develop" });
  });

  it("does not call onDone when button is clicked while disabled", async () => {
    const user = userEvent.setup();
    const { props } = renderSetup({ initial: { name: "", baseBranch: "main" } });

    await user.click(screen.getByRole("button", { name: /continue/i }));
    expect(props.onDone).not.toHaveBeenCalled();
  });

  it("marks squadron name input as required", () => {
    renderSetup();
    expect(screen.getByLabelText(/squadron name/i)).toHaveAttribute("aria-required", "true");
  });
});
