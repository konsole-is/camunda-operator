import { randomUUID } from "node:crypto";
import { type KubernetesObject, Watch } from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { applyContractChange } from "@/app/setup/connections/actions";
import { deleteChange } from "@/components/setup/connections/draft";
import type { ContractObject } from "@/components/setup/connections/facts";
import { defaultCache, ResourceCache } from "@/lib/k8s/cache";
import { getJSON } from "@/lib/k8s/client";
import { KindMap, type KindName } from "@/lib/k8s/kinds";
import { get, listFrom } from "@/lib/k8s/read";
import { type Envtest, startEnvtest } from "../envtest/envtest";
import { kubeRequest } from "../scenarios/kube";

// Each test has its own user, which may get, list, and watch every kind
// but one. The watch of that kind fails on each try after its first list,
// as it does when the permission to watch is taken away.
const CACHE_READER = "cache-reader";
const ACTION_READER = "action-reader";

let envtest: Envtest;
const namespace = `watch-recovery-${randomUUID().slice(0, 8)}`;
const rbac = "/apis/rbac.authorization.k8s.io/v1";

async function grant(
  user: string,
  name: string,
  verbs: string[],
  kinds: "*" | KindName[],
) {
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
      { kind: "User", name: user, apiGroup: "rbac.authorization.k8s.io" },
    ],
  });
}

// readAllButWatchOf lets user get and list every kind, and watch every
// kind but unwatched.
async function readAllButWatchOf(user: string, unwatched: KindName) {
  await grant(user, `${user}-list`, ["get", "list"], "*");
  await grant(
    user,
    `${user}-watch`,
    ["watch"],
    (Object.keys(KindMap) as KindName[]).filter((k) => k !== unwatched),
  );
}

beforeAll(async () => {
  envtest = await startEnvtest({
    users: [{ name: CACHE_READER }, { name: ACTION_READER }],
  });
  // The data layer reads KUBECONFIG through kubeConfig, on its first call.
  process.env.KUBECONFIG = envtest.users[ACTION_READER].kubeconfigPath;
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
afterAll(async () => {
  await defaultCache().stop();
  await envtest?.stop();
}, 60_000);

describe("a watch that fails on each try after the first list", () => {
  it("makes the cache report the kind as not current within the limit, and current again once the watch works", async () => {
    await readAllButWatchOf(CACHE_READER, "Environment");
    const reader = envtest.users[CACHE_READER].kubeConfig;
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
      await grant(
        CACHE_READER,
        `${CACHE_READER}-watch-environments`,
        ["watch"],
        ["Environment"],
      );

      await vi.waitFor(
        async () =>
          expect(
            (await listFrom(cache, "Environment")).map((e) => e.metadata.name),
          ).toEqual(["missed"]),
        { timeout: 10_000 },
      );
    } finally {
      await cache.stop();
    }
  }, 30_000);

  it("makes the delete of a connection contract refuse with the kind that a check reads", async () => {
    await readAllButWatchOf(ACTION_READER, "PluginRequest");
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
        // The reader may not delete, so a call that the guard lets through
        // before the limit fails at the API server and deletes nothing.
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

describe("a kind that does not change", () => {
  it("stays current across server ends of its watch, with no new list", async () => {
    const admin = envtest.kubeConfig;
    const real = new Watch(admin);
    const watched: { resourceVersion?: unknown }[] = [];
    const bookmarks: string[] = [];
    const lists = vi.fn((path: string) => getJSON(admin, path, 10_000));
    const watch = {
      watch: (
        path: string,
        params: Record<string, string | number | boolean | undefined>,
        callback: (phase: string, object: KubernetesObject) => void,
        done: (err: unknown) => void,
      ) => {
        watched.push({ ...params });
        return real.watch(
          path,
          params,
          (phase, object) => {
            if (phase === "BOOKMARK") {
              bookmarks.push(object.metadata?.resourceVersion ?? "");
            }
            callback(phase, object);
          },
          done,
        );
      },
    } as unknown as Watch;
    const cache = new ResourceCache({
      list: lists as never,
      watch,
      watchTimeoutSeconds: 2,
      watchRecoveryMs: 1_000,
    });
    try {
      const lost = vi.fn();
      await cache.subscribe("BackupSchedule", vi.fn(), lost);
      // Writes of another kind move the resourceVersion of the API server.
      for (let i = 0; i < 3; i++) {
        await kubeRequest(admin, "POST", "/api/v1/namespaces", {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: { name: `quiet-${i}-${randomUUID().slice(0, 8)}` },
        });
      }

      await vi.waitFor(() => expect(watched.length).toBeGreaterThan(3), {
        timeout: 15_000,
        interval: 200,
      });

      expect(await listFrom(cache, "BackupSchedule")).toEqual([]);
      expect(lists).toHaveBeenCalledOnce();
      expect(lost).not.toHaveBeenCalled();
      if (bookmarks.length > 0) {
        expect(watched.at(-1)!.resourceVersion).not.toBe(
          watched[0].resourceVersion,
        );
      }
    } finally {
      await cache.stop();
    }
  }, 30_000);
});
