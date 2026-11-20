import { describe, expect, it } from "vitest";
import type { SourceReads } from "@/lib/k8s/unread";
import { releaseFixtures } from "./fixtures";
import { allowedInText, defaultInText, releaseRows, usedByText } from "./rows";

const rowOf = (name: string) => {
  const row = releaseRows(releaseFixtures()).find((r) => r.name === name);
  if (!row) throw new Error(`no row ${name}`);
  return row;
};

describe("releaseRows", () => {
  it("sorts the rows by name", () => {
    expect(releaseRows(releaseFixtures()).map((r) => r.name)).toEqual([
      "camunda-8-10-0",
      "camunda-8-8-5",
      "camunda-8-9-0",
      "camunda-8-9-1",
    ]);
  });

  it("shows only the four facts", () => {
    for (const row of releaseRows(releaseFixtures())) {
      expect(usedByText(row.usage)).toMatch(
        /^(Unused|\d+ (cluster|Elasticsearch cluster|database server|claim)s?(, \d+ (cluster|Elasticsearch cluster|database server)s?)*)$/,
      );
      const defaults = defaultInText(row);
      if (defaults !== undefined) expect(defaults).toMatch(/^Default in /);
      expect(allowedInText(row)).toMatch(
        /^(Allowed in .+|Not offered to new clusters)$/,
      );
    }
  });

  it("names environments by their display name", () => {
    const row = rowOf("camunda-8-9-0");
    expect(defaultInText(row)).toBe("Default in Production");
    expect(allowedInText(row)).toBe("Allowed in Production, Staging");
    expect(row.defaultIn).toEqual([
      { name: "production", label: "Production" },
    ]);
  });

  it("names an environment without a display name by its name", () => {
    const sources = releaseFixtures();
    delete sources.environments[1].spec.displayName;
    const row = releaseRows(sources).find((r) => r.name === "camunda-8-8-5");
    expect(row && allowedInText(row)).toBe("Allowed in staging");
  });

  it("offers delete only for an unused release that no environment allows", () => {
    expect(
      releaseRows(releaseFixtures())
        .filter((r) => r.deletable)
        .map((r) => r.name),
    ).toEqual(["camunda-8-9-1"]);
    expect(allowedInText(rowOf("camunda-8-9-1"))).toBe(
      "Not offered to new clusters",
    );
    expect(defaultInText(rowOf("camunda-8-9-1"))).toBeUndefined();
  });

  it("counts a claim as a user and offers no delete", () => {
    const row = rowOf("camunda-8-10-0");
    expect(usedByText(row.usage)).toBe("1 claim");
    expect(row.deletable).toBe(false);
  });

  it("keeps an unused release that an environment allows", () => {
    const row = rowOf("camunda-8-8-5");
    expect(usedByText(row.usage)).toBe("Unused");
    expect(allowedInText(row)).toBe("Allowed in Staging");
    expect(row.deletable).toBe(false);
  });

  it("offers no delete while a list failed or is not read on the connection", () => {
    const deletable = (reads: SourceReads) =>
      releaseRows({ ...releaseFixtures(), ...reads })
        .filter((r) => r.deletable)
        .map((r) => r.name);
    expect(
      deletable({
        errors: [{ kind: "CamundaClusterClaim", message: "forbidden" }],
      }),
    ).toEqual([]);
    expect(deletable({ stale: ["Namespace"] })).toEqual([]);
    expect(deletable({ errors: [], stale: [] })).toEqual(["camunda-8-9-1"]);
  });

  it("reads the versions of the release", () => {
    const row = rowOf("camunda-8-9-0");
    expect([
      row.camunda,
      row.connectors,
      row.elasticsearch,
      row.postgres,
    ]).toEqual(["8.9.0", "8.9.7", "9.2.4", "17"]);
    expect(rowOf("camunda-8-9-1").connectors).toBeUndefined();
  });
});

describe("usedByText", () => {
  it("counts each kind and leaves out claims when a resource uses the release", () => {
    expect(
      usedByText({
        clusters: ["a/one", "b/two"],
        elasticsearchClusters: [],
        databaseServers: ["a/db"],
        claims: ["a/one"],
      }),
    ).toBe("2 clusters, 1 database server");
  });
});
