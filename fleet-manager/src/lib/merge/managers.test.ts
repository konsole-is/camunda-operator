import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  APP_FIELD_MANAGER,
  CLUSTER_CLAIM_MANAGER_PREFIX,
  PLANE_CLAIM_MANAGER_PREFIX,
  fieldManagers,
  isClaimManager,
  keptRemovals,
  lastChanged,
  wholeFieldManagers,
} from "./managers";

describe("isClaimManager", () => {
  it.each([
    ["CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11", true],
    [
      "CamundaManagementClusterClaim/plane/7d1e2c3b-0a4f-4e6d-8b2a-1c9f0e3d5a77",
      true,
    ],
    ["fleet-manager", false],
    ["kubectl-edit", false],
    ["camundaclusterclaim/cluster/0b5c", false],
  ])("%s is a claim manager: %s", (manager, want) => {
    expect(isClaimManager(manager)).toBe(want);
  });
});

describe("the manager constants", () => {
  // The Go constants are the source. A change on one side alone fails here.
  const goSource = readFileSync(
    new URL("../../../../api/fleet/v1alpha1/shared_types.go", import.meta.url),
    "utf8",
  );
  const goConstant = (name: string): string | undefined =>
    new RegExp(`${name}\\s*=\\s*"([^"]*)"`).exec(goSource)?.[1];

  it("match the constants of api/fleet/v1alpha1", () => {
    expect(CLUSTER_CLAIM_MANAGER_PREFIX).toBe(
      goConstant("FieldManagerPrefixClusterClaim"),
    );
    expect(PLANE_CLAIM_MANAGER_PREFIX).toBe(
      goConstant("FieldManagerPrefixPlaneClaim"),
    );
    expect(APP_FIELD_MANAGER).toBe(goConstant("FieldManagerApp"));
  });
});

const claim = "CamundaClusterClaim/cluster/0b5c7a2e";

describe("fieldManagers", () => {
  it("returns the manager of a nested field and of a list item by key", () => {
    const managers = fieldManagers({
      metadata: {
        managedFields: [
          {
            manager: claim,
            time: "2026-10-15T10:00:00Z",
            fieldsV1: {
              "f:spec": {
                "f:zeebe": { "f:replicas": {} },
                "f:extraEnv": {
                  'k:{"name":"TZ"}': { ".": {}, "f:name": {}, "f:value": {} },
                },
              },
            },
          },
          {
            manager: "volume-resizer",
            time: "2026-10-15T11:00:00Z",
            fieldsV1: { "f:spec": { "f:zeebe": { "f:storageSize": {} } } },
          },
        ],
      },
    });

    expect(managers).toEqual({
      "spec.zeebe.replicas": claim,
      "spec.extraEnv[name=TZ]": claim,
      "spec.extraEnv[name=TZ].name": claim,
      "spec.extraEnv[name=TZ].value": claim,
      "spec.zeebe.storageSize": "volume-resizer",
    });
  });

  it("reads a map key that has dots, and a set item and a list index", () => {
    const managers = fieldManagers({
      metadata: {
        managedFields: [
          {
            manager: APP_FIELD_MANAGER,
            fieldsV1: {
              "f:spec": {
                "f:podLabels": { "f:app.kubernetes.io/part-of": {} },
                "f:finalizers": { 'v:"a"': {} },
                "f:extraEnvFrom": { "i:0": {} },
              },
            },
          },
        ],
      },
    });

    expect(managers).toEqual({
      "spec.podLabels.app.kubernetes.io/part-of": APP_FIELD_MANAGER,
      'spec.finalizers[="a"]': APP_FIELD_MANAGER,
      "spec.extraEnvFrom[0]": APP_FIELD_MANAGER,
    });
  });

  it("gives a field that two managers own to the latest writer", () => {
    const fields = { "f:spec": { "f:replicas": {} } };

    const later = fieldManagers({
      metadata: {
        managedFields: [
          { manager: "b", time: "2026-10-15T12:00:00Z", fieldsV1: fields },
          { manager: "a", time: "2026-10-15T10:00:00Z", fieldsV1: fields },
        ],
      },
    });
    const tie = fieldManagers({
      metadata: {
        managedFields: [
          { manager: "a", fieldsV1: fields },
          { manager: "b", fieldsV1: fields },
        ],
      },
    });

    expect(later["spec.replicas"]).toBe("b");
    expect(tie["spec.replicas"]).toBe("b");
  });

  it("returns no entries for an object without managed fields", () => {
    expect(fieldManagers({})).toEqual({});
    expect(fieldManagers({ metadata: {} })).toEqual({});
  });
});

describe("wholeFieldManagers", () => {
  it("returns a node owned as a whole and not a node owned only by its presence", () => {
    const live = {
      metadata: {
        managedFields: [
          {
            manager: "env-injector",
            fieldsV1: { "f:spec": { "f:extraEnvFrom": {} } },
          },
          {
            manager: "label-injector",
            fieldsV1: {
              "f:spec": { "f:podLabels": { ".": {}, "f:injected": {} } },
            },
          },
        ],
      },
    };
    expect(wholeFieldManagers(live)).toEqual({
      "spec.extraEnvFrom": "env-injector",
      "spec.podLabels.injected": "label-injector",
    });
    expect(fieldManagers(live)["spec.podLabels"]).toBe("label-injector");
  });
});

describe("lastChanged", () => {
  it("returns the newest time of the managedFields entries", () => {
    expect(
      lastChanged({
        metadata: {
          managedFields: [
            { manager: "kubectl", time: "2026-11-01T08:00:00Z" },
            { manager: "fleet-manager", time: "2026-11-14T09:30:00Z" },
            { manager: "gitops", time: "2026-11-02T10:00:00Z" },
          ],
        },
      }),
    ).toBe("2026-11-14T09:30:00.000Z");
  });

  it("reads a Date time", () => {
    expect(
      lastChanged({
        metadata: {
          managedFields: [{ time: new Date("2026-11-14T09:30:00Z") }],
        },
      }),
    ).toBe("2026-11-14T09:30:00.000Z");
  });

  it("skips a time that does not parse", () => {
    expect(
      lastChanged({
        metadata: {
          managedFields: [
            { time: "not a time" },
            { time: "2026-11-02T10:00:00Z" },
          ],
        },
      }),
    ).toBe("2026-11-02T10:00:00.000Z");
  });

  it("is undefined without a timed entry", () => {
    expect(
      lastChanged({ metadata: { managedFields: [{ manager: "x" }] } }),
    ).toBeUndefined();
    expect(lastChanged({})).toBeUndefined();
  });
});

describe("keptRemovals", () => {
  // The claim and the managedFields entry of kubectl as plane-ready of the
  // page scenarios holds them, cut to the fields that the cases read.
  const spec = () => ({
    deletionPolicy: "Delete",
    environments: ["platform", "qa"],
    management: {
      identityProvider: { oidc: {} },
      identity: { admin: { claimName: "email", claimValue: "a@example.com" } },
    },
    plugins: [
      {
        registration: "existing-database-configs",
        section: "management-database",
        parameters: { identity: "identity-db" },
      },
    ],
  });
  const kubectl = {
    manager: "kubectl",
    operation: "Apply",
    time: "2026-01-01T00:00:00Z",
    fieldsV1: {
      "f:spec": {
        "f:environments": { 'v:"platform"': {}, 'v:"qa"': {} },
        "f:management": {
          "f:identity": {
            "f:admin": { "f:claimName": {}, "f:claimValue": {} },
          },
          "f:identityProvider": { "f:oidc": {} },
        },
        "f:plugins": {
          'k:{"registration":"existing-database-configs","section":"management-database"}':
            {
              ".": {},
              "f:parameters": { "f:identity": {} },
              "f:registration": {},
              "f:section": {},
            },
        },
      },
    },
  };
  // After a save of the deletion policy the app owns every field of the
  // claim, and kubectl still owns its own.
  const app = {
    manager: APP_FIELD_MANAGER,
    operation: "Apply",
    time: "2026-01-02T00:00:00Z",
    fieldsV1: {
      "f:spec": {
        "f:deletionPolicy": {},
        ...kubectl.fieldsV1["f:spec"],
      },
    },
  };
  const status = {
    manager: "manager",
    operation: "Update",
    subresource: "status",
    fieldsV1: { "f:status": { ".": {}, "f:environment": {} } },
  };
  const live = { metadata: { managedFields: [kubectl, app, status] } };

  it("names a field that another manager still owns after a save of the app", () => {
    const after = spec();
    after.management = {
      identityProvider: { keycloak: {} },
      identity: { admin: { username: "admin" } },
    } as never;
    const { kept, result } = keptRemovals(
      live,
      { spec: spec() },
      { spec: after },
    );
    expect(kept).toEqual([
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
    expect(result).toEqual({
      spec: {
        ...after,
        management: {
          identityProvider: { keycloak: {}, oidc: {} },
          identity: {
            admin: {
              username: "admin",
              claimName: "email",
              claimValue: "a@example.com",
            },
          },
        },
      },
    });
  });

  it("names an item of a set list, and keeps it in the result", () => {
    const after = { ...spec(), environments: ["platform"] };
    const { kept, result } = keptRemovals(
      live,
      { spec: spec() },
      { spec: after },
    );
    expect(kept).toEqual([
      { path: 'spec.environments[="qa"]', managers: ["kubectl"] },
    ]);
    expect(result).toEqual({ spec: spec() });
  });

  it("names an item of a map list as a whole", () => {
    const after = { ...spec(), plugins: [] };
    const { kept, result } = keptRemovals(
      live,
      { spec: spec() },
      { spec: after },
    );
    expect(kept).toEqual([
      {
        path: "spec.plugins[registration=existing-database-configs,section=management-database]",
        managers: ["kubectl"],
      },
    ]);
    expect(result).toEqual({ spec: spec() });
  });

  it("names a nested map key inside an item of a map list", () => {
    const after = spec();
    after.plugins[0].parameters = {} as never;
    const { kept } = keptRemovals(live, { spec: spec() }, { spec: after });
    expect(kept).toEqual([
      {
        path: "spec.plugins[registration=existing-database-configs,section=management-database].parameters.identity",
        managers: ["kubectl"],
      },
    ]);
  });

  it("names every manager of a field, in the order of the claim, and none that only the applier owns", () => {
    const edit = {
      manager: "kubectl-edit",
      operation: "Update",
      fieldsV1: {
        "f:spec": {
          "f:deletionPolicy": {},
          "f:environments": { 'v:"qa"': {} },
        },
      },
    };
    const after = { ...spec(), environments: ["platform"] } as Record<
      string,
      unknown
    >;
    delete after.deletionPolicy;
    const { kept } = keptRemovals(
      { metadata: { managedFields: [kubectl, app, edit] } },
      { spec: spec() },
      { spec: after },
    );
    expect(kept).toEqual([
      { path: "spec.deletionPolicy", managers: ["kubectl-edit"] },
      {
        path: 'spec.environments[="qa"]',
        managers: ["kubectl", "kubectl-edit"],
      },
    ]);
  });

  it("keeps of an object that another manager owns by its presence only the children that it owns", () => {
    const owned = {
      manager: "kubectl",
      operation: "Apply",
      fieldsV1: {
        "f:spec": {
          "f:clusterSelector": { ".": {}, "f:matchLabels": { "f:team": {} } },
        },
      },
    };
    const mine = {
      manager: APP_FIELD_MANAGER,
      operation: "Apply",
      fieldsV1: {
        "f:spec": {
          "f:clusterSelector": { "f:matchExpressions": {} },
        },
      },
    };
    const before = {
      spec: {
        clusterSelector: {
          matchLabels: { team: "a" },
          matchExpressions: [{ key: "tier", operator: "Exists" }],
        },
      },
    };
    const { kept, result } = keptRemovals(
      { metadata: { managedFields: [owned, mine] } },
      before,
      { spec: {} },
    );
    expect(kept).toEqual([
      { path: "spec.clusterSelector", managers: ["kubectl"] },
    ]);
    expect(result).toEqual({
      spec: { clusterSelector: { matchLabels: { team: "a" } } },
    });
  });

  it("keeps the keys of a list item that another manager owns by its presence", () => {
    const owned = {
      manager: "kubectl",
      operation: "Apply",
      fieldsV1: {
        "f:spec": {
          "f:plugins": { 'k:{"section":"s","registration":"r"}': { ".": {} } },
        },
      },
    };
    const before = {
      spec: { plugins: [{ section: "s", registration: "r", parameters: {} }] },
    };
    const { result } = keptRemovals(
      { metadata: { managedFields: [owned] } },
      before,
      { spec: { plugins: [] } },
    );
    expect(result).toEqual({
      spec: { plugins: [{ section: "s", registration: "r" }] },
    });
  });

  it("keeps a field of an Update entry that has the name of the applier, and skips a status entry", () => {
    const entries = [
      {
        manager: "argocd-controller",
        operation: "Update",
        fieldsV1: { "f:spec": { "f:x": {} } },
      },
      {
        manager: "argocd-controller",
        operation: "Apply",
        fieldsV1: { "f:spec": { "f:x": {} } },
      },
      {
        manager: "manager",
        operation: "Update",
        subresource: "status",
        fieldsV1: { "f:spec": { "f:y": {} } },
      },
    ];
    const { kept } = keptRemovals(
      { metadata: { managedFields: entries } },
      { spec: { x: 1, y: 2 } },
      { spec: {} },
      "argocd-controller",
    );
    expect(kept).toEqual([{ path: "spec.x", managers: ["argocd-controller"] }]);
  });

  it("names the manager of a kept child of a list item that another manager owns by its presence", () => {
    const entries = [
      {
        manager: "a",
        operation: "Apply",
        fieldsV1: {
          "f:spec": {
            "f:env": {
              'k:{"name":"n"}': { ".": {}, "f:name": {}, "f:host": {} },
            },
          },
        },
      },
      {
        manager: "b",
        operation: "Update",
        fieldsV1: {
          "f:spec": { "f:env": { 'k:{"name":"n"}': { "f:port": {} } } },
        },
      },
    ];
    const { kept, result } = keptRemovals(
      { metadata: { managedFields: entries } },
      { spec: { env: [{ name: "n", host: "h", port: 25, user: "u" }] } },
      { spec: { env: [] } },
    );
    expect(kept).toEqual([{ path: "spec.env[name=n]", managers: ["a", "b"] }]);
    expect(result).toEqual({
      spec: { env: [{ name: "n", host: "h", port: 25 }] },
    });
  });

  it("keeps a kept item of a list in its place", () => {
    const entries = [
      {
        manager: "b",
        operation: "Apply",
        fieldsV1: { "f:spec": { "f:environments": { 'v:"qa"': {} } } },
      },
    ];
    const { result } = keptRemovals(
      { metadata: { managedFields: entries } },
      { spec: { environments: ["qa", "prod"] } },
      { spec: { environments: ["prod"] } },
    );
    expect(result).toEqual({ spec: { environments: ["qa", "prod"] } });
  });

  it("keeps nothing for a change that removes nothing, or without managed fields", () => {
    const after = { ...spec(), deletionPolicy: "Orphan" };
    expect(keptRemovals(live, { spec: spec() }, { spec: after })).toEqual({
      kept: [],
      result: { spec: after },
    });
    const gone = { ...spec(), environments: [] };
    expect(keptRemovals({}, { spec: spec() }, { spec: gone })).toEqual({
      kept: [],
      result: { spec: gone },
    });
  });
});
