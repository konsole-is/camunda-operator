import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { REFUSED_CLUSTER_FIELDS, refusedSteps } from "./claim";
import { pathFrom } from "./walk";

const CEL_ORDER = [
  "presetRef",
  "releaseRef",
  "platformConfigRef",
  "suspend",
  "version",
  "connectors.version",
  "serviceAccount.name",
  "connectors.serviceAccount.name",
  "storageRef",
  "backupStorageRef",
];

describe("REFUSED_CLUSTER_FIELDS", () => {
  it("lists the 10 refused fields in the order of the CEL rule", () => {
    expect(REFUSED_CLUSTER_FIELDS.map((r) => r.names.join("."))).toEqual(
      CEL_ORDER,
    );
  });

  // The CEL rule of the Go types is the source. A change on one side alone
  // fails here.
  it("matches the CEL message of the Go types", () => {
    const source = readFileSync(
      new URL(
        "../../../../api/fleet/v1alpha1/camundaclusterclaim_types.go",
        import.meta.url,
      ),
      "utf8",
    );
    const list = /message="spec\.cluster must not set ([^:]*):/.exec(
      source,
    )?.[1];
    expect(list).toBeDefined();
    expect(list!.replace(", or ", ", ").split(", ")).toEqual(
      REFUSED_CLUSTER_FIELDS.map((r) => r.names.join(".")),
    );
  });

  it("names what sets each field instead", () => {
    expect(
      Object.fromEntries(
        REFUSED_CLUSTER_FIELDS.map((r) => [r.names.join("."), r.setBy]),
      ),
    ).toEqual({
      presetRef: "claim",
      releaseRef: "claim",
      platformConfigRef: "claim",
      suspend: "claim",
      version: "release",
      "connectors.version": "release",
      "serviceAccount.name": "claimController",
      "connectors.serviceAccount.name": "claimController",
      storageRef: "section",
      backupStorageRef: "section",
    });
  });
});

describe("refusedSteps", () => {
  it("starts at spec.cluster", () => {
    const row = REFUSED_CLUSTER_FIELDS.find(
      (r) => r.names.join(".") === "connectors.serviceAccount.name",
    )!;
    expect(pathFrom("", refusedSteps(row))).toBe(
      "spec.cluster.connectors.serviceAccount.name",
    );
  });
});
