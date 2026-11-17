import { beforeEach, describe, expect, it, vi } from "vitest";
import { presetDeleteChange } from "@/components/setup/presets/delete";
import { presetFixtures } from "@/components/setup/presets/fixtures";
import { PRESET_SOURCE_KINDS } from "@/components/setup/presets/source-kinds";
import type { PresetSources } from "@/components/setup/presets/sources";
import type { Change, WriteResult } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import { applyPresetChange, previewPresetChange } from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const cache = vi.hoisted(() => ({
  lists: {} as Record<string, unknown[]>,
  failing: [] as string[],
}));
vi.mock("@/lib/k8s/read", () => ({
  list: async (kind: string) => {
    if (cache.failing.includes(kind)) throw new Error(`${kind} is forbidden`);
    return cache.lists[kind] ?? [];
  },
}));

const sources = presetFixtures();
const spare = sources.clusterPresets[1];
const esStandard = sources.elasticsearchPresets[0];

beforeEach(() => {
  fakeWriter.reset();
  cache.failing = [];
  const onServer = presetFixtures();
  cache.lists = Object.fromEntries(
    Object.entries(PRESET_SOURCE_KINDS).map(([key, kind]) => [
      kind,
      onServer[key as keyof PresetSources] as unknown[],
    ]),
  );
});

// remove deletes the presets of each [kind, name], with a precondition
// and a UID for each.
function remove(...presets: [string, string][]): Change {
  return {
    intent: "Delete presets",
    apply: [],
    delete: presets.map(([kind, name]) => ({
      apiVersion: "core.camunda.io/v1",
      kind,
      name,
    })),
    preconditions: Object.fromEntries(
      presets.map(([kind, name]) => [`${kind}//${name}`, "10"]),
    ),
    uids: Object.fromEntries(
      presets.map(([kind, name]) => [`${kind}//${name}`, `uid-${name}`]),
    ),
  };
}

function refusals(result: WriteResult): { code: string; message: string }[] {
  return result.ok
    ? []
    : result.errors.map((e) => ({ code: e.code, message: e.message }));
}

const MEDIUM_IN_USE =
  "Delete is possible when no resource, claim, or request uses the preset. Used by CamundaCluster team-orders/orders, CamundaClusterClaim team-orders/orders, Environment production.";

describe("applyPresetChange", () => {
  it("applyPresetChange writes a delete of presets of two kinds through the writer", async () => {
    const change = presetDeleteChange([spare, esStandard]);
    expect(await applyPresetChange(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("applyPresetChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyPresetChange({
      intent: "Delete the release x",
      apply: [],
      delete: [
        { apiVersion: "core.camunda.io/v1", kind: "CamundaRelease", name: "x" },
      ],
      preconditions: { "CamundaRelease//x": "1" },
    });
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyPresetChange refuses a delete without a precondition", async () => {
    const result = await applyPresetChange({
      ...presetDeleteChange([spare]),
      preconditions: {},
    });
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("the delete guard of applyPresetChange", () => {
  it("refuses a delete of a preset in use, names its users, and writes nothing", async () => {
    const result = await applyPresetChange(
      remove(["CamundaClusterPreset", "medium"]),
    );
    expect(refusals(result)).toEqual([
      { code: "Forbidden", message: MEDIUM_IN_USE },
    ]);
    expect(!result.ok && result.errors[0].ref).toEqual({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaClusterPreset",
      name: "medium",
    });
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of a preset that only a claim selection names", async () => {
    const result = await applyPresetChange(
      remove(["DatabaseServerPreset", "small"]),
    );
    expect(refusals(result)).toEqual([
      {
        code: "Forbidden",
        message:
          "Delete is possible when no resource, claim, or request uses the preset. Used by CamundaManagementClusterClaim platform/plane.",
      },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a bulk delete when one preset is in use, and deletes none of them", async () => {
    const result = await applyPresetChange(
      remove(
        ["CamundaClusterPreset", "spare"],
        ["CamundaClusterPreset", "medium"],
        ["ElasticsearchClusterPreset", "standard"],
      ),
    );
    expect(refusals(result)).toEqual([
      { code: "Forbidden", message: MEDIUM_IN_USE },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of an unused preset while a list that can name it cannot be read, and names the list", async () => {
    cache.failing = ["CamundaClusterClaim"];
    const result = await applyPresetChange(
      remove(["CamundaClusterPreset", "spare"]),
    );
    expect(refusals(result)).toEqual([
      {
        code: "Forbidden",
        message:
          "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("deletes an unused preset while only a list that cannot name it fails", async () => {
    cache.failing = ["PluginRequest"];
    const change = remove(["CamundaClusterPreset", "spare"]);
    expect((await applyPresetChange(change)).ok).toBe(true);
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("passes the precondition and the UID of the page to the writer", async () => {
    const change = remove(["DatabaseServerPreset", "standard"]);
    await applyPresetChange(change);
    expect(fakeWriter.applied[0]).toMatchObject({
      preconditions: { "DatabaseServerPreset//standard": "10" },
      uids: { "DatabaseServerPreset//standard": "uid-standard" },
    });
  });

  it("writes an update of a preset in use", async () => {
    const change: Change = {
      intent: "Change the preset medium",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          metadata: { name: "medium" },
          spec: { cluster: {} },
        },
      ],
      preconditions: { "CamundaClusterPreset//medium": "10" },
    };
    expect((await applyPresetChange(change)).ok).toBe(true);
  });
});

describe("previewPresetChange", () => {
  it("previewPresetChange previews through the writer", async () => {
    const change = presetDeleteChange([spare]);
    await previewPresetChange(change);
    expect(fakeWriter.previewed).toEqual([change]);
  });

  it("refuses a delete of a preset in use as the write does, and previews nothing", async () => {
    const preview = await previewPresetChange(
      remove(
        ["CamundaClusterPreset", "spare"],
        ["CamundaClusterPreset", "medium"],
      ),
    );
    expect(preview.objects).toEqual([]);
    expect(
      preview.errors.map((e) => ({ code: e.code, message: e.message })),
    ).toEqual([{ code: "Forbidden", message: MEDIUM_IN_USE }]);
    expect(fakeWriter.previewed).toEqual([]);
  });

  it("refuses a delete while a list that can name the preset cannot be read", async () => {
    cache.failing = ["Environment"];
    const preview = await previewPresetChange(
      remove(["CamundaClusterPreset", "spare"]),
    );
    expect(preview.errors.map((e) => e.message)).toEqual([
      "Delete is possible when the fleet manager can read Environment.",
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });
});
