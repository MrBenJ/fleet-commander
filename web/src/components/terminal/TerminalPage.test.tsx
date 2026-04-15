import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act } from "@testing-library/react";

// Mock xterm and addons before importing the component
const mockOpen = vi.fn();
const mockWrite = vi.fn();
const mockDispose = vi.fn();
const mockLoadAddon = vi.fn();
const mockOnData = vi.fn();
const mockOnResize = vi.fn();
let terminalOpts: Record<string, unknown> = {};

vi.mock("@xterm/xterm", () => {
  return {
    Terminal: class MockTerminal {
      cols = 80;
      rows = 24;
      constructor(opts: Record<string, unknown>) {
        terminalOpts = opts;
      }
      open = mockOpen;
      write = mockWrite;
      dispose = mockDispose;
      loadAddon = mockLoadAddon;
      onData = mockOnData;
      onResize = mockOnResize;
    },
  };
});

const mockFit = vi.fn();
vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class MockFitAddon {
    fit = mockFit;
  },
}));

vi.mock("@xterm/addon-web-links", () => ({
  WebLinksAddon: class MockWebLinksAddon {},
}));

vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// Track WebSocket instances
let mockWsInstances: MockWebSocket[] = [];

class MockWebSocket {
  url: string;
  binaryType = "";
  readyState = 1;
  onopen: ((ev: Event) => void) | null = null;
  onmessage: ((ev: MessageEvent) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  send = vi.fn();
  close = vi.fn();

  static OPEN = 1;

  constructor(url: string) {
    this.url = url;
    mockWsInstances.push(this);
  }
}

describe("TerminalPage", () => {
  const originalLocation = window.location;

  beforeEach(() => {
    vi.clearAllMocks();
    mockWsInstances = [];

    Object.defineProperty(window, "WebSocket", { value: MockWebSocket, writable: true });
    Object.defineProperty(window, "location", {
      value: {
        ...originalLocation,
        pathname: "/terminal/alpha-agent",
        protocol: "http:",
        host: "localhost:4242",
      },
      writable: true,
    });
  });

  afterEach(() => {
    Object.defineProperty(window, "location", { value: originalLocation, writable: true });
  });

  it("renders the terminal header with agent name", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(screen.getByText("Terminal: alpha-agent")).toBeInTheDocument();
  });

  it("shows disconnected status initially before ws connects", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    // Before onopen fires, status should be disconnected
    expect(screen.getByRole("status")).toHaveTextContent("disconnected");
  });

  it("shows connected status after ws onopen fires", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    const ws = mockWsInstances[0];
    await act(async () => {
      ws.onopen?.(new Event("open"));
    });
    expect(screen.getByRole("status")).toHaveTextContent("connected");
  });

  it("creates a Terminal instance and opens it", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockOpen).toHaveBeenCalled();
  });

  it("loads FitAddon and WebLinksAddon", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockLoadAddon).toHaveBeenCalledTimes(2);
  });

  it("fits the terminal after opening", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockFit).toHaveBeenCalled();
  });

  it("connects WebSocket to the correct URL", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockWsInstances.length).toBeGreaterThan(0);
    expect(mockWsInstances[0].url).toBe("ws://localhost:4242/ws/terminal/alpha-agent");
  });

  it("sets WebSocket binaryType to arraybuffer", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockWsInstances[0].binaryType).toBe("arraybuffer");
  });

  it("sends initial size on WebSocket open", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    const ws = mockWsInstances[0];
    await act(async () => {
      ws.onopen?.(new Event("open"));
    });
    expect(ws.send).toHaveBeenCalledWith(JSON.stringify({ cols: 80, rows: 24 }));
  });

  it("registers onData handler for terminal input", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockOnData).toHaveBeenCalled();
  });

  it("registers onResize handler", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(mockOnResize).toHaveBeenCalled();
  });

  it("renders the terminal container with correct aria label", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(screen.getByRole("application", { name: /alpha-agent/ })).toBeInTheDocument();
  });

  it("configures terminal with expected theme", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    expect(terminalOpts.cursorBlink).toBe(true);
    expect(terminalOpts.fontSize).toBe(14);
  });

  it("writes disconnect message on ws close", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    await act(async () => {
      render(<TerminalPage />);
    });
    const ws = mockWsInstances[0];
    await act(async () => {
      ws.onclose?.();
    });
    expect(mockWrite).toHaveBeenCalledWith("\r\n\x1b[31m[Disconnected]\x1b[0m\r\n");
  });

  it("disposes terminal and closes ws on unmount", async () => {
    const { TerminalPage } = await import("./TerminalPage");
    let unmount: () => void;
    await act(async () => {
      const result = render(<TerminalPage />);
      unmount = result.unmount;
    });
    const ws = mockWsInstances[0];
    act(() => {
      unmount();
    });
    expect(ws.close).toHaveBeenCalled();
    expect(mockDispose).toHaveBeenCalled();
  });
});
