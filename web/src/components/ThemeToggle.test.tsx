import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "./ThemeToggle";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemeToggle", () => {
  it("defaults to dark theme", () => {
    render(<ThemeToggle />);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(screen.getByRole("button")).toHaveAccessibleName("Switch to light mode");
  });

  it("toggles to light theme on click", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByRole("button"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem("fleet-theme")).toBe("light");
    expect(screen.getByRole("button")).toHaveAccessibleName("Switch to dark mode");
  });

  it("toggles back to dark on second click", async () => {
    const user = userEvent.setup();
    render(<ThemeToggle />);

    await user.click(screen.getByRole("button"));
    await user.click(screen.getByRole("button"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem("fleet-theme")).toBe("dark");
  });

  it("restores saved light theme from localStorage", () => {
    localStorage.setItem("fleet-theme", "light");
    render(<ThemeToggle />);
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(screen.getByRole("button")).toHaveAccessibleName("Switch to dark mode");
  });
});
