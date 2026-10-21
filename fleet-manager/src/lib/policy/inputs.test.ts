import { describe, expect, it } from "vitest";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { missingInputs } from "./inputs";

const esRegistration = {
  metadata: { name: "elasticsearch" },
  spec: {
    sections: ["secondary-storage"],
    inputs: [
      {
        name: "objectStorageConfig",
        from: "backup-storage/objectStorageConfig",
        required: true,
      },
      {
        name: "storageClassName",
        from: "volume-encryption/storageClassName",
      },
    ],
  },
} as PluginRegistration;

const es = { section: "secondary-storage", registration: "elasticsearch" };

describe("missingInputs", () => {
  it("returns a required input whose section has no selection", () => {
    expect(missingInputs(es, [es], [esRegistration])).toEqual([
      {
        name: "objectStorageConfig",
        from: "backup-storage/objectStorageConfig",
        section: "backup-storage",
        output: "objectStorageConfig",
      },
    ]);
  });

  it("returns nothing when the producing section has a selection, also before it has outputs", () => {
    expect(
      missingInputs(
        es,
        [es, { section: "backup-storage", registration: "s3" }],
        [esRegistration],
      ),
    ).toEqual([]);
  });

  it("skips an optional input whose section has no selection", () => {
    const optional = structuredClone(esRegistration);
    optional.spec.inputs![0].required = false;

    expect(missingInputs(es, [es], [optional])).toEqual([]);
  });

  it("returns nothing for a registration that does not exist", () => {
    expect(missingInputs(es, [es], [])).toEqual([]);
  });
});
