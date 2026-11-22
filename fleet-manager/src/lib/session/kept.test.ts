import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import {
  CLAIM_LABEL,
  LAST_APPLIED_ANNOTATION,
  REVISION_ANNOTATION,
} from "@/lib/labels";
import type { ForeignManager, ObjectOwnership } from "@/lib/write/types";
import {
  appliedSession,
  dryRunKept,
  keptFields,
  savedOf,
  saveManager,
  savedSession,
} from "./kept";
import {
  claimAfter,
  openSession,
  resetValue,
  setValue,
  type ClaimObject,
} from "./session";
import { valueAt } from "./steps";

const at = (...names: string[]): Step[] => names.map((field) => ({ field }));

function claimOf(managedFields: unknown[]): ClaimObject {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: {
      name: "mgmt",
      namespace: "mgmt-ns",
      resourceVersion: "1",
      managedFields,
    } as ClaimObject["metadata"],
    spec: {
      deletionPolicy: "Delete",
      environments: ["platform", "qa"],
      management: {
        identityProvider: { oidc: {} },
        identity: { admin: { claimName: "sub", claimValue: "42" } },
      },
    },
  };
}

const kubectl = {
  manager: "kubectl",
  operation: "Apply",
  fieldsV1: {
    "f:spec": {
      "f:environments": { 'v:"platform"': {}, 'v:"qa"': {} },
      "f:management": {
        "f:identityProvider": { "f:oidc": {} },
        "f:identity": {
          "f:admin": { "f:claimName": {}, "f:claimValue": {} },
        },
      },
    },
  },
};
const app = {
  manager: "fleet-manager",
  operation: "Apply",
  fieldsV1: { "f:spec": { "f:deletionPolicy": {} } },
};

function session(claim: ClaimObject) {
  return openSession({
    claim,
    resource: { metadata: { labels: { [CLAIM_LABEL]: "mgmt" } } },
    locks: () => undefined,
  })!;
}

describe("keptFields", () => {
  it("names each field that the change leaves out while another manager owns it", () => {
    let s = session(claimOf([kubectl, app]));
    const set = setValue(s, at("spec", "management", "identityProvider"), {
      keycloak: {},
    });
    if ("session" in set) s = set.session;
    for (const name of ["claimName", "claimValue"]) {
      const reset = resetValue(
        s,
        at("spec", "management", "identity", "admin", name),
      );
      if ("session" in reset) s = reset.session;
    }
    expect(keptFields(s)).toEqual([
      { path: "spec.management.identityProvider.oidc", managers: ["kubectl"] },
      {
        path: "spec.management.identity.admin.claimName",
        managers: ["kubectl"],
      },
      {
        path: "spec.management.identity.admin.claimValue",
        managers: ["kubectl"],
      },
    ]);
  });

  it("names no field that the fleet manager owns or that no manager owns", () => {
    let s = session(claimOf([app]));
    const reset = resetValue(s, at("spec", "deletionPolicy"));
    if ("session" in reset) s = reset.session;
    const cleared = resetValue(
      s,
      at("spec", "management", "identity", "admin", "claimName"),
    );
    if ("session" in cleared) s = cleared.session;
    expect(keptFields(s)).toEqual([]);
  });

  it("names a field that kubectl still owns after a save of the app took every field", () => {
    const shared = {
      ...app,
      fieldsV1: {
        "f:spec": { "f:deletionPolicy": {}, ...kubectl.fieldsV1["f:spec"] },
      },
    };
    let s = session(claimOf([kubectl, shared]));
    const set = setValue(s, at("spec", "environments"), ["platform"]);
    if ("session" in set) s = set.session;
    expect(keptFields(s)).toEqual([
      { path: 'spec.environments[="qa"]', managers: ["kubectl"] },
    ]);
    expect(claimAfter(appliedSession(s)).spec).toEqual(s.claim.spec);
  });

  it("gives the claim after the change when no field is kept", () => {
    let s = session(claimOf([kubectl]));
    const set = setValue(s, at("spec", "deletionPolicy"), "Orphan");
    if ("session" in set) s = set.session;
    expect(claimAfter(appliedSession(s)).spec).toEqual({
      ...s.claim.spec,
      deletionPolicy: "Orphan",
    });
  });

  it("names nothing for a change that removes no field", () => {
    let s = session(claimOf([kubectl]));
    const set = setValue(s, at("spec", "deletionPolicy"), "Orphan");
    if ("session" in set) s = set.session;
    expect(keptFields(s)).toEqual([]);
  });
});

describe("saveManager", () => {
  const gitOps = {
    manager: "argocd-controller",
    operation: "Apply",
    fieldsV1: {
      "f:metadata": { "f:annotations": { [`f:${REVISION_ANNOTATION}`]: {} } },
      "f:spec": { "f:deletionPolicy": {} },
    },
  };

  it("is the fleet manager in direct mode", () => {
    expect(saveManager(claimOf([gitOps]), "direct")).toBe("fleet-manager");
  });

  it("is the GitOps tool that applied the revision in Git mode", () => {
    expect(saveManager(claimOf([gitOps]), "git")).toBe("argocd-controller");
  });

  it("is undefined in Git mode while no GitOps tool applied a revision", () => {
    expect(saveManager(claimOf([kubectl, app]), "git")).toBeUndefined();
  });

  it("is undefined in Git mode for a claim that the GitOps tool applies client side", () => {
    const claim = claimOf([]);
    (claim.metadata as { annotations?: object }).annotations = {
      [LAST_APPLIED_ANNOTATION]: "{}",
    };
    expect(saveManager(claim, "git")).toBeUndefined();
  });

  it("lets the GitOps tool remove a field that only it owns", () => {
    const owned = {
      ...gitOps,
      fieldsV1: {
        ...gitOps.fieldsV1,
        "f:spec": { "f:environments": { 'v:"qa"': {} } },
      },
    };
    let s = session(claimOf([owned]));
    const set = setValue(s, at("spec", "environments"), ["platform"]);
    if ("session" in set) s = set.session;
    expect(keptFields(s)).toEqual([
      { path: 'spec.environments[="qa"]', managers: ["argocd-controller"] },
    ]);
    expect(keptFields(s, "argocd-controller")).toEqual([]);
    expect(claimAfter(appliedSession(s, "argocd-controller")).spec).toEqual(
      claimAfter(s).spec,
    );
  });
});

describe("savedOf", () => {
  const changedPolicy = (managedFields: unknown[]) => {
    let s = session(claimOf(managedFields));
    const reset = resetValue(s, at("spec", "deletionPolicy"));
    if ("session" in reset) s = reset.session;
    return s;
  };

  it("keeps no field that only the fleet manager set in direct mode", () => {
    const saved = savedOf(changedPolicy([app]), "direct");
    expect(saved.kept).toEqual([]);
    expect(saved.firstSync).toBe(false);
  });

  it("names a field that the fleet manager set before the first GitOps sync", () => {
    const saved = savedOf(changedPolicy([app]), "git");
    expect(saved.kept).toEqual([
      { path: "spec.deletionPolicy", managers: ["fleet-manager"] },
    ]);
    expect(saved.firstSync).toBe(true);
    // A client-side sync applies the change as written.
    expect(claimAfter(saved.applied).spec).not.toHaveProperty("deletionPolicy");
  });
});

describe("savedSession", () => {
  const operator = {
    manager:
      "CamundaManagementClusterClaim/plane/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11",
    operation: "Apply",
    fieldsV1: { "f:spec": { "f:deletionPolicy": {} } },
  };
  const manager = (entry: {
    manager: string;
    operation: string;
  }): ForeignManager => ({
    name: entry.manager,
    operation: entry.operation,
    kind: entry.manager === operator.manager ? "operator" : "other",
    known: true,
  });
  const ref = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    namespace: "mgmt-ns",
    name: "mgmt",
  };
  // cleared drops qa from the environments and the deletion policy, as the
  // direct writer names them for the claims of claimOf.
  const cleared = (): ObjectOwnership => ({
    ref,
    changed: [],
    cleared: [
      {
        path: "spec.deletionPolicy",
        steps: at("spec", "deletionPolicy"),
        managers: [manager(operator)],
      },
      {
        path: 'spec.environments[="qa"]',
        steps: [...at("spec", "environments"), { value: "qa" }],
        managers: [manager(kubectl)],
      },
    ],
  });
  const dropped = () => {
    let s = session(claimOf([kubectl, operator]));
    const set = setValue(s, at("spec", "environments"), ["platform"]);
    if ("session" in set) s = set.session;
    const reset = resetValue(s, at("spec", "deletionPolicy"));
    if ("session" in reset) s = reset.session;
    return s;
  };

  it("keeps each cleared field of another tool without the take-over", () => {
    const s = dropped();
    expect(claimAfter(savedSession(s, cleared(), false)).spec).toEqual(
      s.claim.spec,
    );
  });

  it("removes each offered cleared field with the take-over, and keeps a field of an operator controller", () => {
    const s = dropped();
    expect(claimAfter(savedSession(s, cleared(), true)).spec).toEqual({
      ...s.claim.spec,
      environments: ["platform"],
    });
  });

  it("is the session itself when the review names no cleared field", () => {
    const s = dropped();
    expect(savedSession(s, undefined, false)).toBe(s);
    expect(savedSession(s, { ...cleared(), cleared: [] }, true)).toBe(s);
  });
});

describe("dryRunKept", () => {
  // clusterClaim returns a cluster claim with spec.cluster set to cluster.
  const clusterClaim = (cluster: object): ClaimObject => ({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "mgmt", namespace: "team-a", resourceVersion: "1" },
    spec: { cluster },
  });
  const zeebe = at("spec", "cluster", "zeebe");
  const resources = at("spec", "cluster", "zeebe", "resources");
  const change = (
    cluster: object,
    edit: (s: ReturnType<typeof session>) => unknown,
  ) => {
    const result = edit(session(clusterClaim(cluster))) as
      { session: ReturnType<typeof session> } | object;
    if (!("session" in result)) throw new Error("refused");
    return result.session;
  };

  it("keeps a field that a write of the whole block leaves out and the dry run holds", () => {
    const s = change(
      { zeebe: { resources: { requests: { cpu: "2" } } } },
      (x) => setValue(x, resources, { limits: { cpu: "1" } }),
    );
    const { kept, applied } = dryRunKept(
      s,
      clusterClaim({
        zeebe: { resources: { requests: { cpu: "2" }, limits: { cpu: "1" } } },
      }),
    );
    expect(kept).toEqual(["spec.cluster.zeebe.resources.requests.cpu"]);
    expect(valueAt(claimAfter(applied), resources)).toEqual({
      limits: { cpu: "1" },
      requests: { cpu: "2" },
    });
  });

  it("names only the fields of a reset block that the dry run holds", () => {
    const s = change(
      {
        zeebe: {
          resources: { limits: { cpu: "1" }, requests: { memory: "1Gi" } },
        },
      },
      (x) => resetValue(x, resources),
    );
    const { kept, applied } = dryRunKept(
      s,
      clusterClaim({ zeebe: { resources: { requests: { memory: "1Gi" } } } }),
    );
    expect(kept).toEqual(["spec.cluster.zeebe.resources.requests.memory"]);
    expect(valueAt(claimAfter(applied), resources)).toEqual({
      requests: { memory: "1Gi" },
    });
  });

  it("names a field of a list item by the keys of the item, and a list item without keys by its index", () => {
    const from = [
      { configMapRef: { name: "a" } },
      { configMapRef: { name: "b" } },
    ];
    const s = change(
      {
        zeebe: { replicas: 3 },
        extraEnv: [{ name: "TZ", value: "UTC" }],
        extraEnvFrom: from,
      },
      (x) =>
        setValue(x, at("spec", "cluster"), {
          zeebe: { replicas: 3 },
          extraEnv: [{ name: "TZ" }],
          extraEnvFrom: [from[0]],
        }),
    );
    const { kept } = dryRunKept(
      s,
      clusterClaim({
        zeebe: { replicas: 3 },
        extraEnv: [{ name: "TZ", value: "UTC" }],
        extraEnvFrom: from,
      }),
    );
    expect(kept).toEqual([
      "spec.cluster.extraEnv[name=TZ].value",
      "spec.cluster.extraEnvFrom[1]",
    ]);
  });

  it("puts back a default that the dry run gives a removed field, and names no other tool for it", () => {
    const retained = at("spec", "attachments", "backupSchedule", "retained");
    const live: ClaimObject = {
      ...clusterClaim({ zeebe: { replicas: 3 } }),
      spec: {
        cluster: { zeebe: { replicas: 3 } },
        attachments: {
          backupSchedule: { retained: { completed: 10, failed: 3 } },
        },
      },
    };
    const reset = resetValue(session(live), [
      ...retained,
      { field: "completed" },
    ]);
    if (!("session" in reset)) throw new Error("refused");
    const { kept, applied } = dryRunKept(reset.session, {
      ...live,
      spec: {
        ...live.spec,
        attachments: {
          backupSchedule: { retained: { completed: 7, failed: 3 } },
        },
      },
    });
    expect(kept).toEqual([]);
    expect(valueAt(claimAfter(applied), retained)).toEqual({
      completed: 7,
      failed: 3,
    });
  });

  it("keeps nothing when the dry run removes what the change removes, and gives s", () => {
    const s = change({ zeebe: { replicas: 3, partitions: 3 } }, (x) =>
      setValue(x, zeebe, { replicas: 3 }),
    );
    const result = dryRunKept(s, clusterClaim({ zeebe: { replicas: 3 } }));
    expect(result).toEqual({ kept: [], applied: s });
  });
});
