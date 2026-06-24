import { useEffect, useRef, useState, useCallback } from "react";
import type { WSEvent } from "../types";

interface UseWebSocketOptions {
  onEvent?: (event: WSEvent) => void;
}

export function useWebSocket(path: string, options: UseWebSocketOptions = {}) {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | undefined>(undefined);
  const onEventRef = useRef(options.onEvent);
  // Keep the latest handler in a ref so `connect` (memoized on `path` only)
  // always invokes the current callback without re-subscribing the socket.
  useEffect(() => {
    onEventRef.current = options.onEvent;
  });

  const connect = useCallback(() => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}${path}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      // Runs asynchronously on the socket 'open' event, not synchronously
      // within the effect — not the re-render storm the rule guards against.
      // eslint-disable-next-line @eslint-react/set-state-in-effect
      setConnected(true);
    };

    ws.onmessage = (event) => {
      try {
        const data: WSEvent = JSON.parse(event.data);
        onEventRef.current?.(data);
      } catch {
        // Binary data or unparseable — ignore for event socket
      }
    };

    ws.onclose = () => {
      // Asynchronous 'close' event handler — see the onopen note above.
      // eslint-disable-next-line @eslint-react/set-state-in-effect
      setConnected(false);
      // Reconnect with backoff. `connect` self-references its own
      // useCallback identifier, which the react-hooks lint flags as
      // a TDZ access during render — fine at runtime because the
      // callback runs only after `connect` is fully assigned.
      // eslint-disable-next-line
      reconnectTimerRef.current = window.setTimeout(connect, 2000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [path]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimerRef.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { connected, ws: wsRef };
}
