import { describe, expect, it } from "vitest";
import { approvedRelay, parseProfile } from "./profile";

const valid = {
  schema_version: 1,
  profile_generation: 9,
  api_origin: "https://api.example.test",
  rendezvous_wss_url: "wss://id.example.test/ws/id",
  relay_wss_urls: { "relay-a": "wss://relay.example.test/ws/relay" },
  server_public_key: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
  server_key_fingerprint: "sha256:66687aadf862bd776c8fc18b8e9f8e20089714856ee233b3902a591d0d5f2925",
  branding: {
    title: "Example Remote",
    logo_light_url: "https://api.example.test/media/branding/logo-light.png",
    logo_dark_url: "https://api.example.test/media/branding/logo-dark.png",
    icon_light_url: "",
    icon_dark_url: "",
    background_light_url: "",
    background_dark_url: "",
    footer_html: "",
  },
  preferences: { language: "en", theme: "light" },
};

describe("fixed profile", () => {
  it("accepts and freezes the exact schema", () => {
    const profile = parseProfile(valid);
    expect(profile.generation).toBe(9);
    expect(profile.apiOrigin).toBe("https://api.example.test");
	expect(profile.branding.title).toBe("Example Remote");
    expect(profile.preferences).toEqual({ language: "en", theme: "light" });
    expect(approvedRelay(profile, "relay-a")).toBe("wss://relay.example.test/ws/relay");
    expect(Object.isFrozen(profile)).toBe(true);
  });

  it.each([
    { ...valid, rendezvous_wss_url: "ws://id.example.test/ws/id" },
    { ...valid, rendezvous_wss_url: "wss://id.example.test/wrong" },
    { ...valid, api_origin: "https://api.example.test/path" },
    { ...valid, relay_wss_urls: { evil: "wss://relay.example.test/ws/relay?token=x" } },
    { ...valid, branding: { ...valid.branding, logo_light_url: "http://cdn.example.test/logo.png" } },
    { ...valid, branding: { ...valid.branding, icon_dark_url: "https://cdn.example.test/icon.png#fragment" } },
    { ...valid, preferences: { language: "unsupported", theme: "light" } },
    { ...valid, preferences: { language: "en", theme: "sepia" } },
    { ...valid, extra: true },
  ])("rejects endpoint or schema policy violations", (candidate) => {
    expect(() => parseProfile(candidate)).toThrow();
  });

  it("rejects a server-selected Relay outside the exact map", () => {
    expect(() => approvedRelay(parseProfile(valid), "relay-b")).toThrow(/unapproved/);
  });

  it("accepts an operator-configured HTTPS brand asset", () => {
    const profile = parseProfile({ ...valid, branding: { ...valid.branding, logo_dark_url: "https://cdn.example.test/brand/logo.png?v=2" } });
    expect(profile.branding.logoDarkUrl).toBe("https://cdn.example.test/brand/logo.png?v=2");
  });
});
