import { describe, expect, it } from "vitest";
import type { CamundaClusterSpec } from "@/generated/crds/camundacluster";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import type { ResolvedPolicy } from "@/lib/policy";
import { clusterFields } from "@/lib/fields";
import { LOCK_FIELDS } from "@/lib/fields/locks";
import { SLOT_FIELDS } from "@/lib/fields/slots";
import { clusterLocks } from "./locks";

const SELECTION =
  "spec.plugins[section=volume-encryption,registration=aws-ebs-kms]";

function locks() {
  const spec = {
    version: "8.9.4",
    serviceAccount: { name: "orders" },
    zeebe: { replicas: 3, storageClassName: "encrypted" },
  };
  const cluster = {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaCluster",
    metadata: {
      name: "orders",
      namespace: "team-a",
      labels: { [CLAIM_LABEL]: "orders" },
    },
    spec,
  } as unknown as Resource<"CamundaCluster">;
  const claim = {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: { name: "orders", namespace: "team-a" },
    spec: {
      presetRef: "medium",
      plugins: [
        {
          section: "volume-encryption",
          registration: "aws-ebs-kms",
          parameters: { keyAlias: "orders" },
        },
      ],
    },
  } as unknown as Resource<"CamundaClusterClaim">;
  const section = {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name: "volume-encryption" },
    spec: {
      label: "Volume encryption",
      slot: "volumeEncryption",
      appliesTo: ["CamundaClusterClaim"],
      selection: "One",
    },
  } as unknown as Resource<"PluginSection">;
  const policy = {
    parameters: {
      "aws-ebs-kms": {
        keyArn: { value: "arn:aws:kms:eu-west-1:1:key/a" },
        keyAlias: { claimEditable: true, value: "default" },
      },
    },
  } as unknown as ResolvedPolicy;
  return clusterLocks(
    clusterFields({
      cluster,
      merged: mergeCluster(undefined, undefined, spec as CamundaClusterSpec),
      claim,
      policy,
      sections: [section],
      registrations: [],
    }),
  );
}

describe("clusterLocks", () => {
  it("locks spec.cluster.version by the release", () => {
    expect(locks()("spec.cluster.version")).toEqual({ reason: "release" });
  });

  it("locks spec.cluster.serviceAccount.name by the claim controller", () => {
    expect(locks()("spec.cluster.serviceAccount.name")).toEqual({
      reason: "claimController",
    });
  });

  it("locks spec.cluster.zeebe.storageClassName as filled by the section", () => {
    expect(locks()("spec.cluster.zeebe.storageClassName")).toEqual({
      reason: "filled",
      section: "volume-encryption",
      sectionLabel: "Volume encryption",
    });
  });

  it("locks an admin parameter at its claim path", () => {
    expect(locks()(`${SELECTION}.parameters.keyArn`)).toEqual({
      reason: "admin",
    });
  });

  it("does not lock a claim-editable parameter", () => {
    expect(locks()(`${SELECTION}.parameters.keyAlias`)).toBeUndefined();
  });

  it("does not lock the selection of a filled section", () => {
    expect(locks()(SELECTION)).toBeUndefined();
  });

  it("does not lock spec.cluster.zeebe.replicas", () => {
    expect(locks()("spec.cluster.zeebe.replicas")).toBeUndefined();
  });

  // clusterLocks builds a claim path from docNames, which hold no list
  // items. That is exact only while each locked or filled field is a plain
  // field.
  it("every LOCK_FIELDS row and every cluster SLOT_FIELDS row names plain fields", () => {
    const names = [
      ...LOCK_FIELDS.map((row) => row.names),
      ...SLOT_FIELDS.filter(
        (row) => row.claimKind === "CamundaClusterClaim",
      ).map((row) => row.names),
    ];
    for (const row of names)
      for (const name of row) expect(name).toMatch(/^[A-Za-z][A-Za-z0-9]*$/);
  });
});
