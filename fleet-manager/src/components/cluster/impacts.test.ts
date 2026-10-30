import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { openSession, resetValue, setValue, type Session } from "@/lib/session";
import {
  clusterImpacts,
  mergedSpecs,
  showsMergedDiff,
  type ClusterChangeInput,
} from "./impacts";

type Claim = Resource<"CamundaClusterClaim">;
const f = (field: string): Step => ({ field });
const cluster = (...names: string[]): Step[] => [
  f("spec"),
  f("cluster"),
  ...names.map(f),
];

const CLUSTER_SPEC = {
  presetRef: "medium",
  releaseRef: "camunda-8-9-5",
  zeebe: { replicas: 3, storageSize: "32Gi" },
  gateway: { replicas: 2 },
  operate: { replicas: 1 },
};

function claim(clusterSpec: Record<string, unknown> = {}): Claim {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a", resourceVersion: "41" },
    spec: {
      presetRef: "medium",
      releaseRef: "camunda-8-9-5",
      cluster: { zeebe: { replicas: 3 }, ...clusterSpec },
      plugins: [
        {
          section: "access",
          registration: "gateway-access",
          parameters: { hostnamePattern: "a" },
        },
      ],
    },
  } as unknown as Claim;
}

function live(spec: Record<string, unknown> = {}): Resource<"CamundaCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      name: "orders",
      namespace: "team-a",
      labels: { [CLAIM_LABEL]: "orders" },
    },
    spec: { ...CLUSTER_SPEC, ...spec },
  } as unknown as Resource<"CamundaCluster">;
}

const preset = (name: string, clusterSpec: Record<string, unknown>) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaClusterPreset",
    metadata: { name },
    spec: { cluster: clusterSpec },
  }) as unknown as Resource<"CamundaClusterPreset">;

const release = (name: string, version: string) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaRelease",
    metadata: { name },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

function input(
  change: (s: Session<Claim>) => Session<Claim>,
  over: { claim?: Claim; live?: Resource<"CamundaCluster"> } = {},
): ClusterChangeInput {
  const s = openSession({
    claim: over.claim ?? claim(),
    resource: { metadata: { labels: { [CLAIM_LABEL]: "orders" } } },
    locks: () => undefined,
  })!;
  return {
    session: change(s),
    live: over.live ?? live(),
    presets: [
      preset("medium", { tasklist: { replicas: 1 }, gateway: { replicas: 1 } }),
      preset("large", { tasklist: { replicas: 2 }, gateway: { replicas: 1 } }),
    ],
    releases: [
      release("camunda-8-9-5", "8.9.5"),
      release("camunda-8-9-5-images", "8.9.5"),
      release("camunda-8-10-1", "8.10.1"),
    ],
    sections: [
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "access" },
        spec: {
          label: "Access",
          slot: "externalAccess",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
  };
}

const set =
  (steps: Step[], value: unknown) =>
  (s: Session<Claim>): Session<Claim> => {
    const result = setValue(s, steps, value);
    if (!("session" in result)) throw new Error("refused");
    return result.session;
  };

const reset =
  (steps: Step[]) =>
  (s: Session<Claim>): Session<Claim> => {
    const result = resetValue(s, steps);
    if (!("session" in result)) throw new Error("refused");
    return result.session;
  };

const both =
  (...changes: ((s: Session<Claim>) => Session<Claim>)[]) =>
  (s: Session<Claim>) =>
    changes.reduce((at, c) => c(at), s);

describe("clusterImpacts", () => {
  it("a release with another Camunda version restarts the brokers one at a time", () => {
    expect(
      clusterImpacts(
        input(set([f("spec"), f("releaseRef")], "camunda-8-10-1")),
      ),
    ).toEqual([
      "The brokers restart one at a time. The other processes restart.",
    ]);
  });

  it("a release with the same Camunda version gives no line", () => {
    expect(
      clusterImpacts(
        input(set([f("spec"), f("releaseRef")], "camunda-8-9-5-images")),
      ),
    ).toEqual([]);
  });

  it("a preset lists each merged field that changes", () => {
    expect(
      clusterImpacts(input(set([f("spec"), f("presetRef")], "large"))),
    ).toEqual(["spec.tasklist.replicas: 1 to 2."]);
  });

  it("a preset field that the new preset drops shows as unset", () => {
    const i = input(set([f("spec"), f("presetRef")], "small"));
    i.presets = [...i.presets, preset("small", { gateway: { replicas: 1 } })];
    expect(clusterImpacts(i)).toEqual(["spec.tasklist.replicas: 1 to unset."]);
  });

  it("a field of a standalone process restarts its pods", () => {
    expect(
      clusterImpacts(input(set(cluster("gateway", "replicas"), 3))),
    ).toEqual(["Gateway pods restart."]);
    expect(clusterImpacts(input(set(cluster("zeebe", "replicas"), 5)))).toEqual(
      ["Broker pods restart."],
    );
  });

  it("two fields of one process give one line", () => {
    expect(
      clusterImpacts(
        input(
          both(
            set(cluster("zeebe", "replicas"), 5),
            set(cluster("zeebe", "storageSize"), "64Gi"),
          ),
        ),
      ),
    ).toEqual(["Broker pods restart."]);
  });

  it("a field of an embedded process applies when it runs standalone", () => {
    expect(
      clusterImpacts(input(set(cluster("tasklist", "replicas"), 2))),
    ).toEqual(["Applies when Tasklist runs standalone."]);
  });

  it("a cluster-wide field restarts every process", () => {
    expect(
      clusterImpacts(
        input(
          set([...cluster("extraEnv"), { item: { name: "TZ" } }], {
            name: "TZ",
            value: "UTC",
          }),
        ),
      ),
    ).toEqual([
      "Every process restarts, except the processes that set the field themselves.",
    ]);
  });

  it("an app that becomes standalone creates its Deployment and starts from the preset", () => {
    expect(
      clusterImpacts(input(set(cluster("tasklist", "mode"), "Standalone"))),
    ).toEqual([
      "Creates Deployment orders-tasklist and Service orders-tasklist.",
      "Tasklist starts from the workload settings of the preset: replicas, CPU, and memory.",
    ]);
  });

  it("an app that becomes standalone starts with the settings prepared in the claim", () => {
    expect(
      clusterImpacts(
        input(set(cluster("tasklist", "mode"), "Standalone"), {
          claim: claim({
            tasklist: { replicas: 2, resources: { requests: { cpu: "1" } } },
          }),
        }),
      ),
    ).toEqual([
      "Creates Deployment orders-tasklist and Service orders-tasklist.",
      "Tasklist starts with the settings prepared in the claim (2 fields).",
    ]);
  });

  it("an app that becomes embedded removes its Deployment and runs in its host", () => {
    expect(
      clusterImpacts(
        input(set(cluster("operate", "mode"), "Embedded"), {
          claim: claim({ operate: { mode: "Standalone" } }),
          live: live({ operate: { mode: "Standalone" } }),
        }),
      ),
    ).toEqual([
      "Removes Deployment orders-operate and Service orders-operate. Operate runs in the gateway, which restarts.",
    ]);
  });

  it("a gateway that becomes embedded removes its Deployment and moves the web apps into the brokers", () => {
    expect(
      clusterImpacts(input(set(cluster("gateway", "mode"), "Embedded"))),
    ).toEqual([
      "Removes Deployment orders-gateway and Service orders-gateway. Gateway runs in the brokers, which restarts.",
    ]);
  });

  it("a smaller storage size cannot shrink the volumes", () => {
    expect(
      clusterImpacts(input(set(cluster("zeebe", "storageSize"), "16Gi"))),
    ).toEqual(["Volumes cannot shrink. The operator keeps the current size."]);
    expect(
      clusterImpacts(input(set(cluster("zeebe", "storageSize"), "64Gi"))),
    ).toEqual(["Broker pods restart."]);
  });

  it("a section parameter applies in its option", () => {
    expect(
      clusterImpacts(
        input(
          set(
            [
              f("spec"),
              f("plugins"),
              { item: { section: "access", registration: "gateway-access" } },
              f("parameters"),
              f("hostnamePattern"),
            ],
            "b",
          ),
        ),
      ),
    ).toEqual(["The Access option applies the new value."]);
  });

  it("a deletion policy restarts nothing", () => {
    expect(
      clusterImpacts(input(set([f("spec"), f("deletionPolicy")], "Orphan"))),
    ).toEqual([
      "Nothing restarts. The policy applies when the cluster is deleted.",
    ]);
  });

  it("suspend takes every workload to zero", () => {
    expect(clusterImpacts(input(set([f("spec"), f("suspend")], true)))).toEqual(
      ["Every workload goes to zero. The volumes stay."],
    );
  });

  it("a backup schedule restarts nothing", () => {
    expect(
      clusterImpacts(
        input(
          set(
            [f("spec"), f("attachments"), f("backupSchedule"), f("schedule")],
            "0 2 * * *",
          ),
        ),
      ),
    ).toEqual(["Nothing restarts."]);
  });

  it("lists the lines in the order of the table", () => {
    expect(
      clusterImpacts(
        input(
          both(
            set([f("spec"), f("suspend")], true),
            set(cluster("gateway", "replicas"), 3),
            set([f("spec"), f("releaseRef")], "camunda-8-10-1"),
          ),
        ),
      ),
    ).toEqual([
      "The brokers restart one at a time. The other processes restart.",
      "Gateway pods restart.",
      "Every workload goes to zero. The volumes stay.",
    ]);
  });
});

describe("mergedSpecs", () => {
  it("mergedSpecs applies a reset, so the preset value shows after", () => {
    const { before, after } = mergedSpecs(
      input(reset(cluster("gateway", "replicas")), {
        claim: claim({ gateway: { replicas: 2 } }),
      }),
    );
    expect(before.spec.gateway?.replicas).toBe(2);
    expect(after.spec.gateway?.replicas).toBe(1);
  });

  it("uses the preset of the claim after the change", () => {
    const { before, after } = mergedSpecs(
      input(set([f("spec"), f("presetRef")], "large")),
    );
    expect(before.spec.tasklist?.replicas).toBe(1);
    expect(after.spec.tasklist?.replicas).toBe(2);
  });
});

describe("showsMergedDiff", () => {
  it("is true for a pending preset, release, or reset, else false", () => {
    const of = (change: (s: Session<Claim>) => Session<Claim>) =>
      showsMergedDiff(input(change).session);
    expect(of(set([f("spec"), f("presetRef")], "large"))).toBe(true);
    expect(of(set([f("spec"), f("releaseRef")], "camunda-8-10-1"))).toBe(true);
    expect(of(reset(cluster("zeebe", "replicas")))).toBe(true);
    expect(of(set(cluster("zeebe", "replicas"), 5))).toBe(false);
  });
});
