import { useEffect, useRef, useState, useCallback } from "react";
import type { WSEvent } from "../types";

interface UseWebSocketOptions {
  onEvent?: (event: WSEvent) => void;
}

export function useWebSocket(path: string, options: UseWebSocketOptions = {}) {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<number | undefined>(undefined);
  const onEventRef = useRef(options.onEvent);
  onEventRef.current = options.onEvent;

  const connect = useCallback(() => {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${protocol}//${window.location.host}${path}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
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
      setConnected(false);
      // Reconnect with backoff
      reconnectTimer.current = window.setTimeout(connect, 2000);
    };

    ws.onerror = () => {
      ws.close();
    };
  }, [path]);

  useEffect(() => {
    connect();
    return () => {
      clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  return { connected, ws: wsRef };
}
