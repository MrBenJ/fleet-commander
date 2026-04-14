import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebLinksAddon } from "@xterm/addon-web-links";
import "@xterm/xterm/css/xterm.css";

export function TerminalPage() {
  const termRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const [connected, setConnected] = useState(false);

  // Extract agent name from URL path: /terminal/:agent
  const agentName = window.location.pathname.split("/").pop() || "";

  useEffect(() => {
    if (!termRef.current || !agentName) return;

    const terminal = new Terminal({
      cursorBlink: true,
      theme: {
        background: "#0d1117",
        foreground: "#c9d1d9",
        cursor: "#58a6ff",
        selectionBackground: "#264f78",
      },
      fontFamily: "'SF Mono', 'Fira Code', 'Fira Mono', monospace",
      fontSize: 14,
    });

    const fitAddon = new FitAddon();
    terminal.loadAddon(fitAddon);
    terminal.loadAddon(new WebLinksAddon());

    terminal.open(termRef.current);
    fitAddon.fit();
    terminalRef.current = terminal;

    // Connect WebSocket
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(
      `${protocol}//${window.location.host}/ws/terminal/${agentName}`
    );
    ws.binaryType = "arraybuffer";
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      // Send initial size
      ws.send(
        JSON.stringify({ cols: terminal.cols, rows: terminal.rows })
      );
    };

    ws.onmessage = (event) => {
      if (event.data instanceof ArrayBuffer) {
        terminal.write(new Uint8Array(event.data));
      } else {
        terminal.write(event.data);
      }
    };

    ws.onclose = () => {
      setConnected(false);
      terminal.write("\r\n\x1b[31m[Disconnected]\x1b[0m\r\n");
    };

    // Terminal → WebSocket
    terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(data);
      }
    });

    // Handle resize
    const handleResize = () => {
      fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(
          JSON.stringify({ cols: terminal.cols, rows: terminal.rows })
        );
      }
    };

    terminal.onResize(({ cols, rows }) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ cols, rows }));
      }
    });

    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      ws.close();
      terminal.dispose();
    };
  }, [agentName]);

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100vh",
        background: "#0d1117",
      }}
    >
      {/* Title bar */}
      <div
        style={{
          background: "var(--bg-secondary)",
          padding: "0.5rem 1rem",
          display: "flex",
          alignItems: "center",
          gap: "0.75rem",
          borderBottom: "1px solid var(--border)",
        }}
      >
        <span style={{ fontSize: "0.8rem", color: "var(--text-secondary)" }}>
          {agentName}
        </span>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: "0.4rem" }}>
          <span
            style={{
              width: 8,
              height: 8,
              borderRadius: "50%",
              background: connected ? "var(--green)" : "var(--red)",
              display: "inline-block",
            }}
          />
          <span
            style={{
              fontSize: "0.75rem",
              color: connected ? "var(--green)" : "var(--red)",
            }}
          >
            {connected ? "connected" : "disconnected"}
          </span>
        </div>
      </div>

      {/* Terminal area */}
      <div ref={termRef} style={{ flex: 1, padding: "0.5rem" }} />
    </div>
  );
}
