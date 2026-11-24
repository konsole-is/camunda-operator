import { describe, expect, it } from "vitest";
import { presetFixtures } from "./fixtures";
import { presetsOf } from "./kinds";

describe("presetsOf", () => {
  it("returns the list of sources that holds the presets of each kind", () => {
    const sources = presetFixtures();
    expect(presetsOf("CamundaClusterPreset", sources)).toBe(
      sources.clusterPresets,
    );
    expect(presetsOf("ElasticsearchClusterPreset", sources)).toBe(
      sources.elasticsearchPresets,
    );
    expect(presetsOf("DatabaseServerPreset", sources)).toBe(
      sources.databasePresets,
    );
  });
});
