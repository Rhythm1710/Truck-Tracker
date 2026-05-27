/**
 * Typed REST client for the Truck Tracker backend.
 */
import { useAuthStore } from '../store/auth';
import { env } from './env';
import type { ExchangeResponse, TruckSummary, Position } from './types';

const BASE_URL = env.API_BASE.replace(/\/$/, '');

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = useAuthStore.getState().apiToken;
  const headers = new Headers(init.headers);
  headers.set('Content-Type', 'application/json');
  if (token) headers.set('Authorization', `Bearer ${token}`);

  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers });
  if (!res.ok) {
    // 401 → token expired or invalid; clear session so navigator flips to Login.
    if (res.status === 401) {
      useAuthStore.getState().clear();
    }
    const body = await res.text().catch(() => '');
    throw new ApiError(body || res.statusText, res.status);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// --- endpoints ---

export function exchangeFirebaseToken(firebaseIdToken: string) {
  return request<ExchangeResponse>('/v1/auth/exchange', {
    method: 'POST',
    body: JSON.stringify({ firebaseIdToken }),
  });
}

export function listTrucks() {
  return request<TruckSummary[]>('/v1/trucks');
}

export function getTruck(id: string) {
  return request<TruckSummary>(`/v1/trucks/${encodeURIComponent(id)}`);
}

export function getHistory(id: string, fromISO: string, toISO: string, limit = 500) {
  const params = new URLSearchParams({ from: fromISO, to: toISO, limit: String(limit) });
  return request<Position[]>(`/v1/trucks/${encodeURIComponent(id)}/history?${params}`);
}

// --- WS URL helper ---

export function liveWebSocketURL() {
  const token = useAuthStore.getState().apiToken ?? '';
  const httpUrl = new URL(`${BASE_URL}/v1/live`);
  httpUrl.protocol = httpUrl.protocol === 'https:' ? 'wss:' : 'ws:';
  httpUrl.searchParams.set('token', token);
  return httpUrl.toString();
}
