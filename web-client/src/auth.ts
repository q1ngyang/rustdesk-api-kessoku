import { LIMITS } from "./limits";
import type { ClientProfile } from "./profile";

interface TokenResponse {
  readonly connection_token: string;
  readonly token_type: "Bearer";
  readonly expires_at: number;
  readonly expires_in: number;
  readonly audience: "rustdesk-connect";
  readonly scope: "connect:initiate";
}

function endpoint(profile: ClientProfile, path: string): string {
  return new URL(path, `${profile.apiOrigin}/`).href;
}

export function parseTokenResponse(input: unknown, nowSeconds = Math.floor(Date.now() / 1000)): TokenResponse {
  if (typeof input !== "object" || input === null || Array.isArray(input)) throw new Error("Invalid token response");
  const value = input as Record<string, unknown>;
  const keys = ["connection_token", "token_type", "expires_at", "expires_in", "audience", "scope"];
  if (Object.keys(value).some((key) => !keys.includes(key)) || keys.some((key) => !(key in value))) {
    throw new Error("Invalid token response fields");
  }
  if (typeof value.connection_token !== "string" || value.connection_token.length === 0 || value.connection_token.length > LIMITS.token) {
    throw new Error("Invalid connection token");
  }
  if (value.token_type !== "Bearer" || value.audience !== "rustdesk-connect" || value.scope !== "connect:initiate") {
    throw new Error("Connection token has the wrong authority");
  }
  if (!Number.isSafeInteger(value.expires_at) || (value.expires_at as number) <= nowSeconds) throw new Error("Invalid token expiry");
  if (!Number.isSafeInteger(value.expires_in) || (value.expires_in as number) <= 0 || (value.expires_in as number) > 3600) {
    throw new Error("Invalid token lifetime");
  }
  const remaining = (value.expires_at as number) - nowSeconds;
  if (remaining > 3660 || Math.abs(remaining - (value.expires_in as number)) > 5) throw new Error("Inconsistent token lifetime");
  return value as unknown as TokenResponse;
}

async function tokenRequest(profile: ClientProfile, path: string, body: object, authorization?: string): Promise<string> {
  const headers: Record<string, string> = { "Content-Type": "application/json", Accept: "application/json" };
  if (authorization !== undefined) headers.Authorization = `Bearer ${authorization}`;
  const response = await fetch(endpoint(profile, path), {
    method: "POST",
    mode: "cors",
    credentials: "omit",
    cache: "no-store",
    redirect: "error",
    referrerPolicy: "no-referrer",
    headers,
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error("Authentication was rejected");
  return parseTokenResponse(await response.json()).connection_token;
}

export function login(profile: ClientProfile, username: string, password: string): Promise<string> {
  if (username.length === 0 || username.length > LIMITS.controlText || password.length === 0 || password.length > LIMITS.passwordBytes) {
    return Promise.reject(new Error("Invalid sign-in credentials"));
  }
  return tokenRequest(profile, "/api/web-client/v1/login", { username, password, platform: "web" });
}

export function exchangeGrant(profile: ClientProfile, apiToken: string): Promise<string> {
  if (apiToken.length === 0 || apiToken.length > LIMITS.token) return Promise.reject(new Error("Invalid API token"));
  return tokenRequest(profile, "/api/web-client/v1/grants", { platform: "web" }, apiToken);
}

export async function logout(profile: ClientProfile, connectionToken: string): Promise<void> {
  if (connectionToken.length === 0) return;
  const response = await fetch(endpoint(profile, "/api/web-client/v1/logout"), {
    method: "POST",
    mode: "cors",
    credentials: "omit",
    cache: "no-store",
    redirect: "error",
    referrerPolicy: "no-referrer",
    headers: { Authorization: `Bearer ${connectionToken}` },
    body: null,
    keepalive: true,
  });
  if (response.status !== 204) throw new Error("Logout was not acknowledged");
}
