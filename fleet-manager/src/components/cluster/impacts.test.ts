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
      cluster: {
        zeebe: { replicas: 3, storageSize: "32Gi" },
        gateway: { replicas: 2 },
        operate: { replicas: 1 },
        ...clusterSpec,
      },
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
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "encryption" },
        spec: {
          label: "Encryption",
          slot: "volumeEncryption",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
      {
        apiVersion: "plugins.camunda.io/v1alpha1",
        kind: "PluginSection",
        metadata: { name: "backup" },
        spec: {
          label: "Backup",
          slot: "backupStorage",
          appliesTo: ["CamundaClusterClaim"],
          selection: "One",
        },
      } as unknown as Resource<"PluginSection">,
    ],
  };
}

// encrypted is claim with a selection of the encryption section, whose
// slot fills zeebe.storageClassName.
function encrypted(clusterSpec: Record<string, unknown> = {}): Claim {
  const c = claim(clusterSpec);
  const spec = c.spec as { plugins: unknown[] };
  spec.plugins = [
    ...spec.plugins,
    { section: "encryption", registration: "kms" },
  ];
  return c;
}

// selecting returns claim with the selections of the named sections only.
function selecting(c: Claim, ...sections: string[]): Claim {
  (c.spec as { plugins: unknown[] }).plugins = sections.map((section) => ({
    section,
    registration: section,
  }));
  return c;
}

const RESTART_ALL =
  "The brokers restart one at a time. The other processes restart.";

const platform = (auth: Record<string, unknown>) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaPlatformConfig",
    metadata: { name: "default" },
    spec: { auth },
  }) as unknown as Resource<"CamundaPlatformConfig">;

const BASIC = platform({ method: "basic" });
const OIDC = platform({
  method: "oidc",
  oidc: {
    issuerUrl: "https://idp",
    clientId: "camunda",
    clientSecretRef: { name: "idp", key: "secret" },
    clientIdClaim: "azp",
  },
});

const storage = (spec: Record<string, unknown>) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "SecondaryStorageConfig",
    metadata: { name: "data", namespace: "team-a" },
    spec,
  }) as unknown as Resource<"SecondaryStorageConfig">;

const ELASTICSEARCH = storage({
  type: "elasticsearch",
  elasticsearch: { nodeCount: 3 },
});
const RDBMS = storage({ type: "rdbms", rdbms: { databaseConfigRef: "db" } });

const environment = (release: string) =>
  ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "Environment",
    metadata: { name: "prod" },
    spec: { releases: { default: release } },
  }) as unknown as Resource<"Environment">;

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

  it("a release with the same Camunda version restarts every process", () => {
    // The config hash of every process takes the name of the release.
    expect(
      clusterImpacts(
        input(set([f("spec"), f("releaseRef")], "camunda-8-9-5-images")),
      ),
    ).toEqual([RESTART_ALL]);
  });

  it("a release that names the live release gives no line", () => {
    const inherits = claim();
    delete (inherits.spec as { releaseRef?: string }).releaseRef;
    expect(
      clusterImpacts(
        input(set([f("spec"), f("releaseRef")], "camunda-8-9-5"), {
          claim: inherits,
        }),
      ),
    ).toEqual([]);
  });

  it("a new platform config restarts every process", () => {
    expect(
      clusterImpacts(
        input(set([f("spec"), f("platformConfigRef")], "b"), {
          live: live({ platformConfigRef: "a" }),
        }),
      ),
    ).toEqual([RESTART_ALL]);
  });

  it("a preset lists each merged field that changes", () => {
    expect(
      clusterImpacts(input(set([f("spec"), f("presetRef")], "large"))),
    ).toEqual([RESTART_ALL, "spec.tasklist.replicas: 1 to 2."]);
  });

  it("a preset field that the new preset drops shows as unset", () => {
    const i = input(set([f("spec"), f("presetRef")], "small"));
    i.presets = [...i.presets, preset("small", { gateway: { replicas: 1 } })];
    expect(clusterImpacts(i)).toEqual([
      RESTART_ALL,
      "spec.tasklist.replicas: 1 to unset.",
    ]);
  });

  it("a field of a standalone process restarts its pods", () => {
    expect(
      clusterImpacts(input(set(cluster("gateway", "replicas"), 3))),
    ).toEqual(["Gateway pods restart."]);
    expect(
      clusterImpacts(
        input(set(cluster("zeebe", "resources"), { limits: { cpu: "2" } })),
      ),
    ).toEqual(["Broker pods restart."]);
  });

  it("two fields of one process give one line", () => {
    expect(
      clusterImpacts(
        input(
          both(
            set(cluster("zeebe", "resources"), { limits: { cpu: "2" } }),
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

  it("the environment of an embedded process restarts its host", () => {
    expect(
      clusterImpacts(
        input(
          set([...cluster("tasklist", "extraEnv"), { item: { name: "TZ" } }], {
            name: "TZ",
            value: "UTC",
          }),
        ),
      ),
    ).toEqual(["Gateway pods restart."]);
    expect(
      clusterImpacts(
        input(
          set(cluster("tasklist", "extraEnvFrom"), [
            { secretRef: { name: "s" } },
          ]),
        ),
      ),
    ).toEqual(["Gateway pods restart."]);
  });

  it("pod annotations of the cluster restart every process", () => {
    expect(
      clusterImpacts(input(set(cluster("podAnnotations", "team"), "a"))),
    ).toEqual([
      "Every process restarts, except the processes that set the field themselves.",
    ]);
  });

  it("a reset of spec.cluster restarts the processes that it changes", () => {
    expect(
      clusterImpacts(
        input(reset([f("spec"), f("cluster")]), {
          claim: claim({ gateway: { replicas: 2 } }),
          live: live({ gateway: { replicas: 2 } }),
        }),
      ),
    ).toEqual([
      "Broker pods restart.",
      "Gateway pods restart.",
      "Applies when Operate runs standalone.",
    ]);
  });

  it("a reset to a smaller preset size cannot shrink the volumes", () => {
    const i = input(reset(cluster("zeebe", "storageSize")), {
      claim: claim({ zeebe: { replicas: 3, storageSize: "64Gi" } }),
      live: live({ zeebe: { replicas: 3, storageSize: "64Gi" } }),
    });
    i.presets = i.presets.map((p) =>
      p.metadata?.name === "medium"
        ? preset("medium", {
            zeebe: { storageSize: "32Gi" },
            tasklist: { replicas: 1 },
          })
        : p,
    );
    expect(clusterImpacts(i)).toEqual([
      "Volumes cannot shrink. The operator keeps the current size.",
    ]);
  });

  it("a reset of the release to the default of the environment restarts the brokers", () => {
    const i = input(reset([f("spec"), f("releaseRef")]), {
      claim: claim(),
      live: live({ releaseRef: "camunda-8-9-5" }),
    });
    i.environment = environment("camunda-8-10-1");
    expect(clusterImpacts(i)).toEqual([
      "The brokers restart one at a time. The other processes restart.",
    ]);
  });

  it("an app that becomes standalone creates its Deployment and starts from the preset", () => {
    expect(
      clusterImpacts(input(set(cluster("tasklist", "mode"), "Standalone"))),
    ).toEqual([
      "Creates Deployment orders-tasklist and Service orders-tasklist.",
      "Tasklist starts from the workload settings of the preset: replicas, CPU, and memory.",
      "Gateway pods restart.",
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
      "Gateway pods restart.",
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

  it("a gateway that changes its mode restarts the connectors, which call it", () => {
    const enabled = { connectors: { enabled: true } };
    const i = input(set(cluster("gateway", "mode"), "Embedded"), {
      claim: claim(enabled),
      live: live(enabled),
    });
    i.platform = BASIC;
    expect(clusterImpacts(i)).toEqual([
      "Connectors pods restart.",
      "Removes Deployment orders-gateway and Service orders-gateway. Gateway runs in the brokers, which restarts.",
    ]);
  });

  it("a process that becomes standalone in the change does not restart", () => {
    expect(
      clusterImpacts(
        input(
          both(
            set(cluster("tasklist", "mode"), "Standalone"),
            set(cluster("tasklist", "replicas"), 2),
          ),
        ),
      ),
    ).toEqual([
      "Creates Deployment orders-tasklist and Service orders-tasklist.",
      "Tasklist starts with the settings prepared in the claim (1 fields).",
      "Gateway pods restart.",
    ]);
  });

  it("enabled connectors create their Deployment and restart nothing else", () => {
    const i = input(set(cluster("connectors", "enabled"), true));
    i.platform = BASIC;
    expect(clusterImpacts(i)).toEqual([
      "Creates Deployment orders-connectors and Service orders-connectors.",
      "Connectors starts from the workload settings of the preset: replicas, CPU, and memory.",
    ]);
  });

  it("disabled connectors remove their Deployment", () => {
    const enabled = { connectors: { enabled: true, replicas: 2 } };
    const i = input(set(cluster("connectors", "enabled"), false), {
      claim: claim(enabled),
      live: live(enabled),
    });
    i.platform = BASIC;
    expect(clusterImpacts(i)).toEqual([
      "Removes Deployment orders-connectors and Service orders-connectors.",
    ]);
  });

  it("connectors under OIDC with a client id claim restart the other processes", () => {
    // The OIDC client gets the connectors role only while connectors run.
    const i = input(set(cluster("connectors", "enabled"), true));
    i.platform = OIDC;
    expect(clusterImpacts(i)).toEqual([
      "Broker pods restart.",
      "Gateway pods restart.",
      "Creates Deployment orders-connectors and Service orders-connectors.",
      "Connectors starts from the workload settings of the preset: replicas, CPU, and memory.",
    ]);
  });

  describe("settings that every process except the connectors renders", () => {
    // Operate runs standalone and connectors run, so the lists show which
    // processes take the setting.
    const running = {
      operate: { mode: "Standalone" },
      connectors: { enabled: true },
    };
    const impacts = (
      change: (s: Session<Claim>) => Session<Claim>,
      over: {
        claim?: Claim;
        live?: Record<string, unknown>;
        platform?: Resource<"CamundaPlatformConfig">;
        storage?: Resource<"SecondaryStorageConfig">;
      } = {},
    ) => {
      const i = input(change, {
        claim: over.claim ?? claim(running),
        live: live({ ...running, ...over.live }),
      });
      i.platform = over.platform;
      i.storage = over.storage;
      return clusterImpacts(i);
    };
    const EVERY_UNIFIED = [
      "Broker pods restart.",
      "Gateway pods restart.",
      "Operate pods restart.",
    ];

    it.each(["replicas", "partitions", "replicationFactor"])(
      "zeebe.%s restarts the brokers, the gateway, and the standalone apps",
      (field) => {
        expect(impacts(set(cluster("zeebe", field), 5))).toEqual(EVERY_UNIFIED);
      },
    );

    it("indexReplicas restarts them on Elasticsearch only", () => {
      const change = set(cluster("indexReplicas"), 2);
      expect(impacts(change)).toEqual(EVERY_UNIFIED);
      expect(impacts(change, { storage: ELASTICSEARCH })).toEqual(
        EVERY_UNIFIED,
      );
      expect(impacts(change, { storage: RDBMS })).toEqual([]);
    });

    it("indexReplicas restarts nothing when it names the count the operator renders", () => {
      // Three nodes give one replica without the field.
      expect(
        impacts(set(cluster("indexReplicas"), 1), { storage: ELASTICSEARCH }),
      ).toEqual([]);
    });

    it("the admin role restarts them under OIDC only", () => {
      const change = set(cluster("auth", "admin"), { users: ["ada"] });
      expect(impacts(change)).toEqual(EVERY_UNIFIED);
      expect(impacts(change, { platform: OIDC })).toEqual(EVERY_UNIFIED);
      expect(impacts(change, { platform: BASIC })).toEqual([]);
    });

    it("the OIDC client restarts the connectors too", () => {
      expect(
        impacts(set(cluster("auth", "clientId"), "orders"), {
          platform: OIDC,
        }),
      ).toEqual([...EVERY_UNIFIED, "Connectors pods restart."]);
      expect(
        impacts(set(cluster("auth", "clientId"), "orders"), {
          platform: BASIC,
        }),
      ).toEqual([]);
    });

    it("an audience equal to the client id restarts nothing", () => {
      expect(
        impacts(set(cluster("auth", "audience"), "camunda"), {
          platform: OIDC,
        }),
      ).toEqual([]);
    });

    it("a password rotation restarts the connectors only", () => {
      expect(
        impacts(set(cluster("auth", "basic", "passwordRotation"), "2"), {
          platform: BASIC,
        }),
      ).toEqual(["Connectors pods restart."]);
      expect(
        impacts(set(cluster("auth", "basic", "adminEmail"), "a@b.c"), {
          platform: BASIC,
        }),
      ).toEqual([]);
    });

    it("externalUrl restarts them under OIDC only", () => {
      const change = set(cluster("externalUrl"), "https://orders.example");
      const own = selecting(claim(running));
      expect(impacts(change, { claim: own, platform: OIDC })).toEqual(
        EVERY_UNIFIED,
      );
      expect(impacts(change, { claim: own, platform: BASIC })).toEqual([]);
      expect(
        impacts(set(cluster("externalUrl"), "https://orders.example/"), {
          claim: selecting(
            claim({ ...running, externalUrl: "https://orders.example" }),
          ),
          live: { externalUrl: "https://orders.example" },
          platform: OIDC,
        }),
      ).toEqual([]);
    });

    describe("backup", () => {
      const bucket = {
        claim: selecting(claim(running), "access", "backup"),
        live: { backupStorageRef: "bucket" },
      };
      const schedule = set(
        cluster("backup", "primaryStorage", "schedule"),
        "PT2H",
      );

      it("the schedule restarts the brokers and the gateway on a relational cluster with a bucket", () => {
        expect(impacts(schedule, { ...bucket, storage: RDBMS })).toEqual([
          "Broker pods restart.",
          "Gateway pods restart.",
        ]);
        expect(impacts(schedule, bucket)).toEqual([
          "Broker pods restart.",
          "Gateway pods restart.",
        ]);
      });

      it("the retention restarts the brokers only", () => {
        expect(
          impacts(
            set(
              cluster("backup", "primaryStorage", "retention", "window"),
              "P1D",
            ),
            { ...bucket, storage: RDBMS },
          ),
        ).toEqual(["Broker pods restart."]);
      });

      it("restarts nothing on Elasticsearch, without a bucket, or for the dump", () => {
        expect(
          impacts(schedule, { ...bucket, storage: ELASTICSEARCH }),
        ).toEqual([]);
        expect(impacts(schedule, { storage: RDBMS })).toEqual([]);
        expect(
          impacts(set(cluster("backup", "dump"), { resources: {} }), {
            ...bucket,
            storage: RDBMS,
          }),
        ).toEqual([]);
      });
    });
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

  it("resets of list items by index remove the items that they name", () => {
    const env = [
      { configMapRef: { name: "a" } },
      { configMapRef: { name: "b" } },
      { configMapRef: { name: "c" } },
    ];
    const { after } = mergedSpecs(
      input(
        both(
          reset([...cluster("extraEnvFrom"), { index: 0 }]),
          reset([...cluster("extraEnvFrom"), { index: 1 }]),
        ),
        {
          claim: claim({ extraEnvFrom: env }),
          live: live({ extraEnvFrom: env }),
        },
      ),
    );
    expect(after.spec.extraEnvFrom).toEqual([{ configMapRef: { name: "c" } }]);
  });

  it("a reset of a keyed list item removes the item", () => {
    const env = [{ name: "TZ", value: "UTC" }];
    const { after } = mergedSpecs(
      input(reset([...cluster("extraEnv"), { item: { name: "TZ" } }]), {
        claim: claim({ extraEnv: env }),
        live: live({ extraEnv: env }),
      }),
    );
    expect(after.spec.extraEnv ?? []).toEqual([]);
  });

  it("a reset of the last claim field of a process keeps the fields that the controller fills", () => {
    const { after } = mergedSpecs(
      input(reset(cluster("zeebe", "replicas")), {
        claim: encrypted(),
        live: live({
          zeebe: { replicas: 3, storageSize: "32Gi", storageClassName: "fast" },
        }),
      }),
    );
    expect(after.spec.zeebe?.storageClassName).toBe("fast");
    expect(after.spec.zeebe?.replicas).toBeUndefined();
  });

  it("an empty block replaces the block before it", () => {
    const scheduling = { tolerations: [{ key: "a", operator: "Exists" }] };
    const i = input(set(cluster("scheduling"), {}), {
      claim: claim({ scheduling }),
      live: live({ scheduling }),
    });
    expect(mergedSpecs(i).after.spec.scheduling).toEqual({});
    expect(clusterImpacts(i)).toEqual([
      "Every process restarts, except the processes that set the field themselves.",
    ]);
  });

  it("a new order of a keyed list is a change", () => {
    const a = { name: "A", value: "1" };
    const b = { name: "B", value: "$(A)" };
    const i = input(set(cluster("extraEnv"), [a, b]), {
      claim: claim({ extraEnv: [b, a] }),
      live: live({ extraEnv: [b, a] }),
    });
    expect(mergedSpecs(i).after.spec.extraEnv).toEqual([a, b]);
    expect(clusterImpacts(i)).toEqual([
      "Every process restarts, except the processes that set the field themselves.",
    ]);
  });

  it("a reset of suspend shows the cluster running after", () => {
    const suspended = claim();
    (suspended.spec as { suspend?: boolean }).suspend = true;
    const { before, after } = mergedSpecs(
      input(reset([f("spec"), f("suspend")]), {
        claim: suspended,
        live: live({ suspend: true }),
      }),
    );
    expect(before.spec.suspend).toBe(true);
    expect(after.spec.suspend).toBeUndefined();
  });

  it("an empty process block keeps the fields that the controller fills", () => {
    const { after } = mergedSpecs(
      input(set(cluster("zeebe"), {}), {
        claim: encrypted(),
        live: live({
          zeebe: { replicas: 3, storageSize: "32Gi", storageClassName: "fast" },
        }),
      }),
    );
    expect(after.spec.zeebe?.storageClassName).toBe("fast");
  });

  it("a reset of an empty block gives the block of the preset back", () => {
    const tolerations = [{ key: "a", operator: "Exists" }];
    const i = input(reset(cluster("scheduling")), {
      claim: claim({ scheduling: {} }),
      live: live({ scheduling: {} }),
    });
    i.presets = i.presets.map((p) =>
      p.metadata?.name === "medium"
        ? preset("medium", { scheduling: { tolerations } })
        : p,
    );
    expect(mergedSpecs(i).after.spec.scheduling).toEqual({ tolerations });
    expect(clusterImpacts(i)).toEqual([
      "Every process restarts, except the processes that set the field themselves.",
    ]);
  });

  it("a field that no selected slot fills follows the claim", () => {
    const fast = { replicas: 3, storageSize: "32Gi", storageClassName: "fast" };
    const { after } = mergedSpecs(
      input(set(cluster("zeebe", "storageClassName"), "slow"), {
        claim: claim({ zeebe: fast }),
        live: live({ zeebe: fast }),
      }),
    );
    expect(after.spec.zeebe?.storageClassName).toBe("slow");
  });

  it("a new platform config of the claim shows after", () => {
    const { after } = mergedSpecs(
      input(set([f("spec"), f("platformConfigRef")], "b"), {
        live: live({ platformConfigRef: "a" }),
      }),
    );
    expect(after.spec.platformConfigRef).toBe("b");
  });

  it("a reset of the platform config takes the default of the environment", () => {
    const withConfig = claim();
    (withConfig.spec as { platformConfigRef?: string }).platformConfigRef = "b";
    const i = input(reset([f("spec"), f("platformConfigRef")]), {
      claim: withConfig,
      live: live({ platformConfigRef: "b" }),
    });
    i.environment = environment("camunda-8-9-5");
    (i.environment.spec as { platformConfigRef?: string }).platformConfigRef =
      "default";
    expect(mergedSpecs(i).after.spec.platformConfigRef).toBe("default");
  });

  it("keeps the live release when the claim does not change it", () => {
    const inherits = claim({ gateway: { replicas: 2 } });
    delete (inherits.spec as { releaseRef?: string }).releaseRef;
    const { before, after } = mergedSpecs(
      input(reset(cluster("gateway", "replicas")), { claim: inherits }),
    );
    expect(before.spec.version).toBe("8.9.5");
    expect(after.spec.version).toBe("8.9.5");
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
    expect(
      of(
        set([f("spec")], {
          ...(claim().spec as object),
          presetRef: "large",
        }),
      ),
    ).toBe(true);
  });
});
