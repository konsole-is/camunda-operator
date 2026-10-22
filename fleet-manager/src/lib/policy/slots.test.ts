import { describe, expect, it } from "vitest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { slotFields, slotOf } from "./slots";

const section = (name: string, slot?: PluginSection["spec"]["slot"]) =>
  ({
    metadata: { name },
    spec: {
      appliesTo: ["CamundaManagementClusterClaim"],
      label: name,
      selection: "One",
      slot,
    },
  }) as PluginSection;

describe("slotOf", () => {
  it("returns the slot of a section and undefined for a section without one", () => {
    expect(slotOf(section("access", "externalAccess"))).toBe("externalAccess");
    expect(slotOf(section("monitoring"))).toBeUndefined();
  });
});

describe("slotFields", () => {
  it("fills the three database references of a plane from the managementDatabases slot", () => {
    const got = slotFields(
      "CamundaManagementClusterClaim",
      [section("management-databases", "managementDatabases")],
      [{ section: "management-databases", registration: "postgres" }],
      [],
    );

    expect(got).toEqual({
      value: [
        "spec.identity.databaseConfigRef",
        "spec.identityProvider.keycloak.databaseConfigRef",
        "spec.webModeler.databaseConfigRef",
      ].map((path) => ({
        kind: "CamundaManagementCluster",
        path,
        section: "management-databases",
      })),
    });
  });

  it("fills nothing of a cluster claim from a slot of a plane", () => {
    expect(
      slotFields(
        "CamundaClusterClaim",
        [section("management-databases", "managementDatabases")],
        [{ section: "management-databases", registration: "postgres" }],
        [],
      ),
    ).toEqual({ value: [] });
  });
});
