import { LIMITS } from "./limits";
import type { ClientProfile } from "./profile";
import { webDeviceIdentity } from "./device";

interface TokenResponse {
  readonly connection_token: string;
  readonly token_type: "Bearer";
  readonly expires_at: number;
  readonly expires_in: number;
  readonly audience: "rustdesk-connect";
  readonly scope: "connect:initiate";
}

export interface TwoFactorRequired { readonly requiresTwoFactor: true; readonly challenge: string }
export interface ConnectionAudit { readonly auditId: number; readonly sessionId: string; readonly peerHostname: string }

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

async function authRequest(profile: ClientProfile, path: string, body: object, authorization?: string): Promise<unknown> {
  const headers: Record<string, string> = { "Content-Type": "application/json", Accept: "application/json" };
  if (authorization !== undefined) headers.Authorization = `Bearer ${authorization}`;
  const response = await fetch(endpoint(profile, path), {
    method: "POST",
    mode: "cors",
    credentials: "include",
    cache: "no-store",
    redirect: "error",
    referrerPolicy: "no-referrer",
    headers,
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error("Authentication was rejected");
	return response.json();
}

async function tokenRequest(profile: ClientProfile, path: string, body: object, authorization?: string): Promise<string> {
	return parseTokenResponse(await authRequest(profile, path, body, authorization)).connection_token;
}

export async function login(profile: ClientProfile, username: string, password: string): Promise<string | TwoFactorRequired> {
  if (username.length === 0 || username.length > LIMITS.controlText || password.length === 0 || password.length > LIMITS.passwordBytes) {
    return Promise.reject(new Error("Invalid sign-in credentials"));
  }
	const device = webDeviceIdentity();
	const result = await authRequest(profile, "/api/web-client/v1/login", { username, password, platform: device.platform, device_id: device.device_id, uuid: device.uuid });
	if (typeof result === "object" && result !== null && !Array.isArray(result)) {
		const value = result as Record<string, unknown>;
		if (value.requires_two_factor === true && typeof value.challenge === "string" && value.challenge.length >= 32 && value.challenge.length <= 128 && Object.keys(value).every((key) => key === "requires_two_factor" || key === "challenge")) {
			return { requiresTwoFactor: true, challenge: value.challenge };
		}
	}
	return parseTokenResponse(result).connection_token;
}

export function completeTwoFactor(profile: ClientProfile, username: string, challenge: string, code: string): Promise<string> {
	if (!/^\d{6}$/.test(code) || challenge.length < 32 || challenge.length > 128) return Promise.reject(new Error("Invalid two-factor code"));
	const device = webDeviceIdentity();
	return tokenRequest(profile, "/api/web-client/v1/login", { username, password: "", platform: device.platform, device_id: device.device_id, uuid: device.uuid, challenge, tfa_code: code });
}

export function exchangeGrant(profile: ClientProfile, apiToken: string): Promise<string> {
  if (apiToken.length === 0 || apiToken.length > LIMITS.token) return Promise.reject(new Error("Invalid API token"));
  const device = webDeviceIdentity();
  return tokenRequest(profile, "/api/web-client/v1/grants", { platform: device.platform, device_id: device.device_id, uuid: device.uuid }, apiToken);
}

export async function logout(profile: ClientProfile, connectionToken: string): Promise<void> {
  if (connectionToken.length === 0) return;
  const response = await fetch(endpoint(profile, "/api/web-client/v1/logout"), {
    method: "POST",
    mode: "cors",
    credentials: "include",
    cache: "no-store",
    redirect: "error",
    referrerPolicy: "no-referrer",
    headers: { Authorization: `Bearer ${connectionToken}` },
    body: null,
    keepalive: true,
  });
  if (response.status !== 204) throw new Error("Logout was not acknowledged");
}

export async function startConnectionAudit(profile: ClientProfile, connectionToken: string, peerId: string): Promise<ConnectionAudit> {
  if (connectionToken.length === 0 || connectionToken.length > LIMITS.token || peerId.length === 0 || peerId.length > LIMITS.controlText) throw new Error("Invalid connection audit request");
  const device = webDeviceIdentity();
  const result = await authRequest(profile, "/api/web-client/v1/connections/audit/start", {
    peer_id: peerId, device_id: device.device_id, uuid: device.uuid, platform: device.platform,
  }, connectionToken);
  if (typeof result !== "object" || result === null || Array.isArray(result)) throw new Error("Invalid connection audit response");
  const value = result as Record<string, unknown>;
  const allowed = new Set(["audit_id", "session_id", "peer_hostname"]);
  if (Object.keys(value).length !== 3 || Object.keys(value).some((key) => !allowed.has(key)) || !Number.isSafeInteger(value.audit_id) || (value.audit_id as number) <= 0 || typeof value.session_id !== "string" || value.session_id.length < 32 || value.session_id.length > 128 || typeof value.peer_hostname !== "string" || value.peer_hostname.length > LIMITS.controlText) {
    throw new Error("Invalid connection audit response");
  }
  return { auditId: value.audit_id as number, sessionId: value.session_id, peerHostname: value.peer_hostname };
}

export async function finishConnectionAudit(profile: ClientProfile, connectionToken: string, audit: ConnectionAudit): Promise<void> {
  if (connectionToken.length === 0 || connectionToken.length > LIMITS.token || !Number.isSafeInteger(audit.auditId) || audit.auditId <= 0 || audit.sessionId.length < 32 || audit.sessionId.length > 128) return;
  const response = await fetch(endpoint(profile, "/api/web-client/v1/connections/audit/finish"), {
    method: "POST", mode: "cors", credentials: "include", cache: "no-store", redirect: "error", referrerPolicy: "no-referrer",
    headers: { Authorization: `Bearer ${connectionToken}`, "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ audit_id: audit.auditId, session_id: audit.sessionId }), keepalive: true,
  });
  if (response.status !== 204) throw new Error("Connection audit close was not acknowledged");
}

export interface BrowserSession {
  readonly authenticated: boolean;
  readonly username: string;
  readonly displayName: string;
  readonly avatar: string;
  readonly language: string;
  readonly theme: "" | "light" | "dark";
}

export async function browserSession(profile: ClientProfile): Promise<BrowserSession> {
  const result = await authRequest(profile, "/api/web-client/v1/session", {});
  if (typeof result !== "object" || result === null || Array.isArray(result)) throw new Error("Invalid browser session response");
  const value = result as Record<string, unknown>;
  const allowed = new Set(["authenticated", "username", "display_name", "avatar", "preference_language", "preference_theme"]);
  if (typeof value.authenticated !== "boolean" || Object.keys(value).some(key => !allowed.has(key))) throw new Error("Invalid browser session response");
  if (!value.authenticated) return { authenticated: false, username: "", displayName: "", avatar: "", language: "", theme: "" };
  if (typeof value.username !== "string" || value.username.length === 0 || value.username.length > LIMITS.controlText) throw new Error("Invalid browser session identity");
  if (typeof value.display_name !== "string" || value.display_name.length === 0 || value.display_name.length > LIMITS.controlText) throw new Error("Invalid browser session display name");
  if (typeof value.avatar !== "string" || value.avatar.length > 512) throw new Error("Invalid browser session avatar");
  if (value.avatar !== "") {
    const avatar = new URL(value.avatar);
    if (avatar.protocol !== "https:" || avatar.origin !== profile.apiOrigin || !avatar.pathname.startsWith("/media/avatars/") || avatar.username !== "" || avatar.password !== "" || avatar.hash !== "") throw new Error("Invalid browser session avatar");
  }
  const languages = new Set(["", "zh-CN", "zh-TW", "en", "ja", "ko", "fr", "es", "ru"]);
  if (typeof value.preference_language !== "string" || !languages.has(value.preference_language)) throw new Error("Invalid browser session language");
  if (value.preference_theme !== "" && value.preference_theme !== "light" && value.preference_theme !== "dark") throw new Error("Invalid browser session theme");
  return {
    authenticated: true, username: value.username, displayName: value.display_name, avatar: value.avatar,
    language: value.preference_language, theme: value.preference_theme,
  };
}

export function browserSessionGrant(profile: ClientProfile): Promise<string> {
  const device = webDeviceIdentity();
  return tokenRequest(profile, "/api/web-client/v1/session/grants", { platform: device.platform, device_id: device.device_id, uuid: device.uuid });
}

export async function establishBrowserSession(profile: ClientProfile, connectionToken: string): Promise<void> {
  if (connectionToken.length === 0 || connectionToken.length > LIMITS.token) throw new Error("Invalid connection token");
  const device = webDeviceIdentity();
  const result = await authRequest(profile, "/api/web-client/v1/session/establish", { platform: device.platform, device_id: device.device_id, uuid: device.uuid }, connectionToken);
  if (typeof result !== "object" || result === null || Array.isArray(result) || (result as Record<string, unknown>).established !== true || Object.keys(result as Record<string, unknown>).length !== 1) {
    throw new Error("Browser session was not established");
  }
}

export async function browserSessionLogout(profile: ClientProfile): Promise<void> {
  const response = await fetch(endpoint(profile, "/api/web-client/v1/session/logout"), { method: "POST", mode: "cors", credentials: "include", cache: "no-store", redirect: "error", referrerPolicy: "no-referrer", headers: { "Content-Type": "application/json", Accept: "application/json" }, body: "{}" });
  if (response.status !== 204) throw new Error("Session logout was not acknowledged");
}

export async function saveBrowserPreferences(profile: ClientProfile, preferences: { language?: string; theme?: "light" | "dark" }): Promise<void> {
	const body = JSON.stringify({ language: preferences.language ?? "", theme: preferences.theme ?? "" });
	const local = await fetch("/preferences/v1", {
		method: "POST", credentials: "same-origin", cache: "no-store", redirect: "error",
		headers: { "Content-Type": "application/json", Accept: "application/json" }, body,
	});
	if (local.status !== 204) throw new Error("Preference update was not acknowledged");
	try {
		// Keep the API-origin compatibility cookie current when the browser allows
		// it, but never discard the first-party WebClient preference if a privacy
		// policy blocks this optional cross-origin write.
		const response = await fetch(endpoint(profile, "/api/web-client/v1/preferences"), {
			method: "POST", mode: "cors", credentials: "include", cache: "no-store", redirect: "error", referrerPolicy: "no-referrer",
			headers: { "Content-Type": "application/json", Accept: "application/json" }, body,
		});
		if (response.status !== 204) throw new Error("API preference update was not acknowledged");
	} catch { /* The WebClient-origin cookie above is authoritative. */ }
}
