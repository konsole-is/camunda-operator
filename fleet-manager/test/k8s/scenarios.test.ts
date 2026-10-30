import path from "node:path";
import type { KubeConfig } from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { type Envtest, startEnvtest } from "../envtest/envtest";
import { Discovery, KubeError, kubeRequest } from "../scenarios/kube";
import {
  LOADER_MANAGER,
  type Scenario,
  ScenarioLoader,
  openScenario,
} from "../scenarios/loader";
import { type KubeObject, readObjects } from "../scenarios/objects";
import { bindChartRBAC } from "../scenarios/rbac";

const USER = "fleet-manager-scenarios";
const OTHER_USER = "other-scenario-user";

// This file loads whole scenarios, whose objects would collide with the
// objects of the other files, so it starts an envtest of its own.
let envtest: Envtest;

beforeAll(async () => {
  envtest = await startEnvtest({
    users: [{ name: USER }, { name: OTHER_USER }],
  });
}, 120_000);

afterAll(async () => {
  await envtest?.stop();
});

describe("the users option of startEnvtest", () => {
  it("authenticates a user that is not an admin", async () => {
    const user = envtest.users[USER];
    const review = await kubeRequest<{
      status: { userInfo: { username: string; groups: string[] } };
    }>(
      user.kubeConfig,
      "POST",
      "/apis/authentication.k8s.io/v1/selfsubjectreviews",
      { apiVersion: "authentication.k8s.io/v1", kind: "SelfSubjectReview" },
    );
    expect(review.status.userInfo.username).toBe(USER);
    expect(review.status.userInfo.groups).not.toContain("system:masters");
    await expect(
      kubeRequest(user.kubeConfig, "GET", "/api/v1/namespaces/default/secrets"),
    ).rejects.toMatchObject({ status: 403 });
  });

  it("refuses a user named envtest-admin before anything starts", async () => {
    await expect(
      startEnvtest({ users: [{ name: "envtest-admin" }] }),
    ).rejects.toThrow(/envtest-admin/);
  });

  it("refuses a user name that is given twice", async () => {
    await expect(
      startEnvtest({ users: [{ name: "a" }, { name: "a" }] }),
    ).rejects.toThrow(/a.*twice/);
  });

  it("writes a kubeconfig for a user name with a slash", async () => {
    const slashed = await startEnvtest({ users: [{ name: "team/alice" }] });
    try {
      const kc = slashed.users["team/alice"].kubeConfig;
      const review = await kubeRequest<{
        status: { userInfo: { username: string } };
      }>(kc, "POST", "/apis/authentication.k8s.io/v1/selfsubjectreviews", {
        apiVersion: "authentication.k8s.io/v1",
        kind: "SelfSubjectReview",
      });
      expect(review.status.userInfo.username).toBe("team/alice");
    } finally {
      await slashed.stop();
    }
  }, 120_000);

  it("refuses a user in system:masters, also in a group list", async () => {
    await expect(
      startEnvtest({ users: [{ name: "b", groups: ["system:masters"] }] }),
    ).rejects.toThrow(/b.*system:masters/);
    await expect(
      startEnvtest({ users: [{ name: "b", groups: ["x,system:masters"] }] }),
    ).rejects.toThrow(/b.*comma/);
  });
});

type ManagedField = {
  manager: string;
  operation: string;
  subresource?: string;
};

function managers(o: KubeObject): string[][] {
  return ((o.metadata.managedFields ?? []) as ManagedField[])
    .map((f) => [f.manager, f.operation, f.subresource ?? ""])
    .sort((a, b) => a.join("/").localeCompare(b.join("/")));
}

async function getLive(kc: KubeConfig, o: KubeObject): Promise<KubeObject> {
  return kubeRequest<KubeObject>(kc, "GET", await new Discovery(kc).path(o));
}

async function gone(kc: KubeConfig, o: KubeObject): Promise<boolean> {
  try {
    await getLive(kc, o);
    return false;
  } catch (error) {
    if (error instanceof KubeError && error.status === 404) return true;
    throw error;
  }
}

describe("the scenario loader", () => {
  it("one update with the recorded managedFields leaves no entry of the loader", async () => {
    const pod: KubeObject = {
      apiVersion: "v1",
      kind: "Pod",
      metadata: {
        name: "fields",
        namespace: "managed-fields",
        uid: "00000000-0000-5000-8000-0000000000f1",
        managedFields: [
          {
            apiVersion: "v1",
            fieldsType: "FieldsV1",
            fieldsV1: { "f:spec": { "f:containers": {} } },
            manager: "kubectl",
            operation: "Update",
            time: "2026-01-01T00:00:00Z",
          },
          {
            apiVersion: "v1",
            fieldsType: "FieldsV1",
            fieldsV1: { "f:status": { "f:phase": {} } },
            manager: "kubelet",
            operation: "Update",
            subresource: "status",
            time: "2026-01-01T00:00:00Z",
          },
        ],
      },
      spec: {
        nodeName: "scenario-node",
        containers: [{ name: "app", image: "registry.example.com/app:1" }],
      },
      status: { phase: "Running" },
    };
    const namespace: KubeObject = {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: "managed-fields" },
    };

    await new ScenarioLoader(envtest.kubeConfig).load([namespace, pod]);

    const live = await getLive(envtest.kubeConfig, pod);
    expect(managers(live)).toEqual([
      ["kubectl", "Update", ""],
      ["kubelet", "Update", "status"],
    ]);
    expect(live.status).toMatchObject({ phase: "Running" });
  });

  it("refuses a field that the CRD does not have and names it", async () => {
    const release: KubeObject = {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaRelease",
      metadata: { name: "strict-check" },
      spec: { version: "8.9.0", notAField: "x" },
    };
    const load = new ScenarioLoader(envtest.kubeConfig).load([release]);
    await expect(load).rejects.toThrow(/notAField/);
    await expect(load).rejects.toThrow(/CamundaRelease strict-check/);
  });

  describe("on the cluster-ready scenario", () => {
    const dir = path.resolve("e2e/scenarios/cluster-ready");
    let recorded: KubeObject[];
    let scenario: Scenario;

    beforeAll(async () => {
      recorded = await readObjects(path.join(dir, "objects.yaml"));
      scenario = await openScenario(envtest.kubeConfig, dir);
    }, 120_000);

    it("load leaves no scenario UID in any object of cluster-ready", async () => {
      const scenarioUIDs = recorded.map((o) => o.metadata.uid!);
      const live = await Promise.all(
        recorded.map((o) => getLive(envtest.kubeConfig, o)),
      );

      const text = JSON.stringify(live);
      expect(scenarioUIDs.filter((uid) => text.includes(uid))).toEqual([]);
      for (const o of live) {
        expect(managers(o).map(([m]) => m)).not.toContain(LOADER_MANAGER);
      }
    });

    it("load keeps the field managers of the reconcilers", async () => {
      const cluster = recorded.find((o) => o.kind === "CamundaCluster")!;
      const live = await getLive(envtest.kubeConfig, cluster);

      const recordedManagers = managers(cluster).map(([m, op, sub]) => [
        m.replace(cluster.metadata.uid!, live.metadata.uid!),
        op,
        sub,
      ]);
      expect(managers(live)).toEqual(recordedManagers);
      expect(managers(live).map(([m]) => m)).toContain("kubectl");
    });

    it("advance to three-brokers-ready leaves no scenario UID and no entry of the loader", async () => {
      const state = await readObjects(
        path.join(dir, "three-brokers-ready.yaml"),
      );
      await scenario.advance("three-brokers-ready");

      const live = await Promise.all(
        state.map((o) => getLive(envtest.kubeConfig, o)),
      );
      const text = JSON.stringify(live);
      const scenarioUIDs = state.map((o) => o.metadata.uid!);
      expect(scenarioUIDs.filter((uid) => text.includes(uid))).toEqual([]);
      for (const o of live) {
        expect(managers(o).map(([m]) => m)).not.toContain(LOADER_MANAGER);
      }
      const zeebe = live.find(
        (o) =>
          o.metadata.name === "camundacluster-sample-zeebe" &&
          o.kind === "StatefulSet",
      )!;
      expect(zeebe.status).toMatchObject({ readyReplicas: 3 });
    });
  });

  describe("advance", () => {
    const dir = path.resolve("test/fixtures/scenarios/advance");
    let scenario: Scenario;
    const deployment = {
      apiVersion: "apps/v1",
      kind: "Deployment",
      metadata: { name: "app", namespace: "advance" },
    };

    beforeAll(async () => {
      scenario = await openScenario(envtest.kubeConfig, dir);
      await scenario.advance("moved");
    }, 60_000);

    it("advance updates a spec and keeps the status observed", async () => {
      const app = await getLive(envtest.kubeConfig, deployment);
      expect(app.spec).toMatchObject({ replicas: 2 });
      expect(app.metadata.generation).toBe(2);
      expect(app.status).toMatchObject({ observedGeneration: 2 });

      await getLive(envtest.kubeConfig, {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name: "new", namespace: "advance" },
      });
      const rs = await getLive(envtest.kubeConfig, {
        apiVersion: "apps/v1",
        kind: "ReplicaSet",
        metadata: { name: "app-1", namespace: "advance" },
      });
      expect(rs.metadata.ownerReferences?.map((r) => r.uid)).toEqual([
        app.metadata.uid,
      ]);
    });

    it("advance clears a status that the state does not have", async () => {
      const rs = await getLive(envtest.kubeConfig, {
        apiVersion: "apps/v1",
        kind: "ReplicaSet",
        metadata: { name: "app-1", namespace: "advance" },
      });
      expect(rs.status).not.toHaveProperty("readyReplicas");
    });

    it("advance writes the UID of a new object into an object that did not change", async () => {
      const created = await getLive(envtest.kubeConfig, {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name: "new", namespace: "advance" },
      });
      const names = await getLive(envtest.kubeConfig, {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name: "names-new", namespace: "advance" },
      });
      expect(names.metadata.annotations).toEqual({
        "scenario.test/new-uid": created.metadata.uid,
      });
    });

    it("advance deletes an object with a finalizer, a Pod, and a Namespace", async () => {
      const removed = [
        {
          apiVersion: "v1",
          kind: "ConfigMap",
          metadata: { name: "gone", namespace: "advance" },
        },
        {
          apiVersion: "v1",
          kind: "Pod",
          metadata: { name: "gone-pod", namespace: "advance" },
        },
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: { name: "advance-gone" },
        },
      ];
      await expect
        .poll(
          async () =>
            Promise.all(removed.map((o) => gone(envtest.kubeConfig, o))),
          { timeout: 10_000 },
        )
        .toEqual([true, true, true]);
    });

    it("advance to an unknown state names the states", async () => {
      await expect(scenario.advance("nope")).rejects.toThrow(
        "The scenario advance has no state nope. Its states are: moved, back",
      );
    });

    it("advance writes the new UID of a recreated object into an object that did not change", async () => {
      await scenario.advance("back");

      const recreated = await getLive(envtest.kubeConfig, {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name: "gone", namespace: "advance" },
      });
      const names = await getLive(envtest.kubeConfig, {
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: { name: "names-gone", namespace: "advance" },
      });
      expect(names.metadata.annotations).toEqual({
        "scenario.test/gone-uid": recreated.metadata.uid,
      });
    });
  });
});

async function allowed(
  kc: KubeConfig,
  attributes: Record<string, string>,
): Promise<boolean> {
  const review = await kubeRequest<{ status: { allowed: boolean } }>(
    kc,
    "POST",
    "/apis/authorization.k8s.io/v1/selfsubjectaccessreviews",
    {
      apiVersion: "authorization.k8s.io/v1",
      kind: "SelfSubjectAccessReview",
      spec: { resourceAttributes: attributes },
    },
  );
  return review.status.allowed;
}

describe("bindChartRBAC", () => {
  it("binds the chart rules to the user", async () => {
    try {
      await kubeRequest(envtest.kubeConfig, "POST", "/api/v1/namespaces", {
        apiVersion: "v1",
        kind: "Namespace",
        metadata: { name: "camunda-system" },
      });
    } catch (error) {
      if (!(error instanceof KubeError && error.status === 409)) throw error;
    }
    const user = envtest.users[USER].kubeConfig;
    const listClusters = {
      group: "core.camunda.io",
      resource: "camundaclusters",
      verb: "list",
    };
    expect(await allowed(user, listClusters)).toBe(false);

    await bindChartRBAC(envtest.kubeConfig, USER, "camunda-system");
    // A second binding finds the objects of the first and does not fail.
    await bindChartRBAC(envtest.kubeConfig, USER, "camunda-system");

    expect(await allowed(user, listClusters)).toBe(true);
    expect(await allowed(user, { ...listClusters, verb: "delete" })).toBe(
      false,
    );
    expect(
      await allowed(user, {
        namespace: "camunda-system",
        resource: "configmaps",
        name: "fleet-manager-settings",
        verb: "update",
      }),
    ).toBe(true);
  });

  it("binds the chart rules to a second user too", async () => {
    const other = envtest.users[OTHER_USER].kubeConfig;
    await bindChartRBAC(envtest.kubeConfig, USER, "camunda-system");
    await bindChartRBAC(envtest.kubeConfig, OTHER_USER, "camunda-system");

    expect(
      await allowed(other, {
        group: "core.camunda.io",
        resource: "camundaclusters",
        verb: "list",
      }),
    ).toBe(true);
    expect(
      await allowed(other, {
        namespace: "camunda-system",
        resource: "configmaps",
        name: "fleet-manager-settings",
        verb: "update",
      }),
    ).toBe(true);
  });
});
