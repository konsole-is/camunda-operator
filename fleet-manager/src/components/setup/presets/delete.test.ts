import { describe, expect, it } from "vitest";
import { presetDeleteChange } from "./delete";
import { presetFixtures } from "./fixtures";

const sources = presetFixtures();
const spare = sources.clusterPresets[1];
const esStandard = sources.elasticsearchPresets[0];

describe("presetDeleteChange", () => {
  it("deletes each preset with its resourceVersion and UID", () => {
    expect(presetDeleteChange([spare, esStandard])).toEqual({
      intent: "Delete 2 presets",
      apply: [],
      delete: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "CamundaClusterPreset",
          name: "spare",
        },
        {
          apiVersion: "core.camunda.io/v1",
          kind: "ElasticsearchClusterPreset",
          name: "standard",
        },
      ],
      preconditions: {
        "CamundaClusterPreset//spare": "11",
        "ElasticsearchClusterPreset//standard": "12",
      },
      uids: {
        "CamundaClusterPreset//spare": "uid-spare",
        "ElasticsearchClusterPreset//standard": "uid-es",
      },
    });
  });

  it("names one preset in the intent", () => {
    expect(presetDeleteChange([spare]).intent).toBe("Delete the preset spare");
  });

  it("keeps two presets of one name and two kinds apart", () => {
    const change = presetDeleteChange([esStandard, sources.databasePresets[1]]);
    expect(Object.keys(change.preconditions ?? {})).toEqual([
      "ElasticsearchClusterPreset//standard",
      "DatabaseServerPreset//standard",
    ]);
  });
});
