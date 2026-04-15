import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useWebSocket } from "./useWebSocket";

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  closeCalled = false;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  close() {
    this.closeCalled = true;
    this.onclose?.();
  }

  simulateOpen() {
    this.onopen?.();
  }

  simulateMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) });
  }

  simulateError() {
    this.onerror?.();
  }
}

beforeEach(() => {
  MockWebSocket.instances = [];
  vi.stubGlobal("WebSocket", MockWebSocket);
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("useWebSocket", () => {
  it("connects on mount and reports connected state", () => {
    const { result } = renderHook(() => useWebSocket("/ws/events"));

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0].url).toContain("/ws/events");
    expect(result.current.connected).toBe(false);

    act(() => {
      MockWebSocket.instances[0].simulateOpen();
    });
    expect(result.current.connected).toBe(true);
  });

  it("calls onEvent when a message arrives", () => {
    const onEvent = vi.fn();
    renderHook(() => useWebSocket("/ws/events", { onEvent }));

    act(() => {
      MockWebSocket.instances[0].simulateOpen();
    });

    const event = { type: "agent_state", agent: "test", state: "working", timestamp: "now" };
    act(() => {
      MockWebSocket.instances[0].simulateMessage(event);
    });
    expect(onEvent).toHaveBeenCalledWith(event);
  });

  it("ignores unparseable messages", () => {
    const onEvent = vi.fn();
    renderHook(() => useWebSocket("/ws/events", { onEvent }));

    act(() => {
      MockWebSocket.instances[0].simulateOpen();
      MockWebSocket.instances[0].onmessage?.({ data: "not json{{{" });
    });
    expect(onEvent).not.toHaveBeenCalled();
  });

  it("reconnects after close with 2s delay", () => {
    renderHook(() => useWebSocket("/ws/events"));

    expect(MockWebSocket.instances).toHaveLength(1);

    act(() => {
      MockWebSocket.instances[0].simulateOpen();
    });

    // Simulate close
    act(() => {
      MockWebSocket.instances[0].onclose?.();
    });

    expect(MockWebSocket.instances).toHaveLength(1); // not yet reconnected

    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(MockWebSocket.instances).toHaveLength(2); // reconnected
  });

  it("closes WebSocket and clears timer on unmount", () => {
    const { unmount } = renderHook(() => useWebSocket("/ws/events"));

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.simulateOpen();
    });

    unmount();
    expect(ws.closeCalled).toBe(true);
  });

  it("closes connection on error", () => {
    renderHook(() => useWebSocket("/ws/events"));

    const ws = MockWebSocket.instances[0];
    act(() => {
      ws.simulateError();
    });
    expect(ws.closeCalled).toBe(true);
  });
});
