import { beforeEach, describe, expect, it, vi } from "vitest";
import { presetDeleteChange } from "@/components/setup/presets/delete";
import { presetFixtures } from "@/components/setup/presets/fixtures";
import { fakeWriter } from "../../../../test/fakes/writer";
import { applyPresetChange, previewPresetChange } from "./actions";

vi.mock("@/lib/write/writer", () => import("../../../../test/fakes/writer"));

const sources = presetFixtures();
const spare = sources.clusterPresets[1];
const esStandard = sources.elasticsearchPresets[0];

beforeEach(() => fakeWriter.reset());

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

describe("previewPresetChange", () => {
  it("previewPresetChange previews through the writer", async () => {
    const change = presetDeleteChange([spare]);
    await previewPresetChange(change);
    expect(fakeWriter.previewed).toEqual([change]);
  });
});
