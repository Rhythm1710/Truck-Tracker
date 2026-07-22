/**
 * Legacy hook kept as a thin shim around the new LiveProvider context.
 *
 * Old screens called `useLivePositions()`; the new model is "the provider
 * owns the connection." Calling this hook now is a no-op that returns the
 * provider's status (so screens can show a banner if they want).
 */
import { useLive } from '../lib/LiveProvider';

export function useLivePositions(_enabled = true) {
  // _enabled is ignored — the provider decides based on auth + AppState.
  return useLive();
}
