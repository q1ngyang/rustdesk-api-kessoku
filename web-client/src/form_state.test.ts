import { describe, expect, it } from "vitest";
import { formDisabledState } from "./form_state";

describe("Web Client form state", () => {
  it("disables hidden required account fields after an admin grant", () => {
    expect(formDisabledState(false, false, true)).toEqual({ account: true, connection: false });
  });

  it("restores account fields after grant cleanup", () => {
    expect(formDisabledState(false, false, false)).toEqual({ account: false, connection: false });
  });

  it("locks every field while connecting or connected", () => {
    expect(formDisabledState(false, true, false)).toEqual({ account: true, connection: true });
    expect(formDisabledState(true, false, true)).toEqual({ account: true, connection: true });
  });
});
