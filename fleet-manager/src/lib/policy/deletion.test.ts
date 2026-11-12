import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import { deletionChoice } from "./deletion";

const environment = {
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "Environment",
  metadata: { name: "prod" },
  spec: { deletionPolicy: "Orphan" },
} as Environment;

const keeps = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name: "keeps" },
  spec: {
    sections: ["storage"],
    mode: "Optional",
    label: "Keeps",
    deletionPolicies: ["Delete", "Orphan"],
    serviceAccount: { name: "keeps", namespace: "plugins" },
  },
} as PluginRegistration;

const selections = [{ section: "storage", registration: "keeps" }];

describe("deletionChoice", () => {
  it("takes the default of the Environment without a given default", () => {
    expect(
      deletionChoice(undefined, selections, environment, [keeps]),
    ).toMatchObject({ defaultPolicy: "Orphan", policy: "Orphan" });
  });

  it("takes a given default over the default of the Environment", () => {
    expect(
      deletionChoice(undefined, selections, environment, [keeps], [], "Delete"),
    ).toMatchObject({ defaultPolicy: "Delete", policy: "Delete" });
  });
});
