import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  APP_FIELD_MANAGER,
  CLUSTER_CLAIM_MANAGER_PREFIX,
  PLANE_CLAIM_MANAGER_PREFIX,
  fieldManagers,
  isClaimManager,
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
