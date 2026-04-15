import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { WizardLayout } from "./WizardLayout";
import type { Persona } from "../../types";

// Mock the api module
vi.mock("../../api", () => ({
  getBranches: vi.fn().mockResolvedValue(["main", "develop"]),
}));

// Mock child components to isolate WizardLayout logic
vi.mock("./AgentsStep", () => ({
  AgentsStep: ({ onDone, onPickPersona }: any) => (
    <div data-testid="agents-step">
      <button onClick={() => onDone([{ name: "a1", branch: "b1", prompt: "p1", driver: "claude-code", persona: "" }])}>
        Finish Agents
      </button>
      <button onClick={() => onPickPersona(0, [{ name: "a1", branch: "b1", prompt: "p1", driver: "claude-code", persona: "" }])}>
        Pick Persona
      </button>
    </div>
  ),
}));

vi.mock("./ReviewStep", () => ({
  ReviewStep: ({ onEdit, onAddMore }: any) => (
    <div data-testid="review-step">
      <button onClick={onEdit}>Edit Agents</button>
      <button onClick={onAddMore}>Add More</button>
    </div>
  ),
}));

const mockPersonas: Persona[] = [
  { name: "zen-master", displayName: "Zen Master", preamble: "Be calm." },
];

const defaultProps = {
  personas: mockPersonas,
  drivers: ["claude-code", "aider"],
  currentBranch: "main",
  onLaunched: vi.fn(),
};

function renderWizard(overrides: Partial<typeof defaultProps> = {}) {
  const props = { ...defaultProps, onLaunched: vi.fn(), ...overrides };
  return { ...render(<WizardLayout {...props} />), props };
}

describe("WizardLayout", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the progress indicator with three steps", async () => {
    renderWizard();
    await waitFor(() => expect(screen.getByRole("navigation", { name: /wizard progress/i })).toBeInTheDocument());
    expect(screen.getByText("Setup")).toBeInTheDocument();
    expect(screen.getByText("Agents")).toBeInTheDocument();
    expect(screen.getByText("Review")).toBeInTheDocument();
  });

  it("starts on the Setup step", async () => {
    renderWizard();
    await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
    // Setup step should be current
    const setupItem = screen.getByText("Setup").closest("li");
    expect(setupItem).toHaveAttribute("aria-current", "step");
  });

  it("navigates from Setup to Agents when SetupStep calls onDone", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Fill in the squadron name and click Continue
    const nameInput = screen.getByLabelText(/squadron name/i);
    await user.type(nameInput, "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Should now show the mocked AgentsStep
    expect(screen.getByTestId("agents-step")).toBeInTheDocument();
  });

  it("navigates from Agents to Review when AgentsStep calls onDone", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Move to Agents step
    const nameInput = screen.getByLabelText(/squadron name/i);
    await user.type(nameInput, "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Finish agents
    await user.click(screen.getByText("Finish Agents"));

    // Should show ReviewStep
    expect(screen.getByTestId("review-step")).toBeInTheDocument();
  });

  it("navigates from Agents to PersonaStep when picking a persona", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Move to Agents step
    const nameInput = screen.getByLabelText(/squadron name/i);
    await user.type(nameInput, "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Pick persona
    await user.click(screen.getByText("Pick Persona"));

    // Should show PersonaStep
    expect(screen.getByText("Select Persona")).toBeInTheDocument();
  });

  it("returns to Agents step after selecting a persona", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Move to Agents step
    await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Pick persona
    await user.click(screen.getByText("Pick Persona"));

    // Select a persona
    await user.click(screen.getByText("Zen Master"));

    // Should return to Agents step
    expect(screen.getByTestId("agents-step")).toBeInTheDocument();
  });

  it("returns to Agents step when persona cancel is clicked", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Move to Agents step
    await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    // Pick persona then cancel
    await user.click(screen.getByText("Pick Persona"));
    await user.click(screen.getByRole("button", { name: /back/i }));

    expect(screen.getByTestId("agents-step")).toBeInTheDocument();
  });

  it("navigates from Review back to Agents when onEdit is called", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Navigate to Review
    await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));
    await user.click(screen.getByText("Finish Agents"));

    // Click Edit from Review
    await user.click(screen.getByText("Edit Agents"));
    expect(screen.getByTestId("agents-step")).toBeInTheDocument();
  });

  it("navigates from Review back to Agents when onAddMore is called", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Navigate to Review
    await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));
    await user.click(screen.getByText("Finish Agents"));

    // Click Add More from Review
    await user.click(screen.getByText("Add More"));
    expect(screen.getByTestId("agents-step")).toBeInTheDocument();
  });

  it("fetches branches on mount", async () => {
    const { getBranches } = await import("../../api");
    renderWizard();
    await waitFor(() => {
      expect(getBranches).toHaveBeenCalled();
    });
  });

  it("marks current step in progress indicator", async () => {
    const user = userEvent.setup();
    renderWizard();

    // Setup should be current
    let setupLi = screen.getByText("Setup").closest("li");
    expect(setupLi).toHaveAttribute("aria-current", "step");

    // Move to Agents
    await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
    await user.click(screen.getByRole("button", { name: /continue/i }));

    const agentsLi = screen.getByText("Agents").closest("li");
    expect(agentsLi).toHaveAttribute("aria-current", "step");

    // Setup should no longer be current
    setupLi = screen.getByText("Setup").closest("li");
    expect(setupLi).not.toHaveAttribute("aria-current", "step");
  });

  it("uses an ordered list for progress steps", async () => {
    renderWizard();
    const nav = screen.getByRole("navigation", { name: /wizard progress/i });
    expect(nav.querySelector("ol")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
  });

  it("has aria-live on the step content area", async () => {
    const { container } = renderWizard();
    const liveRegion = container.querySelector("[aria-live='polite']");
    expect(liveRegion).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
  });
});
