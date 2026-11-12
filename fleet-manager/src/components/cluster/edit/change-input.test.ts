import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { openSession, type Session } from "@/lib/session";
import { cluster, slotContext } from "../../../../test/fakes/slot-context";
import type { ClusterSlotContext } from "../slots";
import { changeInputOf } from "./change-input";
import type { ClusterClaim } from "./cluster-edit";

const pc = (name: string) =>
  ({
    kind: "CamundaPlatformConfig",
    metadata: { name },
    spec: {},
  }) as unknown as Resource<"CamundaPlatformConfig">;

const storage = {
  kind: "SecondaryStorageConfig",
  metadata: { namespace: "team-a", name: "search" },
  spec: { type: "elasticsearch" },
} as unknown as Resource<"SecondaryStorageConfig">;

function sessionOf(context: ClusterSlotContext): Session<ClusterClaim> {
  return openSession({
    claim: context.claim,
    resource: context.cluster,
    locks: () => undefined,
  })!;
}

describe("changeInputOf", () => {
  it("gives the live cluster, the lists of the page, the platform config of the cluster, and the storage of the page", () => {
    const context = slotContext({
      platformConfigs: [pc("base"), pc("other")],
      cluster: cluster({ platformConfigRef: "base" }),
      storage,
    });
    const session = sessionOf(context);
    const input = changeInputOf(context, session);
    expect(input.session).toBe(session);
    expect(input.live).toBe(context.cluster);
    expect(input.platform?.metadata?.name).toBe("base");
    expect(input.storage).toBe(context.storage);
    expect(input.presets).toBe(context.presets);
    expect(input.releases).toBe(context.releases);
    expect(input.sections).toBe(context.sections);
    expect(input.environment).toBe(context.environment);
  });

  it("gives no platform config when the cluster names none", () => {
    const context = slotContext({ platformConfigs: [pc("base")] });
    expect(changeInputOf(context, sessionOf(context)).platform).toBeUndefined();
  });

  it("gives no platform config when the one the cluster names is not on the page", () => {
    const context = slotContext({
      platformConfigs: [pc("other")],
      cluster: cluster({ platformConfigRef: "base" }),
    });
    expect(changeInputOf(context, sessionOf(context)).platform).toBeUndefined();
  });
});
