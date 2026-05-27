/**
 * WebSocket hook — subscribes to /v1/live and feeds incoming positions
 * straight into react-query's cache. Auto-reconnects with exponential backoff.
 */
import { useEffect, useRef } from 'react';
import { useQueryClient } from '@tanstack/react-query';

import { liveWebSocketURL } from '../lib/api';
import type { Position, WSMessage, TruckSummary } from '../lib/types';

export function useLivePositions(enabled = true) {
  const qc = useQueryClient();
  const wsRef = useRef<WebSocket | null>(null);
  const backoff = useRef(1000);

  useEffect(() => {
    if (!enabled) return;
    let stopped = false;

    function connect() {
      const url = liveWebSocketURL();
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => { backoff.current = 1000; };

      ws.onmessage = (e) => {
        try {
          const msg: WSMessage = JSON.parse(e.data);
          if (msg.type === 'position') {
            applyPositionToCache(qc, msg.data);
          }
        } catch { /* ignore parse errors */ }
      };

      ws.onclose = () => {
        if (stopped) return;
        // Exponential backoff capped at 30s.
        const wait = Math.min(backoff.current, 30000);
        backoff.current = Math.min(backoff.current * 2, 30000);
        setTimeout(connect, wait);
      };

      ws.onerror = () => {
        try { ws.close(); } catch { /* noop */ }
      };
    }

    connect();
    return () => {
      stopped = true;
      wsRef.current?.close();
    };
  }, [enabled, qc]);
}

function applyPositionToCache(qc: ReturnType<typeof useQueryClient>, p: Position) {
  // Update the trucks list cache.
  qc.setQueryData<TruckSummary[]>(['trucks'], (prev) => {
    if (!prev) return prev;
    return prev.map((t) => (t.device.id === p.deviceId ? { ...t, latest: p } : t));
  });
  // Update single-truck cache if present.
  qc.setQueryData<TruckSummary>(['truck', p.deviceId], (prev) => {
    if (!prev) return prev;
    return { ...prev, latest: p };
  });
}
