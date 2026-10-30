import { describe, expect, it } from "vitest";
import { PRECHECK_REASONS, precheckOf } from "./precheck";

const ready = (status: "True" | "False" | "Unknown", reason: string) => ({
  type: "Ready",
  status,
  reason,
  message: `because ${reason}`,
});

describe("precheckOf", () => {
  it.each(PRECHECK_REASONS)("returns Ready=False with reason %s", (reason) => {
    const condition = ready("False", reason);
    expect(
      precheckOf({
        status: {
          conditions: [
            { ...ready("True", "Ready"), type: "ClusterReady" },
            condition,
          ],
        },
      }),
    ).toEqual(condition);
  });

  it("returns nothing for Ready=True and for another reason", () => {
    expect(
      precheckOf({ status: { conditions: [ready("True", "Ready")] } }),
    ).toBeUndefined();
    expect(
      precheckOf({
        status: { conditions: [ready("False", "ClusterNotReady")] },
      }),
    ).toBeUndefined();
    expect(precheckOf({})).toBeUndefined();
  });

  it("returns nothing for a condition of an older generation", () => {
    expect(
      precheckOf({
        metadata: { generation: 3 },
        status: {
          conditions: [
            { ...ready("False", "SlotConflict"), observedGeneration: 2 },
          ],
        },
      }),
    ).toBeUndefined();
    const current = {
      ...ready("False", "SlotConflict"),
      observedGeneration: 3,
    };
    expect(
      precheckOf({
        metadata: { generation: 3 },
        status: { conditions: [current] },
      }),
    ).toEqual(current);
  });

  it("reads only the Ready condition", () => {
    expect(
      precheckOf({
        status: {
          conditions: [
            { ...ready("False", "SlotConflict"), type: "ClusterReady" },
          ],
        },
      }),
    ).toBeUndefined();
  });

  it("lists the four precheck reasons", () => {
    expect(PRECHECK_REASONS).toEqual([
      "NotAllowed",
      "SlotConflict",
      "MissingInput",
      "InvalidReference",
    ]);
  });
});
