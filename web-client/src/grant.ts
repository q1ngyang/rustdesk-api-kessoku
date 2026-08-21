import { LIMITS } from "./limits";
import type { ClientProfile } from "./profile";
import { validatePeerId } from "./wire";

export const GRANT_MESSAGE = "kessoku.web-client.grant.v1";
export const READY_MESSAGE = "kessoku.web-client.ready.v1";
export const ACCEPTED_MESSAGE = "kessoku.web-client.grant-accepted.v1";

export interface DeliveredGrant {
  readonly peerId: string;
  readonly token: string;
  readonly expiresAt: number;
}

export function parseDeliveredGrant(input: unknown, nowSeconds = Math.floor(Date.now() / 1000)): DeliveredGrant {
  if (typeof input !== "object" || input === null || Array.isArray(input)) throw new Error("Invalid admin grant");
  const value = input as Record<string, unknown>;
  const keys = ["type", "peerId", "token", "expiresAt"];
  if (Object.keys(value).length !== keys.length || keys.some((key) => !(key in value)) || value.type !== GRANT_MESSAGE) {
    throw new Error("Invalid admin grant fields");
  }
  if (typeof value.peerId !== "string") throw new Error("Invalid admin peer ID");
  const peerId = validatePeerId(value.peerId);
  if (typeof value.token !== "string" || value.token.length === 0 || value.token.length > LIMITS.token) {
    throw new Error("Invalid admin connection token");
  }
  if (!Number.isSafeInteger(value.expiresAt) || (value.expiresAt as number) <= nowSeconds || (value.expiresAt as number) > nowSeconds + 3660) {
    throw new Error("Invalid admin grant expiry");
  }
  return Object.freeze({ peerId, token: value.token, expiresAt: value.expiresAt as number });
}

// Installs a one-shot, exact-origin opener handoff. No token is accepted from
// a URL, browser storage, a wildcard origin, or any window other than the
// authenticated admin popup opener.
export function receiveAdminGrant(profile: ClientProfile, accept: (grant: DeliveredGrant) => void): () => void {
  const opener = window.opener;
  if (opener === null) return () => undefined;
  let active = true;
  const cleanup = (): void => {
    if (!active) return;
    active = false;
    window.removeEventListener("message", receive);
  };
  const receive = (event: MessageEvent<unknown>): void => {
    if (!active || event.origin !== profile.apiOrigin || event.source !== opener) return;
    let grant: DeliveredGrant;
    try {
      grant = parseDeliveredGrant(event.data);
      accept(grant);
    } catch {
      return;
    }
    opener.postMessage({ type: ACCEPTED_MESSAGE }, profile.apiOrigin);
    cleanup();
    try { window.opener = null; } catch { /* Cross-origin opener may be read-only. */ }
  };
  window.addEventListener("message", receive);
  opener.postMessage({ type: READY_MESSAGE }, profile.apiOrigin);
  return cleanup;
}
