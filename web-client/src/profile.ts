import { strictBase64 } from "./bytes";
import { LIMITS } from "./limits";

export const PROFILE_PATH = "/config/v1.json";

export interface ClientProfile {
  readonly generation: number;
  readonly apiOrigin: string;
  readonly rendezvousUrl: string;
  readonly relayUrls: Readonly<Record<string, string>>;
  readonly serverPublicKey: Uint8Array;
  readonly serverPublicKeyText: string;
  readonly serverKeyFingerprint: string;
	readonly branding: ClientBranding;
	readonly preferences: ClientPreferences;
}

export interface ClientBranding {
		readonly title: string;
		readonly logoLightUrl: string;
		readonly logoDarkUrl: string;
		readonly iconLightUrl: string;
		readonly iconDarkUrl: string;
		readonly backgroundLightUrl: string;
		readonly backgroundDarkUrl: string;
		readonly footerHtml: string;
}

export interface ClientPreferences {
	readonly language: string;
	readonly theme: "" | "light" | "dark";
}

function record(value: unknown): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error("Profile must be a JSON object");
  }
  return value as Record<string, unknown>;
}

function exactUrl(value: unknown, protocol: "https:" | "wss:", path?: string): string {
  if (typeof value !== "string" || value.length === 0 || value.length > LIMITS.controlText) {
    throw new Error("Invalid profile URL");
  }
  const url = new URL(value);
  if (
    url.protocol !== protocol ||
    url.username !== "" ||
    url.password !== "" ||
    url.search !== "" ||
    url.hash !== "" ||
    (path !== undefined && url.pathname !== path)
  ) {
    throw new Error("Profile URL violates the fixed endpoint policy");
  }
  return url.href;
}

export function parseProfile(input: unknown): ClientProfile {
  const value = record(input);
  const allowed = new Set([
    "schema_version",
    "profile_generation",
    "api_origin",
    "rendezvous_wss_url",
    "relay_wss_urls",
    "server_public_key",
    "server_key_fingerprint",
	"branding",
	"preferences",
  ]);
  if (Object.keys(value).some((key) => !allowed.has(key))) throw new Error("Unknown profile field");
  if (value.schema_version !== 1) throw new Error("Unsupported profile schema");
  if (!Number.isSafeInteger(value.profile_generation) || (value.profile_generation as number) <= 0) {
    throw new Error("Profile generation must be a positive integer");
  }
  const apiUrl = exactUrl(value.api_origin, "https:");
  if (new URL(apiUrl).pathname !== "/") throw new Error("API value must be an origin");
  const rendezvousUrl = exactUrl(value.rendezvous_wss_url, "wss:", "/ws/id");
  const relayInput = record(value.relay_wss_urls);
  const relayEntries = Object.entries(relayInput);
  if (relayEntries.length === 0 || relayEntries.length > 64) throw new Error("Invalid Relay allowlist");
  const relayUrls: Record<string, string> = Object.create(null) as Record<string, string>;
  for (const [name, candidate] of relayEntries) {
    if (name.length === 0 || name.length > 255 || name.trim() !== name) throw new Error("Invalid Relay name");
    relayUrls[name] = exactUrl(candidate, "wss:", "/ws/relay");
  }
  const serverPublicKeyText = String(value.server_public_key ?? "");
  const serverPublicKey = strictBase64(serverPublicKeyText);
  if (serverPublicKey.byteLength !== 32) throw new Error("Server Ed25519 key must be 32 bytes");
  if (typeof value.server_key_fingerprint !== "string" || !/^sha256:[a-f0-9]{64}$/.test(value.server_key_fingerprint)) {
    throw new Error("Invalid server key fingerprint");
  }
	const brandingInput = record(value.branding);
	const brandingKeys = ["title", "logo_light_url", "logo_dark_url", "icon_light_url", "icon_dark_url", "background_light_url", "background_dark_url", "footer_html"];
	if (Object.keys(brandingInput).some((key) => !brandingKeys.includes(key)) || brandingKeys.some((key) => !(key in brandingInput))) {
		throw new Error("Invalid branding fields");
	}
	if (typeof brandingInput.title !== "string" || brandingInput.title.length > 120) throw new Error("Invalid branding title");
	if (typeof brandingInput.footer_html !== "string" || brandingInput.footer_html.length > 2048 || brandingInput.footer_html.includes("\0")) throw new Error("Invalid branding footer");
	const brandingURL = (candidate: unknown): string => {
		if (candidate === "") return "";
		if (typeof candidate !== "string" || candidate.length > 512) throw new Error("Invalid branding image URL");
		const parsed = new URL(candidate);
		if (parsed.protocol !== "https:" || parsed.username !== "" || parsed.password !== "" || parsed.hash !== "") {
			throw new Error("Invalid branding image URL");
		}
		return parsed.href;
	};
	const preferenceInput = record(value.preferences);
	if (Object.keys(preferenceInput).some(key => key !== "language" && key !== "theme") || !("language" in preferenceInput) || !("theme" in preferenceInput)) {
		throw new Error("Invalid preference fields");
	}
	const allowedLanguages = new Set(["", "zh-CN", "zh-TW", "en", "fr", "es", "ru", "ko", "ja"]);
	if (typeof preferenceInput.language !== "string" || !allowedLanguages.has(preferenceInput.language)) throw new Error("Invalid language preference");
	if (preferenceInput.theme !== "" && preferenceInput.theme !== "light" && preferenceInput.theme !== "dark") throw new Error("Invalid theme preference");
  return Object.freeze({
    generation: value.profile_generation as number,
    apiOrigin: new URL(apiUrl).origin,
    rendezvousUrl,
    relayUrls: Object.freeze(relayUrls),
    serverPublicKey,
    serverPublicKeyText,
    serverKeyFingerprint: value.server_key_fingerprint,
		branding: Object.freeze({
			title: brandingInput.title,
			logoLightUrl: brandingURL(brandingInput.logo_light_url),
			logoDarkUrl: brandingURL(brandingInput.logo_dark_url),
			iconLightUrl: brandingURL(brandingInput.icon_light_url),
			iconDarkUrl: brandingURL(brandingInput.icon_dark_url),
			backgroundLightUrl: brandingURL(brandingInput.background_light_url),
			backgroundDarkUrl: brandingURL(brandingInput.background_dark_url),
			footerHtml: brandingInput.footer_html,
		}),
	preferences: Object.freeze({ language: preferenceInput.language, theme: preferenceInput.theme }),
  });
}

export async function loadProfile(signal?: AbortSignal): Promise<ClientProfile> {
  const init: RequestInit = {
    method: "GET",
    credentials: "same-origin",
    cache: "no-store",
    redirect: "error",
  };
  if (signal !== undefined) init.signal = signal;
  const response = await fetch(PROFILE_PATH, init);
  if (!response.ok || response.type === "opaque") throw new Error("Unable to load the connection profile");
  if (new URL(response.url).origin !== location.origin) throw new Error("Cross-origin profile response rejected");
  const profile = parseProfile(await response.json());
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", new Uint8Array(profile.serverPublicKey).buffer));
  const fingerprint = Array.from(digest, (byte) => byte.toString(16).padStart(2, "0")).join("");
  digest.fill(0);
  if (`sha256:${fingerprint}` !== profile.serverKeyFingerprint) throw new Error("Server key fingerprint mismatch");
  return profile;
}

export function approvedRelay(profile: ClientProfile, name: string): string {
  const endpoint = profile.relayUrls[name];
  if (endpoint === undefined) throw new Error("Rendezvous selected an unapproved Relay");
  return endpoint;
}
