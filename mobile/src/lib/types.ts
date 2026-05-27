/**
 * Shared API types — keep in sync with backend internal/models/models.go.
 *
 * If you change the backend wire format, change here too. We don't auto-generate
 * because the surface is small; revisit if it grows.
 */
import type { Status } from '../theme';

export interface Position {
  deviceId: string;
  imei: string;
  lat: number;
  lng: number;
  speed: number;        // km/h
  heading: number;      // degrees
  ignition: boolean;
  deviceAt: string;     // RFC 3339 timestamp from the tracker
  at: string;           // RFC 3339 timestamp from the server
}

export interface Device {
  id: string;
  imei: string;
  plate: string;
  label?: string;
  ownerId: string;
}

export interface TruckSummary {
  device: Device;
  latest?: Position;
}

export interface ExchangeResponse {
  apiToken: string;
  expiresAt: string;
}

export type WSMessage =
  | { type: 'position'; data: Position };

/**
 * Derive a human-readable status string from a Position. Mirrors the
 * server-side `(*Position).Status()` in Go.
 */
export function statusFromPosition(p: Position | undefined): Status {
  if (!p) return 'Offline';
  const ageMins = (Date.now() - new Date(p.at).getTime()) / 60000;
  if (ageMins > 15) return 'Offline';
  if (p.ignition && p.speed > 5) return 'Running';
  if (p.ignition) return 'Idle';
  return 'Stopped';
}

export function formatRelativeTime(iso: string | undefined): string {
  if (!iso) return '—';
  const ageSecs = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (ageSecs < 60) return 'just now';
  const ageMins = Math.floor(ageSecs / 60);
  if (ageMins < 60) return `${ageMins} min ago`;
  const ageHours = Math.floor(ageMins / 60);
  if (ageHours < 24) return `${ageHours} hr ago`;
  return `${Math.floor(ageHours / 24)} day ago`;
}
