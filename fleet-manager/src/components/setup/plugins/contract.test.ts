import { describe, expect, it } from "vitest";
import { contractOf } from "./contract";
import { sectionFields } from "./groups";
import {
  pluginRegistration,
  pluginSection,
  registrationSources,
} from "./fixtures";

const sources = registrationSources();
const es = sources.registrations.find(
  (r) => r.metadata.name === "elasticsearch",
)!;

describe("contractOf", () => {
  it("shows the declared output of the built-in elasticsearch registration", () => {
    const contract = contractOf(es, sources.sections, []);
    expect(contract.outputs).toEqual([
      {
        key: "secondaryStorageConfig",
        description:
          "The name of the SecondaryStorageConfig that the cluster reads.",
      },
    ]);
    expect(contract.inputs).toContainEqual({
      name: "objectStorageConfig",
      from: "backup-storage/objectStorageConfig",
      fromLabel: "Backup storage / objectStorageConfig",
      formats: [],
      required: false,
    });
    const section = sources.sections.find(
      (s) => s.metadata.name === "secondary-storage",
    )!;
    expect(contract.sections).toEqual([
      {
        name: "secondary-storage",
        label: "Secondary storage",
        fields: sectionFields(section),
      },
    ]);
  });

  it("gives the formats and the required flag of an input", () => {
    const contract = contractOf(
      pluginRegistration("aws-s3", {
        inputs: [
          {
            name: "kmsKeyArn",
            from: "encryption-key/keyArn",
            formats: ["aws-kms-arn"],
            required: true,
          },
        ],
      }),
      sources.sections,
      [],
    );
    expect(contract.inputs).toEqual([
      {
        name: "kmsKeyArn",
        from: "encryption-key/keyArn",
        fromLabel: "Encryption key / keyArn",
        formats: ["aws-kms-arn"],
        required: true,
      },
    ]);
  });

  it("gives no label for a section or an input source that does not exist", () => {
    const contract = contractOf(
      pluginRegistration("x", {
        sections: ["gone"],
        inputs: [{ name: "a", from: "gone/a" }],
      }),
      [pluginSection("other", {})],
      [],
    );
    expect(contract.sections).toEqual([{ name: "gone", fields: [] }]);
    expect(contract.inputs).toEqual([
      { name: "a", from: "gone/a", formats: [], required: false },
    ]);
  });

  it("has only Delete when the registration does not support Orphan", () => {
    const contract = contractOf(
      pluginRegistration("x", { deletionPolicies: ["Delete"] }),
      [],
      [],
    );
    expect(contract.deletionPolicies).toEqual(["Delete"]);
  });

  it("drops a deletion policy that is not Delete or Orphan", () => {
    const contract = contractOf(
      pluginRegistration("x", {
        deletionPolicies: ["Delete", "Retain"] as never,
      }),
      [],
      [],
    );
    expect(contract.deletionPolicies).toEqual(["Delete"]);
  });

  it("gives an error for deletion policies that are not a list", () => {
    const contract = contractOf(
      pluginRegistration("x", { deletionPolicies: "Delete" as never }),
      [],
      [],
    );
    expect(contract.deletionPolicies).toEqual({
      error: "The deletion policies are not a list.",
    });
  });

  it("knows no section while PluginSection is not read", () => {
    const contract = contractOf(es, [], ["PluginSection"]);
    expect(contract.sectionsUnknown).toBe(true);
  });

  it("gives an error for an input or an output that is not valid", () => {
    const contract = contractOf(
      pluginRegistration("x", {
        inputs: [{ name: "a", from: 3 }] as never,
        outputs: [{ key: "b" }, { description: "no key" }] as never,
      }),
      [],
      [],
    );
    expect(contract.inputs).toEqual([
      { error: "Input 1 is not a valid input." },
    ]);
    expect(contract.outputs).toEqual([
      { key: "b" },
      { error: "Output 2 is not a valid output." },
    ]);
  });

  it("gives an error for inputs that are not a list", () => {
    const contract = contractOf(
      pluginRegistration("x", { inputs: { name: "a" } as never }),
      [],
      [],
    );
    expect(contract.inputs).toEqual([{ error: "The inputs are not a list." }]);
  });
});
