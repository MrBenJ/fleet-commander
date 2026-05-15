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
  AgentsStep: ({ agents, onDone, onPickPersona, onChange }: any) => (
    <div data-testid="agents-step">
      <div data-testid="agents-count">{agents.length}</div>
      <button onClick={() => {
        const next = [...agents, { name: `a${agents.length + 1}`, branch: `b${agents.length + 1}`, prompt: "p", driver: "claude-code", persona: "" }];
        onChange?.(next);
      }}>
        Add Live Agent
      </button>
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
  ghAvailable: true,
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

  describe("clickable step navigation", () => {
    it("renders each step as a button", async () => {
      renderWizard();
      await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
      expect(screen.getByRole("button", { name: /Setup.*current step/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Agents.*locked/i })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /Review.*locked/i })).toBeInTheDocument();
    });

    it("disables Agents and Review buttons before setup is complete", async () => {
      renderWizard();
      await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
      const agentsBtn = screen.getByRole("button", { name: /Agents.*locked/i });
      const reviewBtn = screen.getByRole("button", { name: /Review.*locked/i });
      expect(agentsBtn).toBeDisabled();
      expect(reviewBtn).toBeDisabled();
    });

    it("enables Agents step button once a valid squadron name is typed", async () => {
      const user = userEvent.setup();
      renderWizard();
      await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      // Agents button should now be enabled (not locked)
      expect(screen.getByRole("button", { name: /Go to Agents/i })).not.toBeDisabled();
      // Review still locked because no agents yet
      expect(screen.getByRole("button", { name: /Review.*locked/i })).toBeDisabled();
    });

    it("does not enable Agents step for an invalid squadron name", async () => {
      const user = userEvent.setup();
      renderWizard();
      await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
      await user.type(screen.getByLabelText(/squadron name/i), "invalid name with spaces");
      expect(screen.getByRole("button", { name: /Agents.*locked/i })).toBeDisabled();
    });

    it("navigates to Agents when clicking the Agents step button", async () => {
      const user = userEvent.setup();
      renderWizard();
      await waitFor(() => expect(screen.getByText("Squadron Setup")).toBeInTheDocument());
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      await user.click(screen.getByRole("button", { name: /Go to Agents/i }));
      expect(screen.getByTestId("agents-step")).toBeInTheDocument();
    });

    it("navigates back to Setup by clicking the Setup step button", async () => {
      const user = userEvent.setup();
      renderWizard();
      // Get to agents
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      await user.click(screen.getByRole("button", { name: /continue/i }));
      expect(screen.getByTestId("agents-step")).toBeInTheDocument();
      // Click Setup breadcrumb
      await user.click(screen.getByRole("button", { name: /Go to Setup/i }));
      expect(screen.getByText("Squadron Setup")).toBeInTheDocument();
    });

    it("preserves squadron name when navigating away from Setup and back", async () => {
      const user = userEvent.setup();
      renderWizard();
      await user.type(screen.getByLabelText(/squadron name/i), "preserved-name");
      // Navigate to Agents via breadcrumb
      await user.click(screen.getByRole("button", { name: /Go to Agents/i }));
      // Navigate back to Setup via breadcrumb
      await user.click(screen.getByRole("button", { name: /Go to Setup/i }));
      // Name should be preserved
      expect(screen.getByLabelText(/squadron name/i)).toHaveValue("preserved-name");
    });

    it("preserves agents added via onChange when navigating away from Agents and back", async () => {
      const user = userEvent.setup();
      renderWizard();
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      await user.click(screen.getByRole("button", { name: /Go to Agents/i }));
      // Add agents via onChange (simulating in-progress edits)
      await user.click(screen.getByText("Add Live Agent"));
      await user.click(screen.getByText("Add Live Agent"));
      expect(screen.getByTestId("agents-count")).toHaveTextContent("2");
      // Jump back to Setup
      await user.click(screen.getByRole("button", { name: /Go to Setup/i }));
      // Return to Agents — agents should still be there
      await user.click(screen.getByRole("button", { name: /Go to Agents/i }));
      expect(screen.getByTestId("agents-count")).toHaveTextContent("2");
    });

    it("enables the Review step once agents have been added via onChange", async () => {
      const user = userEvent.setup();
      renderWizard();
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      await user.click(screen.getByRole("button", { name: /Go to Agents/i }));
      // Review still locked
      expect(screen.getByRole("button", { name: /Review.*locked/i })).toBeDisabled();
      await user.click(screen.getByText("Add Live Agent"));
      // Review now navigable
      expect(screen.getByRole("button", { name: /Go to Review/i })).not.toBeDisabled();
    });

    it("clicking a disabled step button does not navigate", async () => {
      const user = userEvent.setup();
      renderWizard();
      // Try to click locked Agents button
      const agentsBtn = screen.getByRole("button", { name: /Agents.*locked/i });
      await user.click(agentsBtn);
      // Still on Setup
      expect(screen.getByText("Squadron Setup")).toBeInTheDocument();
    });

    it("highlights Agents step as current when a persona modal is open", async () => {
      const user = userEvent.setup();
      renderWizard();
      await user.type(screen.getByLabelText(/squadron name/i), "test-squad");
      await user.click(screen.getByRole("button", { name: /continue/i }));
      await user.click(screen.getByText("Pick Persona"));
      // Persona modal active — but Agents breadcrumb should still mark current
      const agentsLi = screen.getByText("Agents").closest("li");
      expect(agentsLi).toHaveAttribute("aria-current", "step");
    });
  });
});
