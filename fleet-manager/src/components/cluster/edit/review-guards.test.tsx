// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import { openSession, setValue, type Session } from "@/lib/session";
import {
  claim,
  cluster,
  slotContext,
} from "../../../../test/fakes/slot-context";
import type { BackupObject } from "../backups";
import type { ClusterSlotContext } from "../slots";
import type { ClusterClaim } from "./cluster-edit";
import { reviewGuards } from "./review-guards";

afterEach(cleanup);

const releaseRef: Step[] = [{ field: "spec" }, { field: "releaseRef" }];
const allow: Step[] = [{ field: "spec" }, { field: "allowVersionDowngrade" }];
const suspend: Step[] = [{ field: "spec" }, { field: "suspend" }];
const link = { backupsHref: "?tab=backups" };
const DOWNGRADE =
  "Camunda does not support a downgrade of a running cluster. This change moves the cluster from 8.9.1 to 8.9.0.";

const running = {
  kind: "LogicalBackupRDBMS",
  metadata: { namespace: "team-a", name: "nightly" },
  spec: { clusterRef: { name: "orders" } },
  status: { phase: "Running" },
} as unknown as BackupObject;

function sessionOf(
  context: ClusterSlotContext,
  changes: [Step[], unknown][],
): Session<ClusterClaim> {
  let s = openSession({
    claim: context.claim ?? claim(),
    resource: context.cluster ?? cluster(),
    locks: () => undefined,
  })!;
  for (const [steps, value] of changes) {
    const next = setValue(s, steps, value);
    if (!("session" in next)) throw new Error("refused");
    s = next.session;
  }
  return s;
}

const downgrade: [Step[], unknown][] = [
  [releaseRef, "camunda-8-9-0"],
  [allow, "8.9.0"],
];

describe("reviewGuards", () => {
  it("names both versions of a downgrade and asks for the downgrade confirm", () => {
    const context = slotContext();
    const guards = reviewGuards(
      context,
      sessionOf(context, downgrade),
      "direct",
      link,
    );
    render(<>{guards.caution}</>);
    expect(screen.getByText(DOWNGRADE)).toBeTruthy();
    expect(guards.confirm).toEqual({
      danger: true,
      label: "Downgrade",
      gitLabel: "Push downgrade",
    });
  });

  it("gives no caution and no confirm to a change that is neither a downgrade nor a risk to a backup", () => {
    const context = slotContext();
    const guards = reviewGuards(
      context,
      sessionOf(context, [[suspend, true]]),
      "direct",
      link,
    );
    expect(guards.caution).toBeUndefined();
    expect(guards.confirm).toBeUndefined();
    expect(guards.impacts.length).toBeGreaterThan(0);
  });

  it("shows the downgrade caution before the backup caution, with the confirm of the backup caution", () => {
    const context = slotContext({ backups: [running] });
    const guards = reviewGuards(
      context,
      sessionOf(context, downgrade),
      "direct",
      link,
    );
    render(<>{guards.caution}</>);
    const text = document.body.textContent ?? "";
    expect(text.indexOf(DOWNGRADE)).toBeGreaterThanOrEqual(0);
    expect(text.indexOf("Wait until backup nightly finishes.")).toBeGreaterThan(
      text.indexOf(DOWNGRADE),
    );
    expect(guards.confirm?.label).toBe("Change anyway");
  });
});
