import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi } from "vitest";
import { PersonaStep } from "./PersonaStep";
import type { Persona } from "../../types";

const mockPersonas: Persona[] = [
  { name: "zen-master", displayName: "Zen Master", preamble: "Be calm and deliberate." },
  { name: "paranoid-perfectionist", displayName: "Paranoid Perfectionist", preamble: "Test everything." },
  { name: "overconfident-engineer", displayName: "Overconfident Engineer", preamble: "Ship it." },
];

const defaultProps = {
  personas: mockPersonas,
  onSelect: vi.fn(),
  onCancel: vi.fn(),
};

function renderPersona(overrides: Partial<typeof defaultProps> = {}) {
  const props = { ...defaultProps, onSelect: vi.fn(), onCancel: vi.fn(), ...overrides };
  return { ...render(<PersonaStep {...props} />), props };
}

describe("PersonaStep", () => {
  it("renders heading and description", () => {
    renderPersona();
    expect(screen.getByText("Select Persona")).toBeInTheDocument();
    expect(screen.getByText(/choose a personality/i)).toBeInTheDocument();
  });

  it("renders a card for each persona", () => {
    renderPersona();
    expect(screen.getByText("Zen Master")).toBeInTheDocument();
    expect(screen.getByText("Paranoid Perfectionist")).toBeInTheDocument();
    expect(screen.getByText("Overconfident Engineer")).toBeInTheDocument();
  });

  it("renders the No Persona option", () => {
    renderPersona();
    expect(screen.getByText("No Persona")).toBeInTheDocument();
  });

  it("renders known persona flavor text instead of preamble", () => {
    renderPersona();
    // The component uses its own personaFlavors map for known personas
    expect(screen.getByText(/deliberate, minimal, elegant/i)).toBeInTheDocument();
    expect(screen.getByText(/tests everything/i)).toBeInTheDocument();
    expect(screen.getByText(/ships fast/i)).toBeInTheDocument();
  });

  it("renders preamble for unknown personas", () => {
    const unknownPersona: Persona = {
      name: "unknown-one",
      displayName: "Unknown One",
      preamble: "A mysterious persona with unique traits.",
    };
    renderPersona({ personas: [unknownPersona] });
    expect(screen.getByText("A mysterious persona with unique traits.")).toBeInTheDocument();
  });

  it("truncates long preambles for unknown personas to 80 chars", () => {
    const longPreamble = "A".repeat(100);
    const persona: Persona = {
      name: "verbose-one",
      displayName: "Verbose",
      preamble: longPreamble,
    };
    renderPersona({ personas: [persona] });
    expect(screen.getByText("A".repeat(80))).toBeInTheDocument();
  });

  it("calls onSelect with persona name when a persona card is clicked", async () => {
    const user = userEvent.setup();
    const { props } = renderPersona();

    await user.click(screen.getByText("Zen Master"));
    expect(props.onSelect).toHaveBeenCalledWith("zen-master");
  });

  it("calls onSelect with empty string when No Persona is clicked", async () => {
    const user = userEvent.setup();
    const { props } = renderPersona();

    await user.click(screen.getByText("No Persona"));
    expect(props.onSelect).toHaveBeenCalledWith("");
  });

  it("calls onCancel when Back button is clicked", async () => {
    const user = userEvent.setup();
    const { props } = renderPersona();

    await user.click(screen.getByRole("button", { name: /back/i }));
    expect(props.onCancel).toHaveBeenCalled();
  });

  it("renders persona list with correct accessibility role", () => {
    renderPersona();
    expect(screen.getByRole("list", { name: /available personas/i })).toBeInTheDocument();
  });

  it("renders each persona card as a listitem", () => {
    renderPersona();
    // 3 personas + 1 "No Persona" option
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(4);
  });

  it("renders persona icons as aria-hidden decorative elements", () => {
    const { container } = renderPersona();
    const hiddenIcons = container.querySelectorAll("[aria-hidden='true']");
    // Each persona card + No Persona have an aria-hidden icon div
    expect(hiddenIcons.length).toBeGreaterThanOrEqual(4);
  });

  it("renders accessible labels on persona buttons", () => {
    renderPersona();
    expect(
      screen.getByRole("listitem", {
        name: /zen master.*deliberate, minimal, elegant/i,
      }),
    ).toBeInTheDocument();
  });

  it("handles empty personas list", () => {
    renderPersona({ personas: [] });
    // Should still render heading and the No Persona option
    expect(screen.getByText("Select Persona")).toBeInTheDocument();
    expect(screen.getByText("No Persona")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
  });
});
