import { describe, expect, it } from "vitest";
import { parseTokenResponse } from "./auth";

const valid = {
  connection_token: "connection-token",
  token_type: "Bearer",
  expires_at: 1_700_000_600,
  expires_in: 600,
  audience: "rustdesk-connect",
  scope: "connect:initiate",
};

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
