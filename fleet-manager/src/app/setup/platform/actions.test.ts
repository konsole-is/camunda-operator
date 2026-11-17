import { beforeEach, describe, expect, it, vi } from "vitest";
import { platformFixtures } from "@/components/setup/platform/fixtures";
import { PLATFORM_SOURCE_KINDS } from "@/components/setup/platform/source-kinds";
import type { PlatformSources } from "@/components/setup/platform/sources";
import type { Change } from "@/lib/write/types";
import { fakeWriter } from "../../../../test/fakes/writer";
import {
  applyPlatformConfigChange,
  previewPlatformConfigChange,
} from "./actions";

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

const configChange = (kind = "CamundaPlatformConfig") => ({
  intent: "Change the platform config oidc-prod",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind,
      metadata: { name: "oidc-prod" },
      spec: { auth: { method: "basic" } },
    },
  ],
  preconditions: { [`${kind}//oidc-prod` as const]: "21" },
});

const deleteWithout = {
  intent: "Delete the platform config basic-lab",
  apply: [],
  delete: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      name: "basic-lab",
    },
  ],
};

beforeEach(() => {
  fakeWriter.reset();
  cache.failing = [];
  const onServer = platformFixtures();
  cache.lists = Object.fromEntries(
    Object.entries(PLATFORM_SOURCE_KINDS).map(([key, kind]) => [
      kind,
      onServer[key as keyof PlatformSources] as unknown[],
    ]),
  );
});

// remove deletes the platform configs named, with a precondition and a UID
// for each.
function remove(...names: string[]): Change {
  return {
    intent: "Delete platform configs",
    apply: [],
    delete: names.map((name) => ({
      apiVersion: "core.camunda.io/v1",
      kind: "CamundaPlatformConfig",
      name,
    })),
    preconditions: Object.fromEntries(
      names.map((name) => [`CamundaPlatformConfig//${name}`, "21"]),
    ),
    uids: Object.fromEntries(
      names.map((name) => [`CamundaPlatformConfig//${name}`, `uid-${name}`]),
    ),
  };
}

const IN_USE =
  "Delete is possible when no cluster, management plane, database server, claim, or environment uses this config.";

describe("the delete guard of applyPlatformConfigChange", () => {
  it("deletes an unused config, with the precondition and the UID of the page", async () => {
    const change = remove("basic-lab");
    expect(await applyPlatformConfigChange(change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it("refuses a delete of a config in use, names its users, and writes nothing", async () => {
    const result = await applyPlatformConfigChange(remove("oidc-prod"));
    expect(!result.ok && result.errors).toMatchObject([
      {
        code: "Forbidden",
        message: expect.stringMatching(
          new RegExp(
            `^${IN_USE} Used by CamundaCluster \\S+, CamundaManagementCluster \\S+, DatabaseServer team-a/orders-db, CamundaClusterClaim team-b/billing, Environment production\\.$`,
          ),
        ),
        ref: { kind: "CamundaPlatformConfig", name: "oidc-prod" },
      },
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of a config that only an environment has as its default", async () => {
    const result = await applyPlatformConfigChange(remove("broken"));
    expect(!result.ok && result.errors.map((e) => e.message)).toEqual([
      `${IN_USE} Used by Environment staging.`,
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of several configs when one is in use, and deletes none of them", async () => {
    const result = await applyPlatformConfigChange(
      remove("basic-lab", "broken"),
    );
    expect(!result.ok && result.errors.map((e) => e.ref?.name)).toEqual([
      "broken",
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete of an unused config while a kind that can use it cannot be read, and names the kind", async () => {
    cache.failing = ["DatabaseServerPreset"];
    const result = await applyPlatformConfigChange(remove("basic-lab"));
    expect(!result.ok && result.errors.map((e) => e.message)).toEqual([
      "Delete is possible when the fleet manager can read DatabaseServerPreset.",
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("deletes an unused config while only a kind that cannot use it fails", async () => {
    cache.failing = ["CamundaRelease"];
    expect((await applyPlatformConfigChange(remove("basic-lab"))).ok).toBe(
      true,
    );
  });

  it("writes an update of a config in use", async () => {
    expect((await applyPlatformConfigChange(configChange())).ok).toBe(true);
  });
});

describe("applyPlatformConfigChange", () => {
  it("writes a CamundaPlatformConfig change through the writer", async () => {
    expect(await applyPlatformConfigChange(configChange())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([configChange()]);
  });

  it("refuses a change of another kind and writes nothing", async () => {
    const result = await applyPlatformConfigChange(configChange("Environment"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("refuses a delete without a precondition", async () => {
    const result = await applyPlatformConfigChange(deleteWithout);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewPlatformConfigChange", () => {
  it("returns the dry run of the writer", async () => {
    await previewPlatformConfigChange(configChange());
    expect(fakeWriter.previewed).toEqual([configChange()]);
  });

  it("refuses a change of another kind", async () => {
    const result = await previewPlatformConfigChange(
      configChange("Environment"),
    );
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });

  it("refuses a delete of a config in use as the write does, and previews nothing", async () => {
    const preview = await previewPlatformConfigChange(remove("broken"));
    expect(preview.objects).toEqual([]);
    expect(preview.errors.map((e) => [e.code, e.message])).toEqual([
      ["Forbidden", `${IN_USE} Used by Environment staging.`],
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });

  it("refuses a delete while a kind that can use the config cannot be read", async () => {
    cache.failing = ["Environment"];
    const preview = await previewPlatformConfigChange(remove("basic-lab"));
    expect(preview.errors.map((e) => e.message)).toEqual([
      "Delete is possible when the fleet manager can read Environment.",
    ]);
    expect(fakeWriter.previewed).toEqual([]);
  });
});
