import type { V1Namespace } from "@kubernetes/client-node";
import { describe, expect, it } from "vitest";
import { platformFixtures } from "@/components/setup/platform/fixtures";
import { presetFixtures } from "@/components/setup/presets/fixtures";
import { releaseFixtures } from "@/components/setup/releases/fixtures";
import type { Environment } from "@/generated/crds/environment";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import {
  deleteRefusalText,
  PLATFORM_IN_USE_TEXT,
  platformDeleteBlock,
  presetDeleteChecks,
  RELEASE_IN_USE_TEXT,
  releaseDeleteBlock,
} from "./delete-guard";
import type { PresetUsageSources } from "./presets";

const PRESET_IN_USE =
  "Delete is possible when no resource, claim, or request uses the preset.";

function presetSources(): PresetUsageSources {
  const sources = presetFixtures();
  return {
    ...sources,
    environments: sources.environments as unknown as Environment[],
    namespaces: sources.namespaces as unknown as V1Namespace[],
    registrations: sources.registrations as unknown as PluginRegistration[],
  };
}

describe("deleteRefusalText", () => {
  it("names each user after the reason", () => {
    expect(
      deleteRefusalText({
        reason: "Delete is possible when nothing uses it.",
        users: ["CamundaCluster team-a/orders", "Environment production"],
      }),
    ).toBe(
      "Delete is possible when nothing uses it. Used by CamundaCluster team-a/orders, Environment production.",
    );
  });

  it("gives only the reason when no user blocks the delete", () => {
    expect(
      deleteRefusalText({
        reason: "Delete is possible when the fleet manager can read Namespace.",
        users: [],
      }),
    ).toBe("Delete is possible when the fleet manager can read Namespace.");
  });
});

describe("presetDeleteChecks", () => {
  it("does not block a preset that nothing uses and no environment allows", () => {
    const check = presetDeleteChecks(presetSources(), [])(
      "CamundaClusterPreset",
      "spare",
    );
    expect(check).toEqual({ users: [], allowed: [], unread: [] });
  });

  it("blocks a preset in use and names its users and the environments that allow it", () => {
    const check = presetDeleteChecks(presetSources(), [])(
      "CamundaClusterPreset",
      "medium",
    );
    expect(check.block).toEqual({
      reason: PRESET_IN_USE,
      users: [
        "CamundaCluster team-orders/orders",
        "CamundaClusterClaim team-orders/orders",
        "Environment production",
      ],
    });
  });

  it("blocks a preset that only a claim selection names", () => {
    const check = presetDeleteChecks(presetSources(), [])(
      "DatabaseServerPreset",
      "small",
    );
    expect(check.block).toEqual({
      reason: PRESET_IN_USE,
      users: ["CamundaManagementClusterClaim platform/plane"],
    });
  });

  describe("with a claim whose recorded policy alone names the preset", () => {
    const blockOf = (
      plugins: {
        section: string;
        registration: string;
        parameters?: unknown;
      }[],
    ) => {
      const sources = presetSources();
      sources.clusterClaims = [
        {
          metadata: { namespace: "team-orders", name: "search" },
          spec: { plugins },
          status: {
            resolvedPolicy: {
              deletionPolicy: "Delete",
              parameters: { elasticsearch: { preset: { value: "recorded" } } },
            },
          },
        },
      ];
      return presetDeleteChecks(sources, [])(
        "ElasticsearchClusterPreset",
        "recorded",
      ).block;
    };
    const refused = {
      reason: PRESET_IN_USE,
      users: ["CamundaClusterClaim team-orders/search"],
    };

    it("blocks the delete while another selection is not a JSON object", () => {
      expect(
        blockOf([
          { section: "secondary-storage", registration: "elasticsearch" },
          {
            section: "backup-storage",
            registration: "existing-object-storage",
            parameters: "x",
          },
        ]),
      ).toEqual(refused);
    });

    it("blocks the delete while the selection of the preset is not a JSON object", () => {
      expect(
        blockOf([
          {
            section: "secondary-storage",
            registration: "elasticsearch",
            parameters: "x",
          },
        ]),
      ).toEqual(refused);
    });
  });

  it("blocks a preset that only an environment allows", () => {
    const sources = presetSources();
    sources.clusters = [];
    sources.clusterClaims = [];
    const check = presetDeleteChecks(sources, [])(
      "CamundaClusterPreset",
      "medium",
    );
    expect(check.block).toEqual({
      reason: "Delete is possible when no environment allows the preset.",
      users: ["Environment production"],
    });
  });

  it("blocks an unused preset while a list that can name it is not read, and names only those lists", () => {
    const check = presetDeleteChecks(presetSources(), [
      "PluginRequest",
      "CamundaClusterClaim",
    ])("CamundaClusterPreset", "spare");
    expect(check.unread).toEqual(["CamundaClusterClaim"]);
    expect(check.block).toEqual({
      reason:
        "Delete is possible when the fleet manager can read CamundaClusterClaim.",
      users: [],
    });
  });

  it("does not block a preset for a list that cannot name it", () => {
    const check = presetDeleteChecks(presetSources(), ["PluginRequest"])(
      "CamundaClusterPreset",
      "spare",
    );
    expect(check.block).toBeUndefined();
  });

  it("blocks an unused preset while a list that can name it is not current", () => {
    const check = presetDeleteChecks(
      presetSources(),
      [],
      ["CamundaCluster"],
    )("CamundaClusterPreset", "spare");
    expect(check.block).toEqual({
      reason:
        "Delete is possible when the fleet manager has the current lists.",
      users: [],
    });
  });

  it("names no user while a list that can name the preset is not current", () => {
    const check = presetDeleteChecks(
      presetSources(),
      [],
      ["CamundaCluster"],
    )("CamundaClusterPreset", "medium");
    expect(check.block).toEqual({
      reason:
        "Delete is possible when the fleet manager has the current lists.",
      users: [],
    });
  });
});

describe("releaseDeleteBlock", () => {
  it("does not block a release that nothing uses and no environment allows", () => {
    expect(
      releaseDeleteBlock("camunda-8-9-1", releaseFixtures(), []),
    ).toBeUndefined();
  });

  it("blocks a release in use and names its users and environments once", () => {
    expect(releaseDeleteBlock("camunda-8-9-0", releaseFixtures(), [])).toEqual({
      reason: RELEASE_IN_USE_TEXT,
      users: [
        "CamundaCluster team-orders/orders",
        "ElasticsearchCluster team-orders/orders-search",
        "CamundaClusterClaim team-orders/orders",
        "Environment production",
        "Environment staging",
      ],
    });
  });

  it("blocks a release that only a claim names", () => {
    expect(
      releaseDeleteBlock("camunda-8-10-0", releaseFixtures(), [])?.users,
    ).toEqual(["CamundaClusterClaim team-new/billing"]);
  });

  it("blocks a release that only an environment allows", () => {
    expect(
      releaseDeleteBlock("camunda-8-8-5", releaseFixtures(), [])?.users,
    ).toEqual(["Environment staging"]);
  });

  it("blocks a release that an environment has as its default only", () => {
    const sources = releaseFixtures();
    sources.environments[0].spec.releases = { default: "camunda-8-9-1" };
    expect(releaseDeleteBlock("camunda-8-9-1", sources, [])?.users).toEqual([
      "Environment production",
    ]);
  });

  it("blocks an unused release while a list is not read, and names the list", () => {
    expect(
      releaseDeleteBlock("camunda-8-9-1", releaseFixtures(), ["Namespace"]),
    ).toEqual({
      reason: "Delete is possible when the fleet manager can read Namespace.",
      users: [],
    });
  });
});

describe("platformDeleteBlock", () => {
  it("does not block a config that nothing uses", () => {
    expect(
      platformDeleteBlock("basic-lab", platformFixtures(), []),
    ).toBeUndefined();
  });

  it("blocks a config in use and names each user and the environment whose default it is", () => {
    const block = platformDeleteBlock("oidc-prod", platformFixtures(), []);
    expect(block?.reason).toBe(PLATFORM_IN_USE_TEXT);
    expect(block?.users.map((u) => u.split(" ")[0])).toEqual([
      "CamundaCluster",
      "CamundaManagementCluster",
      "DatabaseServer",
      "CamundaClusterClaim",
      "Environment",
    ]);
    expect(block?.users).toContain("Environment production");
  });

  it("blocks a config that only an environment has as its default", () => {
    expect(
      platformDeleteBlock("broken", platformFixtures(), [])?.users,
    ).toEqual(["Environment staging"]);
  });

  it("blocks a config that only a management plane claim names", () => {
    const sources = platformFixtures();
    sources.planeClaims = [
      {
        apiVersion: "fleet.camunda.io/v1alpha1",
        kind: "CamundaManagementClusterClaim",
        metadata: { namespace: "platform", name: "lab" },
        spec: { platformConfigRef: "basic-lab" },
      } as (typeof sources.planeClaims)[number],
    ];
    expect(platformDeleteBlock("basic-lab", sources, [])?.users).toEqual([
      "CamundaManagementClusterClaim platform/lab",
    ]);
  });

  it("blocks an unused config while a kind that can use it is not read, and names only those kinds", () => {
    expect(
      platformDeleteBlock("basic-lab", platformFixtures(), [
        "CamundaRelease",
        "DatabaseServerPreset",
      ]),
    ).toEqual({
      reason:
        "Delete is possible when the fleet manager can read DatabaseServerPreset.",
      users: [],
    });
  });

  it("does not block a config for a kind that cannot use it", () => {
    expect(
      platformDeleteBlock("basic-lab", platformFixtures(), ["CamundaRelease"]),
    ).toBeUndefined();
  });
});
