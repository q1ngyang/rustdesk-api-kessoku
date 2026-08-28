import { afterEach, describe, expect, it, vi } from "vitest";
import { browserSession, browserSessionGrant, browserSessionLogout, completeTwoFactor, finishConnectionAudit, login, parseTokenResponse, saveBrowserPreferences, startConnectionAudit } from "./auth";
import { parseProfile } from "./profile";

const valid = {
  connection_token: "connection-token",
  token_type: "Bearer",
  expires_at: 1_700_000_600,
  expires_in: 600,
  audience: "rustdesk-connect",
  scope: "connect:initiate",
};

const profile = parseProfile({
  schema_version: 1,
  profile_generation: 1,
  api_origin: "https://api.example.test",
  rendezvous_wss_url: "wss://id.example.test/ws/id",
  relay_wss_urls: { relay: "wss://relay.example.test/ws/relay" },
  server_public_key: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  server_key_fingerprint: "sha256:66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925",
  branding: { title: "Example Remote", logo_light_url: "", logo_dark_url: "", icon_light_url: "", icon_dark_url: "", background_light_url: "", background_dark_url: "", footer_html: "" },
  preferences: { language: "en", theme: "light" },
});

afterEach(() => vi.restoreAllMocks());

describe("connection token response", () => {
  it("accepts the backend Unix-second contract", () => {
    expect(parseTokenResponse(valid, 1_700_000_000).expires_at).toBe(1_700_000_600);
  });

  it.each([
    { ...valid, expires_at: "2026-08-21T00:00:00Z" },
    { ...valid, expires_at: 1_700_000_000 },
    { ...valid, expires_in: 3601 },
    { ...valid, expires_in: 590 },
    { ...valid, audience: "kessoku-api" },
    { ...valid, extra: true },
  ])("rejects inconsistent or over-authorized responses", (candidate) => {
    expect(() => parseTokenResponse(candidate, 1_700_000_000)).toThrow();
  });
});

describe("two-factor sign-in", () => {
  it("recognizes the exact first-stage challenge contract", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      requires_two_factor: true,
      challenge: "c".repeat(43),
    }), { status: 200, headers: { "Content-Type": "application/json" } }));

    await expect(login(profile, "alice", "password")).resolves.toEqual({
      requiresTwoFactor: true,
      challenge: "c".repeat(43),
    });
    const body = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(body).toMatchObject({ username: "alice", password: "password" });
    expect(body.device_id).toMatch(/browser/i);
    expect(body.platform).toBeTypeOf("string");
    expect(body.uuid).toMatch(/^[0-9a-f-]{36}$/i);
  });

  it("exchanges the bound challenge and six-digit code for a connection token", async () => {
    const now = Math.floor(Date.now() / 1000);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(JSON.stringify({
      ...valid,
      expires_at: now + 600,
    }), { status: 200, headers: { "Content-Type": "application/json" } }));

    await expect(completeTwoFactor(profile, "alice", "c".repeat(43), "123456")).resolves.toBe("connection-token");
    const body = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(body).toMatchObject({
      username: "alice",
      password: "",
      challenge: "c".repeat(43),
      tfa_code: "123456",
    });
    expect(body.device_id).toMatch(/browser/i);
    expect(body.uuid).toMatch(/^[0-9a-f-]{36}$/i);
  });

  it("rejects malformed codes before making a request", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch");
    await expect(completeTwoFactor(profile, "alice", "c".repeat(43), "12345a")).rejects.toThrow(/two-factor/);
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

describe("remembered browser session", () => {
  it("checks and exchanges the HttpOnly session with credentialed requests", async () => {
    const now = Math.floor(Date.now() / 1000);
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({
        authenticated: true, username: "alice", display_name: "Alice", avatar: "https://api.example.test/media/avatars/alice.webp",
        preference_language: "ja", preference_theme: "dark",
      }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ...valid, expires_at: now + 600 }), { status: 200, headers: { "Content-Type": "application/json" } }));

    await expect(browserSession(profile)).resolves.toEqual({
      authenticated: true, username: "alice", displayName: "Alice", avatar: "https://api.example.test/media/avatars/alice.webp",
      language: "ja", theme: "dark",
    });
    await expect(browserSessionGrant(profile)).resolves.toBe("connection-token");
    expect(fetchMock.mock.calls.map(call => call[1]?.credentials)).toEqual(["include", "include"]);
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/session");
    expect(fetchMock.mock.calls[1]?.[0]).toContain("/session/grants");
  });

  it("clears the remembered session through the dedicated endpoint", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 204 }));
    await expect(browserSessionLogout(profile)).resolves.toBeUndefined();
    expect(fetchMock.mock.calls[0]?.[1]?.credentials).toBe("include");
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/session/logout");
  });
});

describe("shared presentation preferences", () => {
  it("persists only language and theme on both the WebClient and API origins", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 204 }));
    await expect(saveBrowserPreferences(profile, { language: "ja", theme: "dark" })).resolves.toBeUndefined();
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/preferences/v1");
    expect(fetchMock.mock.calls[0]?.[1]?.credentials).toBe("same-origin");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({ language: "ja", theme: "dark" });
    expect(fetchMock.mock.calls[1]?.[0]).toContain("/api/web-client/v1/preferences");
    expect(fetchMock.mock.calls[1]?.[1]?.credentials).toBe("include");
  });
});

describe("connection audit lifecycle", () => {
  it("opens and closes an authenticated WebClient connection record", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({ audit_id: 42, session_id: "a".repeat(43), peer_hostname: "design-workstation" }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const audit = await startConnectionAudit(profile, "connection-token", "990100001");
    expect(audit).toEqual({ auditId: 42, sessionId: "a".repeat(43), peerHostname: "design-workstation" });
    const start = JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body));
    expect(start).toMatchObject({ peer_id: "990100001" });
    expect(start.device_id).toMatch(/browser/i);
    expect(start.uuid).toMatch(/^[0-9a-f-]{36}$/i);
    await expect(finishConnectionAudit(profile, "connection-token", audit)).resolves.toBeUndefined();
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({ audit_id: 42, session_id: "a".repeat(43) });
    expect(fetchMock.mock.calls[1]?.[1]?.keepalive).toBe(true);
  });
});
