import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  PatchStrategy,
} from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import { APP_FIELD_MANAGER, keptRemovals } from "@/lib/merge";
import type { KubeObject } from "@/lib/write/types";
import { createClientSideApplied } from "./client-side-apply";

// Each case builds a plane claim with real field managers, then saves a
// change with server-side apply without force, as the fleet manager does.
// keptRemovals must predict the object that the API server keeps.

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `kept-${randomUUID().slice(0, 8)}`;

type Spec = Record<string, unknown>;

const claim = (name: string, spec: Spec): KubeObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaManagementClusterClaim",
  metadata: { name, namespace },
  spec,
});

const spec = (over: Spec = {}): Spec => ({
  environments: ["prod"],
  management: {
    identityProvider: { oidc: {} },
    identity: { admin: { claimName: "sub", claimValue: "42" } },
  },
  versions: { identity: "8.9.14" },
  ...over,
});

const apply = (manager: string, object: KubeObject, force = true) =>
  api.patch(
    object,
    undefined,
    undefined,
    manager,
    force,
    PatchStrategy.ServerSideApply,
  );

const update = (manager: string, object: KubeObject) =>
  api.patch(
    object,
    undefined,
    undefined,
    manager,
    undefined,
    PatchStrategy.MergePatch,
  );

const read = (name: string) =>
  api.read({
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { name, namespace },
  }) as Promise<KubeObject & { spec: Spec }>;

// save applies after as applier without force, and returns the object that
// the server keeps with what keptRemovals predicted from the live object.
async function save(name: string, applier: string, after: Spec) {
  const live = await read(name);
  const predicted = keptRemovals(
    live as never,
    { spec: live.spec },
    { spec: after },
    applier,
  );
  const saved = (await apply(
    applier,
    claim(name, after),
    false,
  )) as unknown as {
    spec: Spec;
  };
  return { saved: saved.spec, predicted };
}

let name: string;
const fresh = () => (name = `plane-${randomUUID().slice(0, 8)}`);

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
});

describe("keptRemovals against the API server", () => {
  it("keeps a field of an Update entry that has the name of the applier", async () => {
    fresh();
    await apply("argocd-controller", claim(name, spec()));
    await update(
      "argocd-controller",
      claim(name, { deletionPolicy: "Orphan" }),
    );
    const { saved, predicted } = await save(name, "argocd-controller", spec());
    expect(saved.deletionPolicy).toBe("Orphan");
    expect(predicted.result).toEqual({ spec: saved });
    expect(predicted.kept).toEqual([
      { path: "spec.deletionPolicy", managers: ["argocd-controller"] },
    ]);
  });

  it("keeps a field that the fleet manager saved when the GitOps tool applies first", async () => {
    fresh();
    await apply(
      APP_FIELD_MANAGER,
      claim(name, spec({ deletionPolicy: "Orphan" })),
    );
    const { saved, predicted } = await save(name, "argocd-controller", spec());
    expect(saved.deletionPolicy).toBe("Orphan");
    expect(predicted.result).toEqual({ spec: saved });
    expect(predicted.kept).toEqual([
      { path: "spec.deletionPolicy", managers: [APP_FIELD_MANAGER] },
    ]);
  });

  it("keeps a field that kubectl applied client side when the GitOps tool applies first", async () => {
    fresh();
    await createClientSideApplied(
      api,
      claim(name, spec({ deletionPolicy: "Orphan" })),
    );
    const { saved, predicted } = await save(name, "argocd-controller", spec());
    expect(saved.deletionPolicy).toBe("Orphan");
    expect(predicted.result).toEqual({ spec: saved });
    expect(predicted.kept).toEqual([
      {
        path: "spec.deletionPolicy",
        managers: ["kubectl-client-side-apply"],
      },
    ]);
  });

  it("names the manager of each kept child of a map", async () => {
    fresh();
    const labelled = (labels: Record<string, string>) => ({
      ...claim(name, spec()),
      metadata: { name, namespace, labels },
    });
    await apply("a", labelled({ team: "a" }));
    await update("b", labelled({ tier: "gold" }));
    await apply(
      APP_FIELD_MANAGER,
      labelled({ team: "a", tier: "gold", app: "x" }),
    );
    const live = await read(name);
    const predicted = keptRemovals(
      live as never,
      { metadata: { labels: live.metadata.labels } },
      { metadata: {} },
    );
    const saved = (await apply(
      APP_FIELD_MANAGER,
      claim(name, spec()),
      false,
    )) as KubeObject;
    expect(predicted.result).toEqual({
      metadata: { labels: saved.metadata.labels },
    });
    expect(predicted.kept).toEqual([
      { path: "metadata.labels.team", managers: ["a"] },
      { path: "metadata.labels.tier", managers: ["b"] },
    ]);
  });

  it("puts a kept item of a set list where the server keeps it", async () => {
    fresh();
    await apply("b", claim(name, spec({ environments: ["qa"] })));
    await apply(
      APP_FIELD_MANAGER,
      claim(name, spec({ environments: ["qa", "prod"] })),
    );
    const { saved, predicted } = await save(
      name,
      APP_FIELD_MANAGER,
      spec({ environments: ["prod"] }),
    );
    expect(saved.environments).toContain("qa");
    expect(predicted.result).toEqual({ spec: saved });
  });

  it("refuses a save that leaves an empty object that another manager applied", async () => {
    fresh();
    const withConsole = (console: Spec) =>
      spec({
        management: {
          identityProvider: { oidc: {} },
          identity: { admin: { claimName: "sub", claimValue: "42" } },
          console,
        },
        versions: { identity: "8.9.14", console: "8.9.133" },
      });
    await apply("kubectl", claim(name, withConsole({})));
    await apply(
      APP_FIELD_MANAGER,
      claim(name, withConsole({ externalUrl: "https://console.example.com" })),
    );
    await expect(save(name, APP_FIELD_MANAGER, spec())).rejects.toThrow(
      /spec.management.console in body must be of type object/,
    );
  });
});
