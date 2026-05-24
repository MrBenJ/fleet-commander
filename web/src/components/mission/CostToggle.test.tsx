import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CostToggle } from "./CostToggle";

describe("CostToggle", () => {
  it("renders a button with accessible label", () => {
    render(<CostToggle active={true} onToggle={() => {}} />);
    expect(screen.getByRole("button", { name: /toggle cost/i })).toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    const onToggle = vi.fn();
    const user = userEvent.setup();
    render(<CostToggle active={true} onToggle={onToggle} />);
    await user.click(screen.getByRole("button", { name: /toggle cost/i }));
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("reflects active state via aria-pressed", () => {
    render(<CostToggle active={true} onToggle={() => {}} />);
    expect(
      screen.getByRole("button", { name: /toggle cost/i }).getAttribute("aria-pressed")
    ).toBe("true");
  });

  it("has aria-pressed false when inactive", () => {
    render(<CostToggle active={false} onToggle={() => {}} />);
    expect(
      screen.getByRole("button", { name: /toggle cost/i }).getAttribute("aria-pressed")
    ).toBe("false");
  });

  it("describes the action it will take in its title", () => {
    const { rerender } = render(<CostToggle active={true} onToggle={() => {}} />);
    expect(screen.getByRole("button").getAttribute("title")).toBe("Hide cost meters");
    rerender(<CostToggle active={false} onToggle={() => {}} />);
    expect(screen.getByRole("button").getAttribute("title")).toBe("Show cost meters");
  });
});
