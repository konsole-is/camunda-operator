import { randomUUID } from "node:crypto";
import { KubeConfig } from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it, vi } from "vitest";
import { applyPlatformConfigChange } from "@/app/setup/platform/actions";
import { applyPresetChange } from "@/app/setup/presets/actions";
import { applyReleaseChange } from "@/app/setup/releases/actions";
import { get } from "@/lib/k8s/read";
import type { Change, KubeObject, WriteResult } from "@/lib/write/types";
import { KubeError, kubeRequest } from "../scenarios/kube";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const namespace = `setup-delete-${randomUUID().slice(0, 8)}`;
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

type Case = {
  kind: "CamundaClusterPreset" | "CamundaRelease" | "CamundaPlatformConfig";
  plural: string;
  spec: object;
  // claimSpec is the spec of a claim that uses the object name.
  claimSpec: (name: string) => object;
  apply: (change: Change) => Promise<WriteResult>;
};

const cases: Case[] = [
  {
    kind: "CamundaClusterPreset",
    plural: "camundaclusterpresets",
    spec: { cluster: {} },
    claimSpec: (name) => ({ presetRef: name }),
    apply: applyPresetChange,
  },
  {
    kind: "CamundaRelease",
    plural: "camundareleases",
    spec: { version: "8.9.0" },
    claimSpec: (name) => ({ presetRef: "medium", releaseRef: name }),
    apply: applyReleaseChange,
  },
  {
    kind: "CamundaPlatformConfig",
    plural: "camundaplatformconfigs",
    spec: { auth: { method: "basic" } },
    claimSpec: (name) => ({ presetRef: "medium", platformConfigRef: name }),
    apply: applyPlatformConfigChange,
  },
];

// removal is the delete of object that the page sends after its review.
function removal(object: KubeObject): Change {
  const { name, uid, resourceVersion } = object.metadata;
  const key = `${object.kind}//${name}` as const;
  return {
    intent: `Delete ${name}`,
    apply: [],
    delete: [{ apiVersion: "core.camunda.io/v1", kind: object.kind, name }],
    preconditions: { [key]: resourceVersion as string },
    uids: { [key]: uid as string },
  };
}

async function exists(path: string): Promise<boolean> {
  try {
    await kubeRequest(kc, "GET", path);
    return true;
  } catch (error) {
    if (error instanceof KubeError && error.status === 404) return false;
    throw error;
  }
}

describe.each(cases)(
  "the delete of a $kind against envtest",
  ({ kind, plural, spec, claimSpec, apply }) => {
    it("refuses while a claim uses it, and deletes it once the claim is gone", async () => {
      const name = `guard-${randomUUID().slice(0, 8)}`;
      const path = `/apis/core.camunda.io/v1/${plural}`;
      const object = await kubeRequest<KubeObject>(kc, "POST", path, {
        apiVersion: "core.camunda.io/v1",
        kind,
        metadata: { name },
        spec,
      });
      const claim = await kubeRequest<KubeObject>(kc, "POST", claimsPath, {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaClusterClaim",
        metadata: { name, namespace },
        spec: claimSpec(name),
      });
      // The action reads the cache of the server, which gets the claim from
      // its watch.
      await vi.waitFor(async () =>
        expect(await get("CamundaClusterClaim", namespace, name)).toBeDefined(),
      );

      const refused = await apply(removal(object));
      expect(refused.ok).toBe(false);
      expect(!refused.ok && refused.errors).toEqual([
        expect.objectContaining({
          code: "Forbidden",
          message: expect.stringContaining(
            `Used by CamundaClusterClaim ${namespace}/${name}`,
          ),
        }),
      ]);
      expect(await exists(`${path}/${name}`)).toBe(true);

      await kubeRequest(kc, "DELETE", `${claimsPath}/${claim.metadata.name}`);
      await vi.waitFor(async () =>
        expect(
          await get("CamundaClusterClaim", namespace, name),
        ).toBeUndefined(),
      );
      expect(await apply(removal(object))).toEqual({
        ok: true,
        mode: "direct",
      });
      expect(await exists(`${path}/${name}`)).toBe(false);
    });
  },
);
