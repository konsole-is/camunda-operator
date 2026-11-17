import { randomUUID } from "node:crypto";
import {
  KubeConfig,
  KubernetesObjectApi,
  PatchStrategy,
} from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import {
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { createCluster } from "@/app/fleet/clusters/new/actions";
import {
  applyEnvironmentChange,
  previewEnvironmentChange,
} from "@/app/setup/environments/actions";
import {
  applyReleaseMove,
  previewReleaseMove,
} from "@/app/setup/releases/actions";
import {
  APP_FIELD_MANAGER,
  fieldManagers,
  type ManagedFieldsEntry,
  type ManagedObject,
} from "@/lib/merge";
import { createChangeOf } from "@/lib/write/create";
import { offered, withTakeOver } from "@/lib/write/ownership";
import type { Change, KubeObject, Preview } from "@/lib/write/types";
import { createClientSideApplied } from "./client-side-apply";

// Each case builds an object with real field managers, then saves a change
// through the server actions of the pages, as a review does: preview, then
// apply the change, or the change with the take-over of the preview.

// The data layer reads KUBECONFIG through kubeConfig, on its first call.
process.env.KUBECONFIG = inject("kubeconfig");

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const namespace = `take-over-${randomUUID().slice(0, 8)}`;
const fleet = "fleet.camunda.io/v1alpha1";
const operator =
  "CamundaClusterClaim/cluster/0b5c7a2e-1f0e-4f43-9d0a-6a0c2f1e9b11";

beforeAll(async () => {
  await api.create({
    apiVersion: "v1",
    kind: "Namespace",
    metadata: { name: namespace },
  });
  // The settings live in this namespace. It has none, so the writer is
  // direct.
  process.env.FLEET_NAMESPACE = namespace;
  delete process.env.FLEET_SETTINGS_GIT_REPOSITORY;
});

const unique = (prefix: string) => `${prefix}-${randomUUID().slice(0, 8)}`;

// The editors send deletionPolicy, which the API server defaults, so a
// field that an Update entry got from the default is not cleared.
const environment = (name: string, spec: object): KubeObject => ({
  apiVersion: fleet,
  kind: "Environment",
  metadata: { name },
  spec: { deletionPolicy: "Delete", ...spec },
});

const clusterClaim = (name: string, spec: object): KubeObject => ({
  apiVersion: fleet,
  kind: "CamundaClusterClaim",
  metadata: { name, namespace },
  spec,
});

const read = async (object: KubeObject) =>
  (await api.read({
    apiVersion: object.apiVersion,
    kind: object.kind,
    metadata: object.metadata,
  })) as unknown as KubeObject & { spec: Record<string, unknown> };

const applyAs = (manager: string, object: KubeObject, force = false) =>
  api.patch(
    object,
    undefined,
    undefined,
    manager,
    force,
    PatchStrategy.ServerSideApply,
  );

// update writes object as a JSON merge patch, as kubectl edit or a plain
// kubectl apply does.
const update = (manager: string, object: KubeObject) =>
  api.patch(
    object,
    undefined,
    undefined,
    manager,
    undefined,
    PatchStrategy.MergePatch,
  );

const keyOf = (object: KubeObject) =>
  `${object.kind}/${object.metadata.namespace ?? ""}/${object.metadata.name}` as const;

// updateOf is the change of a page that applies object over live, at the
// resourceVersion of live.
const updateOf = (
  intent: string,
  live: KubeObject,
  object: KubeObject,
): Change => ({
  intent,
  apply: [object],
  preconditions: { [keyOf(object)]: live.metadata.resourceVersion! },
});

// owns reports whether an entry of manager owns the field at keys of
// FieldsV1, such as ["f:spec", "f:displayName"].
const owns = (
  object: KubeObject,
  manager: string,
  keys: string[],
  operation?: string,
) =>
  ((object.metadata.managedFields ?? []) as ManagedFieldsEntry[]).some(
    (entry) => {
      if (entry.manager !== manager) return false;
      if (operation !== undefined && entry.operation !== operation)
        return false;
      let at: unknown = entry.fieldsV1;
      for (const key of keys) {
        if (typeof at !== "object" || at === null || !(key in at)) return false;
        at = (at as Record<string, unknown>)[key];
      }
      return true;
    },
  );

const changedPaths = (preview: Preview) =>
  (preview.ownership ?? []).flatMap((o) => o.changed.map((f) => f.path));
const clearedPaths = (preview: Preview) =>
  (preview.ownership ?? []).flatMap((o) => o.cleared.map((f) => f.path));

describe("a take-over against the API server", () => {
  it("a change of a field that kubectl apply set conflicts without the choice and writes nothing", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await createClientSideApplied(api, made);
    const live = await read(made);
    const change = updateOf(
      "Rename",
      live,
      environment(made.metadata.name, { displayName: "Prod" }),
    );

    const preview = await previewEnvironmentChange(change);

    expect(preview.ownership).toEqual([
      expect.objectContaining({
        changed: [
          expect.objectContaining({
            path: "spec.displayName",
            managers: [
              {
                name: "kubectl-client-side-apply",
                operation: "Update",
                kind: "other",
                known: true,
              },
            ],
          }),
        ],
        cleared: [],
      }),
    ]);
    expect(preview.errors).toEqual([
      expect.objectContaining({ code: "Conflict", path: "spec.displayName" }),
    ]);
    expect(await applyEnvironmentChange(change)).toMatchObject({
      ok: false,
      errors: [{ code: "Conflict" }],
    });
    expect((await read(made)).metadata.resourceVersion).toBe(
      live.metadata.resourceVersion,
    );
  });

  it("with the choice, the field has the new value and the fleet manager owns it", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await createClientSideApplied(api, made);
    const live = await read(made);
    const change = updateOf(
      "Rename",
      live,
      environment(made.metadata.name, { displayName: "Prod" }),
    );
    const chosen = withTakeOver(
      change,
      (await previewEnvironmentChange(change)).ownership ?? [],
    );

    const preview = await previewEnvironmentChange(chosen);

    expect(preview.errors).toEqual([]);
    expect(preview.objects[0].diff).toContainEqual(
      expect.objectContaining({
        op: "added",
        text: expect.stringContaining("displayName: Prod"),
      }),
    );
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });
    const after = await read(made);
    expect(after.spec.displayName).toBe("Prod");
    expect(fieldManagers(after as ManagedObject)["spec.displayName"]).toBe(
      APP_FIELD_MANAGER,
    );
    expect(
      owns(after, "kubectl-client-side-apply", ["f:spec", "f:displayName"]),
    ).toBe(false);
  });

  it("a field that the fleet manager shares with kubectl conflicts on a change and is offered", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await applyAs("kubectl", made);
    await applyAs(APP_FIELD_MANAGER, made);
    const change = updateOf(
      "Rename",
      await read(made),
      environment(made.metadata.name, { displayName: "Prod" }),
    );

    const preview = await previewEnvironmentChange(change);

    expect(preview.ownership?.[0].changed).toEqual([
      expect.objectContaining({
        path: "spec.displayName",
        managers: [
          { name: "kubectl", operation: "Apply", kind: "other", known: true },
        ],
      }),
    ]);
    expect(preview.errors).toEqual([
      expect.objectContaining({ code: "Conflict", manager: "kubectl" }),
    ]);
    expect(
      await applyEnvironmentChange(
        withTakeOver(change, preview.ownership ?? []),
      ),
    ).toEqual({ ok: true, mode: "direct" });
  });

  it("after a take-over, kubectl apply --server-side of the old file conflicts", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await applyAs("kubectl", made);
    const live = await read(made);
    const change = updateOf(
      "Rename",
      live,
      environment(made.metadata.name, { displayName: "Prod" }),
    );
    const chosen = withTakeOver(
      change,
      (await previewEnvironmentChange(change)).ownership ?? [],
    );
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });

    await expect(applyAs("kubectl", made)).rejects.toThrow(
      /conflict with \\+"fleet-manager\\+"/,
    );
  });

  it("a cleared field, map entry, and list member stay without the choice and go with it", async () => {
    const name = unique("env");
    await createClientSideApplied(
      api,
      environment(name, {
        displayName: "Stage",
        sections: {
          gone: { allowed: ["echo"], required: true },
          kept: { required: false },
        },
        presets: { camundaCluster: ["a", "b"] },
      }),
    );
    const object = environment(name, {
      sections: { kept: { required: false } },
      presets: { camundaCluster: ["a"] },
    });
    const cleared = [
      "spec.displayName",
      'spec.presets.camundaCluster[="b"]',
      "spec.sections.gone",
    ];

    const plain = updateOf("Clear", await read(object), object);
    const shown = await previewEnvironmentChange(plain);
    expect(shown.errors).toEqual([]);
    expect(clearedPaths(shown)).toEqual(cleared);
    expect(await applyEnvironmentChange(plain)).toEqual({
      ok: true,
      mode: "direct",
    });
    const kept = await read(object);
    expect(kept.spec).toMatchObject({
      displayName: "Stage",
      sections: { gone: { allowed: ["echo"], required: true } },
      presets: { camundaCluster: ["a", "b"] },
    });

    const again = updateOf("Clear", kept, object);
    const chosen = withTakeOver(
      again,
      (await previewEnvironmentChange(again)).ownership ?? [],
    );
    expect(chosen.remove).toEqual({ [keyOf(object)]: cleared });
    const preview = await previewEnvironmentChange(chosen);
    expect(preview.errors).toEqual([]);
    const removed = preview.objects[0].diff
      .filter((line) => line.op === "removed")
      .map((line) => line.text);
    expect(removed).toEqual(
      expect.arrayContaining([
        expect.stringContaining("displayName: Stage"),
        expect.stringContaining("gone:"),
        expect.stringContaining("- b"),
      ]),
    );
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });
    const after = await read(object);
    expect(after.spec).toEqual({
      ...after.spec,
      sections: { kept: { required: false } },
      presets: { camundaCluster: ["a"] },
    });
    expect(after.spec).not.toHaveProperty("displayName");
    expect(owns(after, APP_FIELD_MANAGER, [], "Update")).toBe(false);
  });

  it("removes a field inside a kept list item", async () => {
    // Every field of a parameter choice of an Environment is required, so
    // the optional value of an extraEnv entry of a claim is the case.
    const name = unique("orders");
    const spec = (entry: object) => ({
      presetRef: "medium",
      attachments: {
        optimize: {
          managementAuthConfigRef: "auth",
          importer: { extraEnv: [entry] },
        },
      },
    });
    await createClientSideApplied(
      api,
      clusterClaim(name, spec({ name: "TZ", value: "UTC" })),
    );
    const object = clusterClaim(name, spec({ name: "TZ" }));
    const change = updateOf("Clear TZ", await read(object), object);
    const shown = await previewClaimChange(change);
    expect(clearedPaths(shown)).toEqual([
      "spec.attachments.optimize.importer.extraEnv[name=TZ].value",
    ]);

    expect(
      await applyClaimChange(withTakeOver(change, shown.ownership ?? [])),
    ).toEqual({ ok: true, mode: "direct" });

    const after = await read(object);
    expect(after.spec.attachments).toEqual({
      optimize: {
        managementAuthConfigRef: "auth",
        importer: { extraEnv: [{ name: "TZ" }] },
      },
    });
    const item = [
      "f:spec",
      "f:attachments",
      "f:optimize",
      "f:importer",
      "f:extraEnv",
      'k:{"name":"TZ"}',
    ];
    expect(owns(after, "kubectl-client-side-apply", item, "Update")).toBe(true);
    expect(
      owns(after, "kubectl-client-side-apply", [...item, "f:name"], "Update"),
    ).toBe(true);
  });

  it("the preview of a removal shows a null inside a value as the save keeps it", async () => {
    const name = unique("env");
    const tier = (extra: object) => ({
      claimEditable: true,
      ...extra,
    });
    await createClientSideApplied(
      api,
      environment(name, {
        displayName: "Stage",
        parameters: {
          "aws-s3": {
            tier: tier({
              allowed: [
                { name: "x", label: "X", value: { a: null } },
                { name: "y", label: "Y", value: 1 },
              ],
            }),
          },
        },
      }),
    );
    const parameters = {
      "aws-s3": {
        tier: tier({
          value: { b: null },
          allowed: [{ name: "x", label: "X", value: { a: null } }],
        }),
      },
    };
    const object = environment(name, { parameters });
    const change = updateOf("Clear", await read(object), object);
    const chosen = withTakeOver(
      change,
      (await previewEnvironmentChange(change)).ownership ?? [],
    );
    expect(chosen.remove).toEqual({
      [keyOf(object)]: [
        "spec.displayName",
        "spec.parameters.aws-s3.tier.allowed[name=y]",
      ],
    });

    const preview = await previewEnvironmentChange(chosen);
    expect(preview.errors).toEqual([]);
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });
    const saved = await read(object);
    expect(preview.objects[0].after?.spec).toEqual(saved.spec);
    expect(saved.spec.parameters).toEqual(parameters);
  });

  it("a field of an operator controller is not offered and a request that lists it writes nothing", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await applyAs(operator, made);
    const live = await read(made);
    const change = updateOf(
      "Rename",
      live,
      environment(made.metadata.name, { displayName: "Prod" }),
    );

    const preview = await previewEnvironmentChange(change);
    const field = preview.ownership![0].changed[0];
    expect(field.managers).toEqual([
      { name: operator, operation: "Apply", kind: "operator", known: true },
    ]);
    expect(offered(field)).toBe(false);
    expect(withTakeOver(change, preview.ownership!)).toBe(change);

    const result = await applyEnvironmentChange({
      ...change,
      takeOver: { [keyOf(made)]: ["spec.displayName"] },
    });
    expect(result).toEqual({
      ok: false,
      errors: [
        expect.objectContaining({
          code: "Conflict",
          path: "spec.displayName",
          manager: operator,
          message: `spec.displayName belongs to ${operator}, a controller of the operator. The fleet manager does not take it over.`,
        }),
      ],
    });
    expect((await read(made)).metadata.resourceVersion).toBe(
      live.metadata.resourceVersion,
    );
  });

  it("a stale resourceVersion fails with Stale with and without a take-over", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await createClientSideApplied(api, made);
    const live = await read(made);
    const change = updateOf(
      "Rename",
      live,
      environment(made.metadata.name, { displayName: "Prod" }),
    );
    const chosen = withTakeOver(
      change,
      (await previewEnvironmentChange(change)).ownership ?? [],
    );
    await update(
      "kubectl-edit",
      environment(made.metadata.name, { color: "#123456" }),
    );
    const moved = (await read(made)).metadata.resourceVersion;

    for (const attempt of [change, chosen]) {
      expect(await applyEnvironmentChange(attempt)).toMatchObject({
        ok: false,
        errors: [{ code: "Stale" }],
      });
    }
    expect((await read(made)).metadata.resourceVersion).toBe(moved);
  });

  it("a final object that a CRD rule refuses fails in the preview and writes nothing", async () => {
    const name = unique("env");
    const allowed = [{ name: "x", label: "X", value: 1 }];
    await createClientSideApplied(
      api,
      environment(name, {
        parameters: { "aws-s3": { tier: { claimEditable: true, allowed } } },
      }),
    );
    const object = environment(name, {
      parameters: { "aws-s3": { tier: { allowed } } },
    });
    const live = await read(object);
    const change = updateOf("Lock the tier", live, object);
    const chosen = withTakeOver(
      change,
      (await previewEnvironmentChange(change)).ownership ?? [],
    );
    expect(chosen.remove).toEqual({
      [keyOf(object)]: ["spec.parameters.aws-s3.tier.claimEditable"],
    });

    const refusal = expect.objectContaining({
      code: "Invalid",
      message: expect.stringContaining("allowed needs claimEditable: true"),
    });
    const preview = await previewEnvironmentChange(chosen);
    expect(preview.objects).toEqual([]);
    expect(preview.errors).toEqual([refusal]);
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: false,
      errors: [refusal],
    });
    expect((await read(object)).metadata.resourceVersion).toBe(
      live.metadata.resourceVersion,
    );
  });

  it("a cluster claim made with kubectl apply moves to a release only with the choice", async () => {
    const made = clusterClaim(unique("orders"), {
      presetRef: "medium",
      releaseRef: "camunda-8-9-1",
    });
    await createClientSideApplied(api, made);
    const live = await read(made);
    const change = updateOf(
      "Move to camunda-8-9-2",
      live,
      clusterClaim(made.metadata.name, {
        presetRef: "medium",
        releaseRef: "camunda-8-9-2",
      }),
    );

    const preview = await previewClaimChange(change);
    expect(changedPaths(preview)).toEqual(["spec.releaseRef"]);
    expect(await applyClaimChange(change)).toMatchObject({
      ok: false,
      errors: [{ code: "Conflict", path: "spec.releaseRef" }],
    });

    expect(
      await applyClaimChange(withTakeOver(change, preview.ownership ?? [])),
    ).toEqual({ ok: true, mode: "direct" });
    expect((await read(made)).spec.releaseRef).toBe("camunda-8-9-2");
  });

  it("a move of two claims takes over the field of one and writes both", async () => {
    const byKubectl = clusterClaim(unique("orders"), {
      presetRef: "medium",
      releaseRef: "camunda-8-9-1",
    });
    await createClientSideApplied(api, byKubectl);
    const byApp = clusterClaim(unique("payments"), {
      presetRef: "medium",
      releaseRef: "camunda-8-9-1",
    });
    expect(
      await createCluster(createChangeOf("Create payments", byApp)),
    ).toEqual({ ok: true, mode: "direct" });
    const lives = [await read(byKubectl), await read(byApp)];
    // A move sends each live claim with the new release only.
    const moved = lives.map((live) => {
      const metadata = { ...live.metadata };
      for (const key of [
        "managedFields",
        "resourceVersion",
        "uid",
        "creationTimestamp",
        "generation",
      ])
        delete metadata[key];
      return {
        apiVersion: live.apiVersion,
        kind: live.kind,
        metadata,
        spec: { ...live.spec, releaseRef: "camunda-8-9-2" },
      } as KubeObject;
    });
    const change: Change = {
      intent: "Move 2 clusters to the release camunda-8-9-2",
      apply: moved,
      preconditions: Object.fromEntries(
        lives.map((live) => [keyOf(live), live.metadata.resourceVersion!]),
      ),
    };

    const preview = await previewReleaseMove(change);
    expect(preview.ownership?.map((o) => o.ref.name)).toEqual([
      byKubectl.metadata.name,
    ]);
    const chosen = withTakeOver(change, preview.ownership ?? []);
    expect(Object.keys(chosen.takeOver ?? {})).toEqual([keyOf(byKubectl)]);
    expect(await applyReleaseMove(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });

    const [first, second] = [await read(byKubectl), await read(byApp)];
    expect([first.spec.releaseRef, second.spec.releaseRef]).toEqual([
      "camunda-8-9-2",
      "camunda-8-9-2",
    ]);
    expect(
      ((second.metadata.managedFields ?? []) as ManagedFieldsEntry[]).map(
        (e) => `${e.manager}/${e.operation}`,
      ),
    ).toEqual([`${APP_FIELD_MANAGER}/Apply`]);
  });

  it("the choice is not stored", async () => {
    const made = environment(unique("env"), { displayName: "Stage" });
    await createClientSideApplied(api, made);
    const first = updateOf(
      "Rename",
      await read(made),
      environment(made.metadata.name, { displayName: "Prod" }),
    );
    const chosen = withTakeOver(
      first,
      (await previewEnvironmentChange(first)).ownership ?? [],
    );
    expect(await applyEnvironmentChange(chosen)).toEqual({
      ok: true,
      mode: "direct",
    });
    await update(
      "kubectl-client-side-apply",
      environment(made.metadata.name, { displayName: "Stage" }),
    );

    const next = updateOf(
      "Rename",
      await read(made),
      environment(made.metadata.name, { displayName: "Final" }),
    );
    const preview = await previewEnvironmentChange(next);
    expect(changedPaths(preview)).toEqual(["spec.displayName"]);
    expect(preview.errors).toEqual([
      expect.objectContaining({ code: "Conflict", path: "spec.displayName" }),
    ]);
  });
});
