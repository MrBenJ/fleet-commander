import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MultiView } from "./MultiView";

const agents = [
  { name: "alpha", branch: "feat/alpha", prompt: "Do alpha", driver: "claude-code", persona: "zen-master" },
  { name: "beta", branch: "feat/beta", prompt: "Do beta", driver: "codex", persona: "raging-jerk" },
  { name: "gamma", branch: "feat/gamma", prompt: "Do gamma", driver: "claude-code", persona: "zen-master" },
];

describe("MultiView", () => {
  it("renders an iframe for each agent", () => {
    render(<MultiView agents={agents} />);
    const iframes = screen.getAllByTitle(/terminal: /i);
    expect(iframes).toHaveLength(3);
  });

  it("sets correct src for each iframe", () => {
    render(<MultiView agents={agents} />);
    const iframes = screen.getAllByTitle(/terminal: /i) as HTMLIFrameElement[];
    expect(iframes[0].src).toContain("/terminal/alpha");
    expect(iframes[1].src).toContain("/terminal/beta");
    expect(iframes[2].src).toContain("/terminal/gamma");
  });

  it("renders agent name labels", () => {
    render(<MultiView agents={agents} />);
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("beta")).toBeInTheDocument();
    expect(screen.getByText("gamma")).toBeInTheDocument();
  });

  it("uses CSS grid layout", () => {
    const { container } = render(<MultiView agents={agents} />);
    const grid = container.firstElementChild as HTMLElement;
    expect(grid.style.display).toBe("grid");
  });

  it("renders with a single agent", () => {
    render(<MultiView agents={[agents[0]]} />);
    expect(screen.getAllByTitle(/terminal: /i)).toHaveLength(1);
  });

  it("renders with four agents in 2x2 grid", () => {
    const four = [
      ...agents,
      { name: "delta", branch: "feat/delta", prompt: "Do delta", driver: "claude-code", persona: "zen-master" },
    ];
    render(<MultiView agents={four} />);
    const grid = screen.getAllByTitle(/terminal: /i)[0].closest("[data-testid='multi-view-grid']") as HTMLElement;
    expect(grid.style.gridTemplateColumns).toContain("1fr 1fr");
  });
});
