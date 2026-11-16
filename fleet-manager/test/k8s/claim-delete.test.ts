import { randomUUID } from "node:crypto";
import { KubeConfig } from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import {
  applyClaimDelete,
  previewClaimDelete,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
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
async function made(name = `orders-${randomUUID().slice(0, 8)}`) {
  return kubeRequest<KubeObject>(kc, "POST", claimsPath, {
    apiVersion,
    kind: "CamundaClusterClaim",
    metadata: { name, namespace },
    spec: { presetRef: "medium" },
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
  it("deletes a claim whose status changed since the review", async () => {
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
