import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import { openSession, setValue, type Session } from "@/lib/session";
import {
  claim,
  cluster,
  slotContext,
} from "../../../../test/fakes/slot-context";
import type { ClusterSlotContext } from "../slots";
import type { ClusterClaim } from "./cluster-edit";
import { downgradeDone, downgradeOf } from "./downgrade";

const releaseRef: Step[] = [{ field: "spec" }, { field: "releaseRef" }];
const suspend: Step[] = [{ field: "spec" }, { field: "suspend" }];

function atVersion(version: string | undefined) {
  const c = cluster();
  return { ...c, status: version ? { management: { version } } : {} };
}

function sessionOf(
  context: ClusterSlotContext,
  changes: [Step[], unknown][],
): Session<ClusterClaim> {
  let s = openSession({
    claim: context.claim,
    resource: context.cluster,
    locks: () => undefined,
  })!;
  for (const [steps, value] of changes) {
    const next = setValue(s, steps, value);
    if (!("session" in next)) throw new Error("refused");
    s = next.session;
  }
  return s;
}

describe("downgradeDone", () => {
  const allowing = (value: string) =>
    claim("41", { allowVersionDowngrade: value });

  it("is true when the cluster reports the version that allowVersionDowngrade names", () => {
    expect(downgradeDone(atVersion("8.9.1"), allowing("8.9.1"))).toBe(true);
  });

  it.each([
    ["another version", atVersion("8.9.3"), allowing("8.9.1")],
    ["no reported version", atVersion(undefined), allowing("8.9.1")],
    ["an empty field", atVersion("8.9.1"), allowing("")],
    ["no field", atVersion("8.9.1"), claim()],
  ])("is false for %s", (_, c, cl) => {
    expect(downgradeDone(c, cl)).toBe(false);
  });
});

describe("downgradeOf", () => {
  const releases = [
    ...slotContext().releases,
    {
      kind: "CamundaRelease",
      metadata: { name: "camunda-8-9-3" },
      spec: { version: "8.9.3" },
    },
  ] as ClusterSlotContext["releases"];

  it("gives both versions of a move to a lower version", () => {
    const context = slotContext({
      cluster: atVersion("8.9.3"),
      claim: claim("41", { releaseRef: "camunda-8-9-3" }),
      releases,
    });
    expect(
      downgradeOf(context, sessionOf(context, [[releaseRef, "camunda-8-9-1"]])),
    ).toEqual({ from: "8.9.3", to: "8.9.1" });
  });

  it.each([
    ["an upgrade", "8.9.0", [[releaseRef, "camunda-8-9-3"]]],
    ["no release change", "8.9.3", [[suspend, true]]],
    ["a cluster without a version", undefined, [[releaseRef, "camunda-8-9-0"]]],
  ] as [string, string | undefined, [Step[], unknown][]][])(
    "is undefined for %s",
    (_, version, changes) => {
      const context = slotContext({ cluster: atVersion(version), releases });
      expect(downgradeOf(context, sessionOf(context, changes))).toBeUndefined();
    },
  );
});
