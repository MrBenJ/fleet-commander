import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MultiViewToggle } from "./MultiViewToggle";

describe("MultiViewToggle", () => {
  it("renders a button with accessible label", () => {
    render(<MultiViewToggle active={false} onToggle={() => {}} />);
    expect(screen.getByRole("button", { name: /multi-view/i })).toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    const onToggle = vi.fn();
    const user = userEvent.setup();
    render(<MultiViewToggle active={false} onToggle={onToggle} />);
    await user.click(screen.getByRole("button", { name: /multi-view/i }));
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("applies active styling when active is true", () => {
    render(<MultiViewToggle active={true} onToggle={() => {}} />);
    const btn = screen.getByRole("button", { name: /multi-view/i });
    expect(btn.getAttribute("aria-pressed")).toBe("true");
  });

  it("has aria-pressed false when inactive", () => {
    render(<MultiViewToggle active={false} onToggle={() => {}} />);
    const btn = screen.getByRole("button", { name: /multi-view/i });
    expect(btn.getAttribute("aria-pressed")).toBe("false");
  });
});
