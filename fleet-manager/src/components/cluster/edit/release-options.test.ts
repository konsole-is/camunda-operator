import { describe, expect, it } from "vitest";
import type { Environment } from "@/generated/crds/environment";
import type { Resource } from "@/lib/k8s/kinds";
import { claim, cluster } from "../../../../test/fakes/slot-context";
import {
  createReleaseChoices,
  releaseChoices,
  releaseLabel,
} from "./release-options";

const release = (version: string) =>
  ({
    kind: "CamundaRelease",
    metadata: { name: `camunda-${version.replaceAll(".", "-")}` },
    spec: { version },
  }) as unknown as Resource<"CamundaRelease">;

const releases = ["8.9.1", "8.9.3", "8.10.0", "8.11.0", "8.12.0"].map(release);

const environment = (allowed: string[], fallback = "camunda-8-9-3") =>
  ({
    kind: "Environment",
    metadata: { name: "prod" },
    spec: { releases: { allowed, default: fallback } },
  }) as unknown as Resource<"Environment">;

const ALL = environment([
  "camunda-8-9-1",
  "camunda-8-9-3",
  "camunda-8-10-0",
  "camunda-8-11-0",
]);

function at(
  version: string | undefined,
  spec: object = { releaseRef: "camunda-8-9-3" },
) {
  const c = cluster(spec);
  return {
    ...c,
    status: version ? { management: { version } } : {},
  } as Resource<"CamundaCluster">;
}

const choices = (over: Partial<Parameters<typeof releaseChoices>[0]> = {}) =>
  releaseChoices({
    cluster: at("8.9.3"),
    claim: claim("41", { releaseRef: "camunda-8-9-3" }),
    releases,
    environment: ALL,
    backups: [],
    ...over,
  });

const option = (result: ReturnType<typeof releaseChoices>, name: string) =>
  result.options.find((o) => o.name === name);

describe("releaseChoices", () => {
  it("keeps the options of a cluster with a running backup, because the Claim tab only warns", () => {
    const backups = [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalBackupRDBMS",
        metadata: { namespace: "team-a", name: "nightly" },
        spec: { clusterRef: { name: "orders" } },
        status: { phase: "Running" },
      },
    ] as unknown as Parameters<typeof releaseChoices>[0]["backups"];
    expect(option(choices({ backups }), "camunda-8-9-1")?.downgrade).toEqual({
      from: "8.9.3",
      to: "8.9.1",
    });
    expect(option(choices({ backups }), "camunda-8-11-0")?.disabled).toBe(
      "Skips 8.10. Move to an 8.10 release first.",
    );
  });

  it("gives an allowed downgrade both versions", () => {
    expect(option(choices(), "camunda-8-9-1")).toMatchObject({
      version: "8.9.1",
      downgrade: { from: "8.9.3", to: "8.9.1" },
    });
    expect(option(choices(), "camunda-8-9-1")?.disabled).toBeUndefined();
  });

  it("disables a release that skips a minor, with the reason", () => {
    expect(option(choices(), "camunda-8-11-0")?.disabled).toBe(
      "Skips 8.10. Move to an 8.10 release first.",
    );
    expect(option(choices(), "camunda-8-10-0")?.disabled).toBeUndefined();
  });

  it("leaves out a release that the environment does not allow", () => {
    expect(option(choices(), "camunda-8-12-0")).toBeUndefined();
  });

  it("keeps the current release when the environment no longer allows it", () => {
    const result = choices({
      environment: environment(["camunda-8-10-0"], "camunda-8-10-0"),
    });
    expect(result.options.map((o) => o.name)).toEqual([
      "camunda-8-10-0",
      "camunda-8-9-3",
    ]);
    expect(option(result, "camunda-8-9-3")).toMatchObject({ isCurrent: true });
  });

  it("takes the release of the cluster as the current one when the claim names none", () => {
    const result = choices({ claim: claim("41", { releaseRef: undefined }) });
    expect(
      result.options.filter((o) => o.isCurrent).map((o) => o.name),
    ).toEqual(["camunda-8-9-3"]);
  });

  it("marks the default release of the environment", () => {
    expect(
      choices()
        .options.filter((o) => o.isDefault)
        .map((o) => o.name),
    ).toEqual(["camunda-8-9-3"]);
  });

  it("offers every release, newest first, without an environment", () => {
    expect(
      choices({ environment: undefined }).options.map((o) => o.version),
    ).toEqual(["8.12.0", "8.11.0", "8.10.0", "8.9.3", "8.9.1"]);
  });

  it("runs no version check on a suspended cluster and says when they run", () => {
    const result = choices({
      cluster: at(undefined, { releaseRef: "camunda-8-9-3", suspend: true }),
    });
    expect(result.note).toBe(
      "The version checks run when the cluster resumes.",
    );
    expect(result.options.some((o) => o.disabled || o.downgrade)).toBe(false);
  });

  it("runs no version check on a cluster that reports no version yet", () => {
    const result = choices({ cluster: at(undefined) });
    expect(result.note).toBe(
      "The version checks run when the cluster reports its version.",
    );
    expect(result.options.some((o) => o.disabled || o.downgrade)).toBe(false);
  });

  it("has no note on a cluster that reports its version", () => {
    expect(choices().note).toBeUndefined();
  });
});

describe("releaseLabel", () => {
  it("marks the default release", () => {
    expect(releaseLabel({ name: "camunda-8-9", version: "8.9.9" }, true)).toBe(
      "8.9.9 (camunda-8-9), Default",
    );
  });

  it("names a downgrade and a disabled release as the release panel showed them", () => {
    expect(
      releaseLabel(
        {
          name: "r",
          version: "8.8.1",
          downgrade: { from: "8.9.0", to: "8.8.1" },
        },
        false,
      ),
    ).toBe("8.8.1 (r), downgrade");
    expect(
      releaseLabel(
        { name: "r", version: "8.7.0", disabled: "Skips 8.8." },
        true,
      ),
    ).toBe("8.7.0 (r): Skips 8.8.");
  });
});

describe("createReleaseChoices", () => {
  const named = (name: string, version: string) =>
    ({
      kind: "CamundaRelease",
      metadata: { name },
      spec: { version },
    }) as unknown as Resource<"CamundaRelease">;
  const all = [
    named("camunda-8-8", "8.8.4"),
    named("camunda-8-9", "8.9.9"),
    named("camunda-8-7", "8.7.2"),
  ];
  const prod = (releases?: { allowed: string[]; default?: string }) =>
    ({
      kind: "Environment",
      metadata: { name: "prod" },
      spec: { ...(releases && { releases }) },
    }) as unknown as Environment;

  it("lists the allowed releases newest first with the default marked", () => {
    const options = createReleaseChoices(
      all,
      prod({ allowed: ["camunda-8-8", "camunda-8-9"], default: "camunda-8-9" }),
    );
    expect(options.map((o) => o.name)).toEqual(["camunda-8-9", "camunda-8-8"]);
    expect(options.map((o) => o.isDefault)).toEqual([true, false]);
    expect(
      options.every(
        (o) => !o.isCurrent && o.disabled === undefined && !o.downgrade,
      ),
    ).toBe(true);
  });

  it("lists every release without an Environment", () => {
    const options = createReleaseChoices(all, undefined);
    expect(options.map((o) => o.name)).toEqual([
      "camunda-8-9",
      "camunda-8-8",
      "camunda-8-7",
    ]);
    expect(options.some((o) => o.isDefault)).toBe(false);
  });

  it("lists no release when the Environment allows none", () => {
    expect(createReleaseChoices(all, prod())).toEqual([]);
  });
});
