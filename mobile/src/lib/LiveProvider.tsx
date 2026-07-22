/**
 * LiveProvider — single hoisted WebSocket connection for the whole app.
 *
 * Before: Home and TruckDetail each ran `useLivePositions()`, opening
 * two parallel WebSockets that pumped identical data.
 *
 * Now: one provider mounted just inside the auth gate owns a single socket,
 * mutates the react-query cache when positions land, and exposes connection
 * state via context for the connectivity banner.
 *
 * Also handles:
 *   - AppState pause/resume so we don't keep the socket open in background.
 *   - Exponential reconnect backoff capped at 30s.
 *   - Auth-aware reconnect: closes immediately when auth clears.
 */
import React, { createContext, useContext, useEffect, useRef, useState } from 'react';
import { AppState, AppStateStatus } from 'react-native';
import { useQueryClient } from '@tanstack/react-query';

import { liveWebSocketURL } from './api';
import { useAuthStore } from '../store/auth';
import type { Position, TruckSummary, WSMessage } from './types';

export type LiveStatus = 'connecting' | 'open' | 'paused' | 'offline';

interface LiveCtxValue {
  status: LiveStatus;
  /** Best-effort timestamp (ms) of the last position the WS pushed in. */
  lastMessageAt: number | null;
}

const LiveCtx = createContext<LiveCtxValue>({ status: 'offline', lastMessageAt: null });

export function useLive(): LiveCtxValue {
  return useContext(LiveCtx);
}

interface Props {
  enabled: boolean;
  children: React.ReactNode;
}

export function LiveProvider({ enabled, children }: Props) {
  const qc = useQueryClient();
  const [status, setStatus] = useState<LiveStatus>('offline');
  const [lastMessageAt, setLastMessageAt] = useState<number | null>(null);

  const wsRef = useRef<WebSocket | null>(null);
  const backoffRef = useRef(1000);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stoppedRef = useRef(false);
  const appStateRef = useRef<AppStateStatus>(AppState.currentState);
  // Re-read the token from the store fresh on every connect — avoids stale closures.
  const authedRef = useRef(enabled);
  authedRef.current = enabled;

  // --- helpers ---

  const closeSocket = (next: LiveStatus = 'offline') => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
    const ws = wsRef.current;
    wsRef.current = null;
    if (ws) {
      ws.onopen = ws.onclose = ws.onerror = ws.onmessage = null;
      try { ws.close(); } catch { /* noop */ }
    }
    setStatus(next);
  };

  const connect = () => {
    if (!authedRef.current) {
      setStatus('offline');
      return;
    }
    if (appStateRef.current !== 'active') {
      setStatus('paused');
      return;
    }
    if (!useAuthStore.getState().apiToken) {
      setStatus('offline');
      return;
    }

    setStatus('connecting');
    const url = liveWebSocketURL();
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      backoffRef.current = 1000;
      setStatus('open');
    };

    ws.onmessage = (e) => {
      try {
        const msg: WSMessage = JSON.parse(e.data);
        if (msg.type === 'position') {
          applyPositionToCache(qc, msg.data);
          setLastMessageAt(Date.now());
        }
      } catch { /* ignore */ }
    };

    ws.onerror = () => {
      try { ws.close(); } catch { /* noop */ }
    };

    ws.onclose = () => {
      wsRef.current = null;
      if (stoppedRef.current || !authedRef.current) {
        setStatus('offline');
        return;
      }
      if (appStateRef.current !== 'active') {
        setStatus('paused');
        return;
      }
      setStatus('offline');
      const wait = Math.min(backoffRef.current, 30000);
      backoffRef.current = Math.min(backoffRef.current * 2, 30000);
      reconnectTimerRef.current = setTimeout(connect, wait);
    };
  };

  // --- lifecycle ---

  // Connect / disconnect based on auth.
  useEffect(() => {
    stoppedRef.current = false;
    if (enabled) {
      connect();
    } else {
      closeSocket('offline');
    }
    return () => {
      stoppedRef.current = true;
      closeSocket('offline');
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  // Pause on background, reconnect on foreground.
  useEffect(() => {
    const sub = AppState.addEventListener('change', (next) => {
      appStateRef.current = next;
      if (next === 'active') {
        if (enabled && !wsRef.current) {
          backoffRef.current = 1000;
          connect();
        }
      } else {
        closeSocket('paused');
      }
    });
    return () => sub.remove();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  return (
    <LiveCtx.Provider value={{ status, lastMessageAt }}>
      {children}
    </LiveCtx.Provider>
  );
}

// ---------------------------------------------------------------------------
// cache mutation
// ---------------------------------------------------------------------------

function applyPositionToCache(qc: ReturnType<typeof useQueryClient>, p: Position) {
  qc.setQueryData<TruckSummary[]>(['trucks'], (prev: TruckSummary[] | undefined) => {
    if (!prev) return prev;
    return prev.map((t: TruckSummary) =>
      t.device.id === p.deviceId ? { ...t, latest: p } : t,
    );
  });
  qc.setQueryData<TruckSummary>(['truck', p.deviceId], (prev: TruckSummary | undefined) => {
    if (!prev) return prev;
    return { ...prev, latest: p };
  });
}
