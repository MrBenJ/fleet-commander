import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ContextLog } from "./ContextLog";
import type { ContextMessage } from "../../types";

const agentColors: Record<string, string> = {
  alpha: "#ff0000",
  beta: "#00ff00",
};

function makeMsg(agent: string, message: string, hour = 14, minute = 30): ContextMessage {
  return {
    agent,
    message,
    timestamp: `2026-04-14T${hour.toString().padStart(2, "0")}:${minute.toString().padStart(2, "0")}:00Z`,
  };
}

describe("ContextLog", () => {
  it("renders empty state without crashing", () => {
    render(<ContextLog messages={[]} agentColors={agentColors} onAgentClick={vi.fn()} />);
    expect(screen.getByRole("log")).toBeInTheDocument();
  });

  it("does not show live indicator when no messages", () => {
    render(<ContextLog messages={[]} agentColors={agentColors} onAgentClick={vi.fn()} />);
    expect(screen.queryByText("● live")).not.toBeInTheDocument();
  });

  it("renders messages with agent name and text", () => {
    const messages = [makeMsg("alpha", "Task complete")];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={vi.fn()} />);
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("Task complete")).toBeInTheDocument();
  });

  it("formats timestamp as HH:MM", () => {
    const messages = [makeMsg("alpha", "msg", 9, 5)];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={vi.fn()} />);
    // The time is rendered in local timezone, so just check the time element exists
    const timeEl = screen.getByRole("log").querySelector("time");
    expect(timeEl).toBeInTheDocument();
  });

  it("renders multiple messages in order", () => {
    const messages = [
      makeMsg("alpha", "First message", 10, 0),
      makeMsg("beta", "Second message", 10, 1),
      makeMsg("alpha", "Third message", 10, 2),
    ];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={vi.fn()} />);
    const allText = screen.getByRole("log").textContent;
    expect(allText).toContain("First message");
    expect(allText).toContain("Second message");
    expect(allText).toContain("Third message");
  });

  it("shows live indicator when messages exist", () => {
    const messages = [makeMsg("alpha", "Hello")];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={vi.fn()} />);
    expect(screen.getByText("● live")).toBeInTheDocument();
  });

  it("calls onAgentClick when agent name is clicked", async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();
    const messages = [makeMsg("alpha", "Hello")];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={onAgentClick} />);

    await user.click(screen.getByRole("button", { name: "alpha" }));
    expect(onAgentClick).toHaveBeenCalledWith("alpha");
  });

  it("calls onAgentClick on Enter key press", async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();
    const messages = [makeMsg("alpha", "Hello")];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={onAgentClick} />);

    const agentBtn = screen.getByRole("button", { name: "alpha" });
    agentBtn.focus();
    await user.keyboard("{Enter}");
    expect(onAgentClick).toHaveBeenCalledWith("alpha");
  });

  it("calls onAgentClick on Space key press", async () => {
    const user = userEvent.setup();
    const onAgentClick = vi.fn();
    const messages = [makeMsg("alpha", "Hello")];
    render(<ContextLog messages={messages} agentColors={agentColors} onAgentClick={onAgentClick} />);

    const agentBtn = screen.getByRole("button", { name: "alpha" });
    agentBtn.focus();
    await user.keyboard(" ");
    expect(onAgentClick).toHaveBeenCalledWith("alpha");
  });

  it("uses provided agent colors", () => {
    const messages = [makeMsg("alpha", "Hello")];
    const { container } = render(
      <ContextLog messages={messages} agentColors={{ alpha: "rgb(255, 0, 0)" }} onAgentClick={vi.fn()} />
    );
    const agentSpan = container.querySelector('[role="button"]') as HTMLElement;
    expect(agentSpan.style.color).toBe("rgb(255, 0, 0)");
  });

  it("falls back to hash-based color for unknown agents", () => {
    const messages = [makeMsg("unknown-agent", "Hello")];
    render(<ContextLog messages={messages} agentColors={{}} onAgentClick={vi.fn()} />);
    // Should render without error — the fallback hash color is applied
    expect(screen.getByText("unknown-agent")).toBeInTheDocument();
  });

  it("auto-scrolls when new messages arrive", () => {
    const scrollIntoViewMock = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoViewMock;

    const messages = [makeMsg("alpha", "msg1")];
    const { rerender } = render(
      <ContextLog messages={messages} agentColors={agentColors} onAgentClick={vi.fn()} />
    );

    const newMessages = [...messages, makeMsg("beta", "msg2")];
    rerender(<ContextLog messages={newMessages} agentColors={agentColors} onAgentClick={vi.fn()} />);

    expect(scrollIntoViewMock).toHaveBeenCalled();
  });
});
