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
// No test binds a role to PLAIN_USER.
const PLAIN_USER = "plain-scenario-user";
const FIRST_USER = "first-scenario-user";
const SECOND_USER = "second-scenario-user";
// The RBAC tests bind in a namespace that no scenario holds.
const RBAC_NAMESPACE = "rbac-scenarios";

// This file loads whole scenarios, whose objects would collide with the
// objects of the other files, so it starts an envtest of its own.
let envtest: Envtest;

beforeAll(async () => {
  envtest = await startEnvtest({
    users: [USER, PLAIN_USER, FIRST_USER, SECOND_USER].map((name) => ({
      name,
    })),
  });
}, 120_000);

afterAll(async () => {
  await envtest?.stop();
});

describe("the users option of startEnvtest", () => {
  it("authenticates a user that is not an admin", async () => {
    const user = envtest.users[PLAIN_USER];
    const review = await kubeRequest<{
      status: { userInfo: { username: string; groups: string[] } };
    }>(
      user.kubeConfig,
      "POST",
      "/apis/authentication.k8s.io/v1/selfsubjectreviews",
      { apiVersion: "authentication.k8s.io/v1", kind: "SelfSubjectReview" },
    );
    expect(review.status.userInfo.username).toBe(PLAIN_USER);
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

  it("writes a kubeconfig for a user name with a slash or a colon", async () => {
    const names = ["team/alice", "a: b"];
    const odd = await startEnvtest({ users: names.map((name) => ({ name })) });
    try {
      for (const name of names) {
        const review = await kubeRequest<{
          status: { userInfo: { username: string } };
        }>(
          odd.users[name].kubeConfig,
          "POST",
          "/apis/authentication.k8s.io/v1/selfsubjectreviews",
          { apiVersion: "authentication.k8s.io/v1", kind: "SelfSubjectReview" },
        );
        expect(review.status.userInfo.username).toBe(name);
      }
    } finally {
      await odd.stop();
    }
  }, 120_000);

  it("refuses a user name that tokens.csv cannot hold", async () => {
    for (const name of ["a,uid,system:masters", 'a"b', "a\nb"]) {
      await expect(startEnvtest({ users: [{ name }] })).rejects.toThrow(
        /tokens.csv/,
      );
    }
  });

  it("refuses a user in system:masters, also in a group list", async () => {
    await expect(
      startEnvtest({ users: [{ name: "b", groups: ["system:masters"] }] }),
    ).rejects.toThrow(/b.*system:masters/);
    for (const group of ["x,system:masters", 'x"', "x\ny"]) {
      await expect(
        startEnvtest({ users: [{ name: "b", groups: [group] }] }),
      ).rejects.toThrow(/b.*tokens.csv/);
    }
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
    let loaded: KubeObject[];
    let state: KubeObject[];
    let advanced: KubeObject[];

    beforeAll(async () => {
      const kc = envtest.kubeConfig;
      recorded = await readObjects(path.join(dir, "objects.yaml"));
      const scenario = await openScenario(kc, dir);
      loaded = await Promise.all(recorded.map((o) => getLive(kc, o)));
      state = await readObjects(path.join(dir, "three-brokers-ready.yaml"));
      await scenario.advance("three-brokers-ready");
      advanced = await Promise.all(state.map((o) => getLive(kc, o)));
    }, 120_000);

    it("load leaves no scenario UID in any object of cluster-ready", () => {
      const scenarioUIDs = recorded.map((o) => o.metadata.uid!);
      const text = JSON.stringify(loaded);
      expect(scenarioUIDs.filter((uid) => text.includes(uid))).toEqual([]);
      for (const o of loaded) {
        expect(managers(o).map(([m]) => m)).not.toContain(LOADER_MANAGER);
      }
    });

    it("load keeps the field managers of the reconcilers", () => {
      const index = recorded.findIndex((o) => o.kind === "CamundaCluster");
      const cluster = recorded[index];
      const live = loaded[index];

      const recordedManagers = managers(cluster).map(([m, op, sub]) => [
        m.replace(cluster.metadata.uid!, live.metadata.uid!),
        op,
        sub,
      ]);
      expect(managers(live)).toEqual(recordedManagers);
      expect(managers(live).map(([m]) => m)).toContain("kubectl");
    });

    it("advance to three-brokers-ready leaves no scenario UID and no entry of the loader", () => {
      const text = JSON.stringify(advanced);
      const scenarioUIDs = state.map((o) => o.metadata.uid!);
      expect(scenarioUIDs.filter((uid) => text.includes(uid))).toEqual([]);
      for (const o of advanced) {
        expect(managers(o).map(([m]) => m)).not.toContain(LOADER_MANAGER);
      }
      const zeebe = advanced.find(
        (o) =>
          o.metadata.name === "camundacluster-sample-zeebe" &&
          o.kind === "StatefulSet",
      )!;
      expect(zeebe.status).toMatchObject({ readyReplicas: 3 });
    });
  });

  describe("advance", () => {
    const dir = path.resolve("test/fixtures/scenarios/advance");
    const inAdvance = (apiVersion: string, kind: string, name: string) => ({
      apiVersion,
      kind,
      metadata: { name, namespace: "advance" },
    });
    let scenario: Scenario;
    // What the API server holds after advance to moved, and then to back.
    let moved: Record<string, KubeObject>;
    let removedGone: boolean[];
    let back: Record<string, KubeObject>;

    beforeAll(async () => {
      const kc = envtest.kubeConfig;
      const read = async (objs: Record<string, KubeObject>) =>
        Object.fromEntries(
          await Promise.all(
            Object.entries(objs).map(
              async ([key, o]) => [key, await getLive(kc, o)] as const,
            ),
          ),
        );
      scenario = await openScenario(kc, dir);

      await scenario.advance("moved");
      moved = await read({
        app: inAdvance("apps/v1", "Deployment", "app"),
        rs: inAdvance("apps/v1", "ReplicaSet", "app-1"),
        created: inAdvance("v1", "ConfigMap", "new"),
        namesNew: inAdvance("v1", "ConfigMap", "names-new"),
      });
      const removed = [
        inAdvance("v1", "ConfigMap", "gone"),
        inAdvance("v1", "Pod", "gone-pod"),
        {
          apiVersion: "v1",
          kind: "Namespace",
          metadata: { name: "advance-gone" },
        },
      ];
      const deadline = Date.now() + 10_000;
      do {
        removedGone = await Promise.all(removed.map((o) => gone(kc, o)));
        if (removedGone.every(Boolean)) break;
        await new Promise((resolve) => setTimeout(resolve, 200));
      } while (Date.now() < deadline);

      await scenario.advance("back");
      back = await read({
        recreated: inAdvance("v1", "ConfigMap", "gone"),
        namesGone: inAdvance("v1", "ConfigMap", "names-gone"),
      });
    }, 60_000);

    it("advance updates a spec and keeps the status observed", () => {
      const { app, rs } = moved;
      expect(app.spec).toMatchObject({ replicas: 2 });
      expect(app.metadata.generation).toBe(2);
      expect(app.status).toMatchObject({ observedGeneration: 2 });
      expect(rs.metadata.ownerReferences?.map((r) => r.uid)).toEqual([
        app.metadata.uid,
      ]);
    });

    it("advance clears a status that the state does not have", () => {
      expect(moved.rs.status).not.toHaveProperty("readyReplicas");
    });

    it("advance writes the UID of a new object into an object that did not change", () => {
      expect(moved.namesNew.metadata.annotations).toEqual({
        "scenario.test/new-uid": moved.created.metadata.uid,
      });
    });

    it("advance deletes an object with a finalizer, a Pod, and a Namespace", () => {
      expect(removedGone).toEqual([true, true, true]);
    });

    it("advance writes the new UID of a recreated object into an object that did not change", () => {
      expect(back.namesGone.metadata.annotations).toEqual({
        "scenario.test/gone-uid": back.recreated.metadata.uid,
      });
    });

    it("advance to an unknown state names the states", async () => {
      await expect(scenario.advance("nope")).rejects.toThrow(
        "The scenario advance has no state nope. Its states are: moved, back",
      );
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
  beforeAll(async () => {
    await kubeRequest(envtest.kubeConfig, "POST", "/api/v1/namespaces", {
      apiVersion: "v1",
      kind: "Namespace",
      metadata: { name: RBAC_NAMESPACE },
    });
  });

  it("binds the chart rules to the user", async () => {
    const user = envtest.users[USER].kubeConfig;
    const listClusters = {
      group: "core.camunda.io",
      resource: "camundaclusters",
      verb: "list",
    };
    expect(await allowed(user, listClusters)).toBe(false);

    await bindChartRBAC(envtest.kubeConfig, USER, RBAC_NAMESPACE);
    // A second binding finds the objects of the first and does not fail.
    await bindChartRBAC(envtest.kubeConfig, USER, RBAC_NAMESPACE);

    expect(await allowed(user, listClusters)).toBe(true);
    expect(await allowed(user, { ...listClusters, verb: "delete" })).toBe(
      false,
    );
    expect(
      await allowed(user, {
        namespace: RBAC_NAMESPACE,
        resource: "configmaps",
        name: "fleet-manager-settings",
        verb: "update",
      }),
    ).toBe(true);
  });

  it("does not bind a built-in role that has the name of the user", async () => {
    const edit = await startEnvtest({ users: [{ name: "edit" }] });
    try {
      await bindChartRBAC(edit.kubeConfig, "edit", "default");
      const kc = edit.users["edit"].kubeConfig;
      // The built-in ClusterRole edit allows a delete of a Secret, and the
      // chart rules do not.
      expect(
        await allowed(kc, {
          namespace: "default",
          resource: "secrets",
          verb: "delete",
        }),
      ).toBe(false);
      expect(
        await allowed(kc, {
          group: "core.camunda.io",
          resource: "camundaclusters",
          verb: "list",
        }),
      ).toBe(true);
    } finally {
      await edit.stop();
    }
  }, 120_000);

  it("binds the chart rules to a second user too", async () => {
    const other = envtest.users[SECOND_USER].kubeConfig;
    await bindChartRBAC(envtest.kubeConfig, FIRST_USER, RBAC_NAMESPACE);
    await bindChartRBAC(envtest.kubeConfig, SECOND_USER, RBAC_NAMESPACE);

    expect(
      await allowed(other, {
        group: "core.camunda.io",
        resource: "camundaclusters",
        verb: "list",
      }),
    ).toBe(true);
    expect(
      await allowed(other, {
        namespace: RBAC_NAMESPACE,
        resource: "configmaps",
        name: "fleet-manager-settings",
        verb: "update",
      }),
    ).toBe(true);
  });
});
