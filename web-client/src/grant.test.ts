import { describe, expect, it } from "vitest";
import { GRANT_MESSAGE, parseDeliveredGrant } from "./grant";

const valid = {
  type: GRANT_MESSAGE,
  peerId: "900000102",
  token: "short-lived-connection-token",
  expiresAt: 1_700_000_600,
};

describe("admin grant handoff", () => {
  it("accepts the exact bounded one-shot DTO", () => {
    expect(parseDeliveredGrant(valid, 1_700_000_000)).toEqual({
      peerId: "900000102",
      token: "short-lived-connection-token",
      expiresAt: 1_700_000_600,
    });
  });

  it.each([
    { ...valid, type: "other" },
    { ...valid, peerId: "bad/id" },
    { ...valid, token: "" },
    { ...valid, expiresAt: 1_700_000_000 },
    { ...valid, expiresAt: 1_700_003_700 },
    { ...valid, extra: true },
  ])("rejects malformed, expired, excessive, or extended grants", (candidate) => {
    expect(() => parseDeliveredGrant(candidate, 1_700_000_000)).toThrow();
  });
});
