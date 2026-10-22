import { describe, expect, it } from "vitest";
import { checkAllowed } from "./allowed";

describe("checkAllowed with a registration named after a key of Object", () => {
  it("refuses a parameter of a registration that the snapshot has no parameters for", () => {
    const got = checkAllowed({
      policy: {
        sections: { dns: { allowed: ["constructor"] } },
        parameters: { other: {} },
      },
      selections: [
        {
          section: "dns",
          registration: "constructor",
          parameters: { name: "zone" },
        },
      ],
    });

    expect(got).toEqual({
      refusal: {
        reason: "NotAllowed",
        message:
          'parameter "name" of registration "constructor" is not claim-editable',
      },
    });
  });
});
