import { randomUUID } from "node:crypto";
import { KubeConfig, KubernetesObjectApi } from "@kubernetes/client-node";
import { beforeAll, describe, expect, inject, it } from "vitest";
import {
  presetDraftOf,
  presetUpdateChange,
} from "@/components/setup/presets/draft";
import type { PresetObject } from "@/components/setup/presets/kinds";
import { withTakeOver } from "@/lib/write/ownership";
import type { KubeObject } from "@/lib/write/types";
import { directWriter, readObject } from "@/lib/write/writer";

const kc = new KubeConfig();
kc.loadFromFile(inject("kubeconfig"));
const api = KubernetesObjectApi.makeApiClient(kc);
const writer = directWriter(kc);

// The editor of an Elasticsearch or PostgreSQL preset has no schema of its
// own: the dry run of the review is the check against the CRD.
describe.each([
  {
    kind: "ElasticsearchClusterPreset",
    body: "cluster",
    spec: { cluster: { replicas: 3, storageSize: "64Gi" } },
    typo: { replicas: 3, storageSize: "64Gi", storgeClassName: "fast" },
  },
  {
    kind: "DatabaseServerPreset",
    body: "server",
    spec: { server: { instances: 1, storageSize: "10Gi" } },
    typo: { instances: 1, storageSize: "10Gi", walStorageSzie: "5Gi" },
  },
])("an update of a $kind", ({ kind, body, spec, typo }) => {
  const name = `typo-${randomUUID().slice(0, 8)}`;
  const ref = { apiVersion: "core.camunda.io/v1", kind, name };
  let live: PresetObject;

  beforeAll(async () => {
    await api.create({
      apiVersion: "core.camunda.io/v1",
      kind,
      metadata: { name },
      spec,
    } as KubeObject);
    live = (await readObject(kc, ref)) as unknown as PresetObject;
  });

  it("refuses a misspelled field in the preview and never writes it", async () => {
    const change = presetUpdateChange(live, {
      ...presetDraftOf(live),
      body: typo,
    });

    const preview = await writer.preview(change);
    expect(preview.errors).not.toEqual([]);
    expect(
      preview.errors.map((e) => `${e.path ?? ""} ${e.message}`).join("\n"),
    ).toMatch(Object.keys(typo).at(-1) ?? "");

    const result = await writer.apply(change);
    expect(result.ok).toBe(false);
    const after = await readObject(kc, ref);
    expect(after?.spec).toEqual(spec);
    expect(after?.metadata.resourceVersion).toBe(live.metadata.resourceVersion);
  });

  // The test created the preset under another field manager, so the save
  // takes the storage size over, as the review does after the choice.
  it("writes a body that the CRD knows", async () => {
    const change = presetUpdateChange(live, {
      ...presetDraftOf(live),
      body: {
        ...(spec as unknown as Record<string, Record<string, unknown>>)[body],
        storageSize: "80Gi",
      },
    });
    const preview = await writer.preview(change);
    expect(
      (preview.ownership ?? []).flatMap((o) => o.changed.map((f) => f.path)),
    ).toEqual([`spec.${body}.storageSize`]);
    const taken = withTakeOver(change, preview.ownership ?? []);
    expect((await writer.preview(taken)).errors).toEqual([]);
    expect((await writer.apply(taken)).ok).toBe(true);
    const after = await readObject(kc, ref);
    expect(
      (after?.spec as Record<string, Record<string, unknown>>)[body]
        .storageSize,
    ).toBe("80Gi");
  });
});
