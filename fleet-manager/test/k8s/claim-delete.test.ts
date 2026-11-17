import { randomUUID } from "node:crypto";
import { KubeConfig } from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import {
  applyClaimDelete,
  previewClaimDelete,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import {
  applyPlaneClaimDelete,
  previewPlaneClaimDelete,
} from "@/app/fleet/planes/[namespace]/[name]/actions";
import type { Change, KubeObject } from "@/lib/write/types";
import { KubeError, kubeRequest } from "../scenarios/kube";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const namespace = `claim-delete-${randomUUID().slice(0, 8)}`;
const apiVersion = "fleet.camunda.io/v1alpha1";
const claimsPath = `/apis/fleet.camunda.io/v1alpha1/namespaces/${namespace}/camundaclusterclaims`;

beforeAll(async () => {
  await kubeRequest(kc, "POST", "/api/v1/namespaces", {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
  // The settings live in this namespace. It has none, so the writer is
  // direct.
  process.env.FLEET_NAMESPACE = namespace;
  delete process.env.FLEET_SETTINGS_GIT_REPOSITORY;
});

// made creates a claim and returns it as the API server answered.
async function made(
  name = `orders-${randomUUID().slice(0, 8)}`,
  spec: object = {},
  finalizers: string[] = [],
) {
  return kubeRequest<KubeObject>(kc, "POST", claimsPath, {
    apiVersion,
    kind: "CamundaClusterClaim",
    metadata: { name, namespace, finalizers },
    spec: { presetRef: "medium", ...spec },
  });
}

const requestsPath = `/apis/plugins.camunda.io/v1alpha1/namespaces/${namespace}/pluginrequests`;

// requested creates a PluginRequest that claim controls, with policy.
async function requested(claim: KubeObject, policy: "Delete" | "Orphan") {
  const { name, uid } = claim.metadata;
  return kubeRequest<KubeObject>(kc, "POST", requestsPath, {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: {
      name: `${name}-backup-storage`,
      namespace,
      ownerReferences: [
        {
          apiVersion,
          kind: "CamundaClusterClaim",
          name,
          uid,
          controller: true,
        },
      ],
    },
    spec: {
      section: "backup-storage",
      registration: "echo",
      claimRef: { kind: "CamundaClusterClaim", name },
      target: { kind: "CamundaCluster", name },
      deletionPolicy: policy,
    },
  });
}

async function read(name: string): Promise<KubeObject | undefined> {
  try {
    return await kubeRequest<KubeObject>(kc, "GET", `${claimsPath}/${name}`);
  } catch (error) {
    if (error instanceof KubeError && error.status === 404) return undefined;
    throw error;
  }
}

async function patch(name: string, body: object, status = false) {
  await kubeRequest(
    kc,
    "PATCH",
    `${claimsPath}/${name}${status ? "/status" : ""}`,
    body,
    "application/merge-patch+json",
  );
}

// reviewed is the delete of claim that the page sends after its review.
function reviewed(claim: KubeObject): Change {
  const { name } = claim.metadata;
  const key = `CamundaClusterClaim/${namespace}/${name}` as const;
  return {
    intent: `Delete the claim ${name}`,
    apply: [],
    delete: [{ apiVersion, kind: "CamundaClusterClaim", namespace, name }],
    uids: { [key]: claim.metadata.uid as string },
    generations: { [key]: claim.metadata.generation as number },
  };
}

describe("the delete of a claim against envtest", () => {
  it("deletes a claim without requests after a status write since the review", async () => {
    const claim = await made();
    await patch(claim.metadata.name, { status: { environment: "prod" } }, true);

    expect(await previewClaimDelete(reviewed(claim))).toMatchObject({
      errors: [],
    });
    expect(await applyClaimDelete(reviewed(claim))).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(await read(claim.metadata.name)).toBeUndefined();
  });

  it("refuses while a request of the claim has another deletion policy, and deletes the claim once the request has its policy", async () => {
    const claim = await made(undefined, { deletionPolicy: "Orphan" });
    const request = await requested(claim, "Delete");

    expect(await applyClaimDelete(reviewed(claim))).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Stale",
          message: `The request ${request.metadata.name} does not have the deletion policy of the claim. The operator has not updated it.`,
        }),
      ],
    });
    expect(await read(claim.metadata.name)).toBeDefined();

    await kubeRequest(
      kc,
      "PATCH",
      `${requestsPath}/${request.metadata.name}`,
      { spec: { deletionPolicy: "Orphan" } },
      "application/merge-patch+json",
    );
    expect(await applyClaimDelete(reviewed(claim))).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(await read(claim.metadata.name)).toBeUndefined();
  });

  it("deletes a set-up claim with the cleanup finalizer whose precheck fails and whose request still has the old policy", async () => {
    const claim = await made(undefined, { deletionPolicy: "Orphan" }, [
      "fleet.camunda.io/cleanup",
    ]);
    await patch(
      claim.metadata.name,
      {
        status: {
          observedGeneration: claim.metadata.generation,
          resolvedPolicy: { deletionPolicy: "Delete" },
          conditions: [
            {
              type: "Ready",
              status: "False",
              reason: "InvalidReference",
              message: "CamundaRelease r1 does not exist",
              lastTransitionTime: new Date()
                .toISOString()
                .replace(/\.\d+Z$/, "Z"),
            },
          ],
        },
      },
      true,
    );
    await requested(claim, "Delete");

    expect(await applyClaimDelete(reviewed(claim))).toEqual({
      ok: true,
      mode: "direct",
    });
    // The cleanup finalizer holds the claim until the operator removes it.
    expect(
      (await read(claim.metadata.name))?.metadata.deletionTimestamp,
    ).toBeDefined();
  });

  it("refuses a claim that lost its cleanup finalizer after the operator recorded its policy", async () => {
    const claim = await made();
    await patch(
      claim.metadata.name,
      { status: { resolvedPolicy: { deletionPolicy: "Delete" } } },
      true,
    );

    expect(await applyClaimDelete(reviewed(claim))).toMatchObject({
      ok: false,
      errors: [
        {
          code: "Invalid",
          message: expect.stringContaining("fleet.camunda.io/cleanup"),
        },
      ],
    });
    expect(await read(claim.metadata.name)).toBeDefined();
  });

  it("refuses with Stale and keeps a claim whose spec changed since the review", async () => {
    const claim = await made();
    await patch(claim.metadata.name, { spec: { presetRef: "large" } });

    expect(await previewClaimDelete(reviewed(claim))).toMatchObject({
      objects: [],
      errors: [{ code: "Stale" }],
    });
    expect(await applyClaimDelete(reviewed(claim))).toMatchObject({
      ok: false,
      errors: [{ code: "Stale" }],
    });
    expect(await read(claim.metadata.name)).toBeDefined();
  });

  it("refuses with Stale and keeps a claim that was made again under the same name", async () => {
    const claim = await made();
    await kubeRequest(kc, "DELETE", `${claimsPath}/${claim.metadata.name}`);
    const again = await made(claim.metadata.name);

    expect(await applyClaimDelete(reviewed(claim))).toMatchObject({
      ok: false,
      errors: [{ code: "Stale" }],
    });
    expect((await read(claim.metadata.name))?.metadata.uid).toBe(
      again.metadata.uid,
    );
  });
});

const planeClaimsPath = `/apis/fleet.camunda.io/v1alpha1/namespaces/${namespace}/camundamanagementclusterclaims`;

// planeMade creates a plane claim and returns it as the API server answered.
async function planeMade(spec: object = {}, finalizers: string[] = []) {
  return kubeRequest<KubeObject>(kc, "POST", planeClaimsPath, {
    apiVersion,
    kind: "CamundaManagementClusterClaim",
    metadata: {
      name: `plane-${randomUUID().slice(0, 8)}`,
      namespace,
      finalizers,
    },
    spec: {
      environments: [namespace],
      management: {
        identityProvider: { oidc: {} },
        identity: {
          externalUrl: "https://management.example.com/identity",
          admin: { claimName: "sub", claimValue: "42" },
        },
      },
      versions: { identity: "8.9.14" },
      ...spec,
    },
  });
}

async function planeRead(name: string): Promise<KubeObject | undefined> {
  try {
    return await kubeRequest<KubeObject>(
      kc,
      "GET",
      `${planeClaimsPath}/${name}`,
    );
  } catch (error) {
    if (error instanceof KubeError && error.status === 404) return undefined;
    throw error;
  }
}

// planeReviewed is the delete of a plane claim that the page sends after
// its review.
function planeReviewed(claim: KubeObject): Change {
  const { name } = claim.metadata;
  const kind = "CamundaManagementClusterClaim";
  const key = `${kind}/${namespace}/${name}` as const;
  return {
    intent: `Delete the management plane ${name}`,
    apply: [],
    delete: [{ apiVersion, kind, namespace, name }],
    uids: { [key]: claim.metadata.uid as string },
    generations: { [key]: claim.metadata.generation as number },
  };
}

describe("the delete of a plane claim against envtest", () => {
  it("deletes a plane claim after a status write since the review", async () => {
    const claim = await planeMade();
    await kubeRequest(
      kc,
      "PATCH",
      `${planeClaimsPath}/${claim.metadata.name}/status`,
      { status: { environment: "prod" } },
      "application/merge-patch+json",
    );

    expect(await previewPlaneClaimDelete(planeReviewed(claim))).toMatchObject({
      errors: [],
    });
    expect(await applyPlaneClaimDelete(planeReviewed(claim))).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(await planeRead(claim.metadata.name)).toBeUndefined();
  });

  it("deletes a plane claim with the cleanup finalizer whose request still has the old policy", async () => {
    const claim = await planeMade({ deletionPolicy: "Orphan" }, [
      "fleet.camunda.io/cleanup",
    ]);
    await kubeRequest(
      kc,
      "PATCH",
      `${planeClaimsPath}/${claim.metadata.name}/status`,
      {
        status: {
          observedGeneration: claim.metadata.generation,
          resolvedPolicy: { deletionPolicy: "Orphan" },
        },
      },
      "application/merge-patch+json",
    );
    await planeRequested(claim, "Delete");

    expect(await applyPlaneClaimDelete(planeReviewed(claim))).toEqual({
      ok: true,
      mode: "direct",
    });
    // The cleanup finalizer holds the claim until the operator removes it.
    expect(
      (await planeRead(claim.metadata.name))?.metadata.deletionTimestamp,
    ).toBeDefined();
  });

  it("refuses a plane claim without the cleanup finalizer while its request has another policy", async () => {
    const claim = await planeMade({ deletionPolicy: "Orphan" });
    const request = await planeRequested(claim, "Delete");

    expect(await applyPlaneClaimDelete(planeReviewed(claim))).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Stale",
          message: `The request ${request.metadata.name} does not have the deletion policy of the claim. The operator has not updated it.`,
        }),
      ],
    });
    expect(await planeRead(claim.metadata.name)).toBeDefined();
  });
});

// planeRequested creates a PluginRequest that the plane claim controls,
// with policy.
async function planeRequested(claim: KubeObject, policy: "Delete" | "Orphan") {
  const { name, uid } = claim.metadata;
  const kind = "CamundaManagementClusterClaim";
  return kubeRequest<KubeObject>(kc, "POST", requestsPath, {
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRequest",
    metadata: {
      name: `${name}-management-database`,
      namespace,
      ownerReferences: [{ apiVersion, kind, name, uid, controller: true }],
    },
    spec: {
      section: "management-database",
      registration: "echo",
      claimRef: { kind, name },
      target: { kind: "CamundaManagementCluster", name },
      deletionPolicy: policy,
    },
  });
}
