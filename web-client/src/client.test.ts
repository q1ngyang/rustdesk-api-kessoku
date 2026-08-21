import { describe, expect, it } from "vitest";
import { testDelayResponse } from "./client";

describe("TestDelay interoperability", () => {
  it("answers a peer delay probe without changing its measurements", () => {
    expect(testDelayResponse({
      time: 42n,
      fromClient: false,
      lastDelay: 7,
      targetBitrate: 900,
    })).toEqual({
      time: 42n,
      fromClient: false,
      lastDelay: 7,
      targetBitrate: 900,
    });
  });

  it("ignores a response to a client-originated probe", () => {
    expect(testDelayResponse({
      time: 0n,
      fromClient: true,
      lastDelay: 0,
      targetBitrate: 0,
    })).toBeUndefined();
  });
});
