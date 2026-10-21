import { randomUUID } from "node:crypto";
import { KubeConfig, KubernetesObjectApi } from "@kubernetes/client-node";
import { afterAll, beforeAll, describe, expect, inject, it } from "vitest";
import { defaultCache } from "@/lib/k8s/cache";
import { events, podUsage } from "@/lib/observe/kube";

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `observe-${randomUUID().slice(0, 8)}`;

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});
afterAll(async () => {
  await defaultCache().stop();
});

describe("observe readers against envtest", () => {
  it("podUsage returns unavailable without metrics-server", async () => {
    expect(await podUsage(namespace, "")).toBe("unavailable");
  });

  it("events returns an event recorded on a PluginRequest", async () => {
    await api.create({
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginRequest",
      metadata: { name: "orders-access", namespace },
      spec: {
        section: "access",
        registration: "echo",
        claimRef: { kind: "CamundaClusterClaim", name: "orders" },
        target: { kind: "CamundaCluster", name: "orders" },
        deletionPolicy: "Delete",
      },
    });
    for (const [name, regarding] of [
      ["orders-access.1", "orders-access"],
      ["orders-other.1", "orders-other"],
    ]) {
      await api.create({
        apiVersion: "events.k8s.io/v1",
        kind: "Event",
        metadata: { name, namespace },
        // eventTime is a MicroTime, which needs six digits after the second.
        eventTime: new Date().toISOString().replace(/Z$/, "000Z"),
        reportingController: "example.com/plugin",
        reportingInstance: "plugin-0",
        action: "Apply",
        reason: regarding === "orders-access" ? "Applied" : "Other",
        type: "Normal",
        note: "Applied the bucket",
        regarding: {
          apiVersion: "plugins.camunda.io/v1alpha1",
          kind: "PluginRequest",
          name: regarding,
          namespace,
        },
      } as never);
    }

    await expect
      .poll(
        async () =>
          (
            await events({ kind: "PluginRequest", name: "orders-access", namespace })
          ).map((e) => e.reason),
        { timeout: 10_000 },
      )
      .toEqual(["Applied"]);
  });
});
