import { describe, expect, it } from "vitest";
import { parse } from "yaml";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { claimConditions, claimView, storedYaml } from "./claim";

const now = "2026-10-30T12:00:00Z";

type ClaimSpec = Resource<"CamundaClusterClaim">["spec"];
type ClaimStatus = NonNullable<Resource<"CamundaClusterClaim">["status"]>;

function claimOf(
  spec: Partial<ClaimSpec> = {},
  status: Partial<ClaimStatus> = {},
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-a",
      name: "orders",
      managedFields: [{ manager: "fleet-manager", operation: "Apply" }],
    },
    spec: { presetRef: "medium", ...spec },
    status,
  } as Resource<"CamundaClusterClaim">;
}

function clusterOf(spec: object = {}): Resource<"CamundaCluster"> {
  return {
    kind: "CamundaCluster",
    metadata: {
      namespace: "team-a",
      name: "orders",
      labels: { [CLAIM_LABEL]: "orders" },
    },
    spec: { presetRef: "medium", ...spec },
  } as unknown as Resource<"CamundaCluster">;
}

const environment = {
  kind: "Environment",
  metadata: { name: "prod" },
  spec: { displayName: "Production" },
} as unknown as Resource<"Environment">;

const section = (name: string, label: string, order?: number) =>
  ({
    kind: "PluginSection",
    metadata: { name },
    spec: { label, order, appliesTo: ["CamundaClusterClaim"] },
  }) as unknown as Resource<"PluginSection">;

const registration = (name: string, label: string) =>
  ({
    kind: "PluginRegistration",
    metadata: { name },
    spec: { label, deletionPolicies: ["Delete"] },
  }) as unknown as Resource<"PluginRegistration">;

const condition = (type: string, status = "True", reason = "Healthy") => ({
  type,
  status: status as "True" | "False" | "Unknown",
  reason,
  message: "",
  lastTransitionTime: now,
});

function view(
  claim: Resource<"CamundaClusterClaim">,
  over: {
    cluster?: Resource<"CamundaCluster">;
    sections?: Resource<"PluginSection">[];
    registrations?: Resource<"PluginRegistration">[];
  } = {},
) {
  const cluster = over.cluster ?? clusterOf();
  return claimView({
    claim,
    cluster,
    merged: mergeCluster(undefined, undefined, cluster.spec),
    environments: [environment],
    sections: over.sections ?? [],
    registrations: over.registrations ?? [],
  });
}

describe("claimView", () => {
  it("shows the environment of the claim by its display name", () => {
    expect(view(claimOf({}, { environment: "prod" })).environment).toBe(
      "Production",
    );
  });

  it("shows the environment by its name when no Environment has it", () => {
    expect(view(claimOf({}, { environment: "staging" })).environment).toBe(
      "staging",
    );
  });

  it("has no environment when the status names none", () => {
    expect(view(claimOf()).environment).toBeUndefined();
  });

  it("marks the release and the platform config that come from the environment", () => {
    const v = view(claimOf(), {
      cluster: clusterOf({ releaseRef: "r-8.9", platformConfigRef: "eks" }),
    });
    expect(v.release).toEqual({ value: "r-8.9", fromEnvironment: true });
    expect(v.platformConfig).toEqual({ value: "eks", fromEnvironment: true });
  });

  it("marks the refs that the claim sets as its own", () => {
    const v = view(claimOf({ releaseRef: "r-8.9", platformConfigRef: "eks" }), {
      cluster: clusterOf({ releaseRef: "r-8.9", platformConfigRef: "eks" }),
    });
    expect(v.release).toEqual({ value: "r-8.9", fromEnvironment: false });
    expect(v.platformConfig).toEqual({ value: "eks", fromEnvironment: false });
    expect(v.preset).toBe("medium");
  });

  it("takes the deletion policy of the resolved policy when the claim sets none", () => {
    expect(
      view(claimOf({}, { resolvedPolicy: { deletionPolicy: "Orphan" } }))
        .deletionPolicy,
    ).toEqual({ value: "Orphan", fromEnvironment: true });
    expect(view(claimOf({ deletionPolicy: "Delete" })).deletionPolicy).toEqual({
      value: "Delete",
      fromEnvironment: false,
    });
  });

  it("orders the sections by their order and gives each its parameters", () => {
    const v = view(
      claimOf(
        {
          plugins: [
            { section: "access", registration: "gateway-access" },
            { section: "backup-storage", registration: "rustfs" },
          ],
        },
        {
          resolvedPolicy: {
            parameters: {
              "gateway-access": { hostname: { value: "orders.example.com" } },
            },
          },
        },
      ),
      {
        sections: [
          section("access", "Access", 50),
          section("backup-storage", "Backup storage", 30),
        ],
        registrations: [
          registration("gateway-access", "Gateway"),
          registration("rustfs", "RustFS"),
        ],
      },
    );
    expect(v.sections.map((s) => s.sectionLabel)).toEqual([
      "Backup storage",
      "Access",
    ]);
    expect(v.sections[1]).toMatchObject({
      section: "access",
      registration: "gateway-access",
      optionLabel: "Gateway",
    });
    expect(v.sections[1].parameters).toHaveLength(1);
    expect(v.sections[1].parameters[0]).toMatchObject({
      label: "hostname",
      value: "orders.example.com",
      state: { kind: "admin" },
      lock: { reason: "admin" },
    });
    expect(v.sections[0].parameters).toEqual([]);
  });

  it("falls back to the names of a missing section and registration", () => {
    const v = view(
      claimOf({ plugins: [{ section: "dns", registration: "route53" }] }),
    );
    expect(v.sections[0]).toMatchObject({
      sectionLabel: "dns",
      optionLabel: "route53",
    });
  });

  it("reads the attachments of the claim", () => {
    const v = view(
      claimOf({
        attachments: {
          optimize: { managementAuthConfigRef: "auth" },
          backupSchedule: {
            schedule: "0 2 * * *",
            retained: { completed: 7 },
          },
        },
      } as Partial<ClaimSpec>),
    );
    expect(v.optimize).toEqual({ managementAuthConfigRef: "auth" });
    expect(v.backupSchedule).toEqual({ schedule: "0 2 * * *", completed: 7 });
  });

  it("has no attachments when the claim sets none", () => {
    const v = view(claimOf());
    expect(v.optimize).toBeUndefined();
    expect(v.backupSchedule).toBeUndefined();
  });

  it("labels each request with its section and option", () => {
    const v = view(
      claimOf(
        {},
        {
          requests: [
            {
              section: "access",
              registration: "gateway-access",
              name: "orders-access",
              outputsReady: "True",
              ready: "False",
            },
          ],
        },
      ),
      {
        sections: [section("access", "Access")],
        registrations: [registration("gateway-access", "Gateway")],
      },
    );
    expect(v.requests).toEqual([
      {
        name: "orders-access",
        sectionLabel: "Access",
        optionLabel: "Gateway",
        outputsReady: "True",
        ready: "False",
      },
    ]);
  });
});

describe("claimConditions", () => {
  it("puts Ready first and keeps the claim conditions in the generated order", () => {
    const claim = claimOf(
      {},
      {
        conditions: [
          condition("ClusterReady"),
          condition("AttachmentsReady"),
          condition("Ready"),
          condition("RequestsReady"),
        ],
      },
    );
    expect(claimConditions(claim).map((c) => c.type)).toEqual([
      "Ready",
      "AttachmentsReady",
      "ClusterReady",
      "RequestsReady",
    ]);
  });

  it("keeps a condition type that the map does not place, last", () => {
    const claim = claimOf(
      {},
      { conditions: [condition("FutureReady"), condition("Ready")] },
    );
    expect(claimConditions(claim).map((c) => c.type)).toEqual([
      "Ready",
      "FutureReady",
    ]);
  });

  it("gives no conditions to a claim without a status", () => {
    const claim = claimOf();
    delete claim.status;
    expect(claimConditions(claim)).toEqual([]);
  });
});

describe("storedYaml", () => {
  it("drops managedFields from the stored YAML and keeps the rest", () => {
    const claim = claimOf(
      { releaseRef: "r-8.9" },
      { conditions: [condition("Ready")] },
    );
    const stored = parse(storedYaml(claim));
    expect(stored.metadata.managedFields).toBeUndefined();
    expect(stored.metadata.name).toBe("orders");
    expect(stored.spec).toEqual(claim.spec);
    expect(stored.status).toEqual(claim.status);
    expect(claim.metadata.managedFields).toHaveLength(1);
  });
});
