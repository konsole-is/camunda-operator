import { describe, expect, it } from "vitest";
import type { Step } from "@/lib/fields";
import { CLAIM_LABEL } from "@/lib/labels";
import { keptFields } from "./kept";
import { openSession, resetValue, setValue, type ClaimObject } from "./session";

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
      { path: "spec.management.identityProvider.oidc", manager: "kubectl" },
      {
        path: "spec.management.identity.admin.claimName",
        manager: "kubectl",
      },
      {
        path: "spec.management.identity.admin.claimValue",
        manager: "kubectl",
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

  it("names nothing for a change that removes no field", () => {
    let s = session(claimOf([kubectl]));
    const set = setValue(s, at("spec", "deletionPolicy"), "Orphan");
    if ("session" in set) s = set.session;
    expect(keptFields(s)).toEqual([]);
  });
});
