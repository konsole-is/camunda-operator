import { randomUUID } from "node:crypto";
import { Watch } from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { applyContractChange } from "@/app/setup/connections/actions";
import { deleteChange } from "@/components/setup/connections/draft";
import type { ContractObject } from "@/components/setup/connections/facts";
import { ResourceCache } from "@/lib/k8s/cache";
import { getJSON } from "@/lib/k8s/client";
import { KindMap, type KindName } from "@/lib/k8s/kinds";
import { get, listFrom } from "@/lib/k8s/read";
import { type Envtest, startEnvtest } from "../envtest/envtest";
import { kubeRequest } from "../scenarios/kube";

// The reader may list every kind but watch only the kinds that WATCHED
// holds. A watch of any other kind fails on each try after its first list,
// as it does when the permission to watch is taken away.
const READER = "fleet-reader";
const UNWATCHED: readonly KindName[] = ["Environment", "PluginRequest"];
const WATCHED = (Object.keys(KindMap) as KindName[]).filter(
  (kind) => !UNWATCHED.includes(kind),
);

let envtest: Envtest;
const namespace = `watch-recovery-${randomUUID().slice(0, 8)}`;
const rbac = "/apis/rbac.authorization.k8s.io/v1";

async function grant(name: string, verbs: string[], kinds: "*" | KindName[]) {
  const kc = envtest.kubeConfig;
  const rules =
    kinds === "*"
      ? [{ apiGroups: ["*"], resources: ["*"], verbs }]
      : kinds.map((kind) => ({
          apiGroups: [KindMap[kind].group],
          resources: [KindMap[kind].plural],
          verbs,
        }));
  await kubeRequest(kc, "POST", `${rbac}/clusterroles`, {
    apiVersion: "rbac.authorization.k8s.io/v1",
    kind: "ClusterRole",
    metadata: { name },
    rules,
  });
  await kubeRequest(kc, "POST", `${rbac}/clusterrolebindings`, {
    apiVersion: "rbac.authorization.k8s.io/v1",
    kind: "ClusterRoleBinding",
    metadata: { name },
    roleRef: {
      apiGroup: "rbac.authorization.k8s.io",
      kind: "ClusterRole",
      name,
    },
    subjects: [
      { kind: "User", name: READER, apiGroup: "rbac.authorization.k8s.io" },
    ],
  });
}

beforeAll(async () => {
  envtest = await startEnvtest({ users: [{ name: READER }] });
  await grant("reader-list", ["get", "list"], "*");
  await grant("reader-watch", ["watch"], WATCHED);
  // The data layer reads KUBECONFIG through kubeConfig, on its first call.
  process.env.KUBECONFIG = envtest.users[READER].kubeconfigPath;
  await kubeRequest(envtest.kubeConfig, "POST", "/api/v1/namespaces", {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
  // The settings live in this namespace. It has none, so the writer is
  // direct.
  process.env.FLEET_NAMESPACE = namespace;
  delete process.env.FLEET_SETTINGS_GIT_REPOSITORY;
}, 120_000);
afterAll(async () => envtest?.stop(), 60_000);

describe("a watch that fails on each try after the first list", () => {
  it("makes the cache report the kind as not current within the limit, and current again once the watch works", async () => {
    const reader = envtest.users[READER].kubeConfig;
    const limitMs = 1_000;
    const cache = new ResourceCache({
      list: (path) => getJSON(reader, path, 10_000),
      watch: new Watch(reader),
      retryDelayMs: 100,
      maxRetryDelayMs: 200,
      watchRecoveryMs: limitMs,
    });
    try {
      const listed = Date.now();
      expect(await listFrom(cache, "Environment")).toEqual([]);

      await vi.waitFor(
        () =>
          expect(listFrom(cache, "Environment")).rejects.toThrow(
            /no current list of Environment.*Forbidden/,
          ),
        { timeout: 10_000 },
      );
      expect(Date.now() - listed).toBeLessThan(limitMs + 2_000);

      await kubeRequest(
        envtest.kubeConfig,
        "POST",
        "/apis/fleet.camunda.io/v1alpha1/environments",
        {
          apiVersion: "fleet.camunda.io/v1alpha1",
          kind: "Environment",
          metadata: { name: "missed" },
          spec: { displayName: "Created while the watch failed" },
        },
      );
      await grant("reader-watch-environments", ["watch"], ["Environment"]);

      const environments = await vi.waitFor(
        () => listFrom(cache, "Environment"),
        {
          timeout: 10_000,
        },
      );
      expect(environments.map((e) => e.metadata.name)).toEqual(["missed"]);
    } finally {
      await cache.stop();
    }
  }, 30_000);

  it("makes the delete of a connection contract refuse with the kind that a check reads", async () => {
    const name = "backups";
    await kubeRequest(
      envtest.kubeConfig,
      "POST",
      `/apis/core.camunda.io/v1/namespaces/${namespace}/objectstorageconfigs`,
      {
        apiVersion: "core.camunda.io/v1",
        kind: "ObjectStorageConfig",
        metadata: { name, namespace },
        spec: {
          type: "S3",
          s3: {
            bucketName: "backups",
            region: "eu-west-1",
            auth: {
              type: "workloadIdentity",
              workloadIdentity: {
                roleArn: "arn:aws:iam::123456789012:role/backups",
              },
            },
          },
        },
      },
    );
    const live = (await vi.waitFor(async () => {
      const found = await get("ObjectStorageConfig", namespace, name);
      expect(found).toBeDefined();
      return found;
    })) as unknown as ContractObject;

    // The first sync of PluginRequest worked, and its watch fails since.
    const result = await vi.waitFor(
      async () => {
        const refused = await applyContractChange(deleteChange(live));
        expect(refused.ok).toBe(false);
        expect(JSON.stringify(refused)).toContain("PluginRequest");
        return refused;
      },
      { timeout: 25_000, interval: 500 },
    );

    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Forbidden",
          message:
            "Delete is possible when the fleet manager can read PluginRequest.",
        }),
      ],
    });
  }, 60_000);
});
