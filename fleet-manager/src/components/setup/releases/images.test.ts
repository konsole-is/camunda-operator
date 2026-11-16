import { describe, expect, it } from "vitest";
import { imagesFixture } from "./fixtures";
import { releaseImages } from "./images";

const rowsOf = (spec: Parameters<typeof releaseImages>[0]) =>
  Object.fromEntries(
    (releaseImages(spec, imagesFixture) ?? []).map((row) => [row.image, row]),
  );

describe("releaseImages", () => {
  it("tags camunda and optimize with the release version and connectors with the connectors version", () => {
    const rows = rowsOf({ version: "8.9.3", connectors: { version: "8.9.7" } });
    expect(rows.camunda).toMatchObject({
      label: "Camunda",
      repository: "camunda/camunda",
      tag: "8.9.3",
    });
    expect(rows.optimize).toMatchObject({
      repository: "camunda/optimize",
      tag: "8.9.3",
    });
    expect(rows.connectors).toMatchObject({
      repository: "camunda/connectors-bundle",
      tag: "8.9.7",
    });
  });

  it("keeps the table tag for Elasticsearch and PostgreSQL, without a repository for Elasticsearch", () => {
    const rows = rowsOf({ version: "8.9.3" });
    expect(rows.elasticsearch.tag).toBe("9.2.8");
    expect(rows.elasticsearch.repository).toBeUndefined();
    expect(rows.postgres).toMatchObject({
      label: "PostgreSQL",
      repository: "ghcr.io/cloudnative-pg/postgresql",
      tag: "17",
    });
  });

  it("names the pin field only of the camunda and connectors images", () => {
    const rows = rowsOf({ version: "8.9.3" });
    expect(rows.camunda.pinField).toBe("camunda");
    expect(rows.connectors.pinField).toBe("connectors");
    expect(rows.optimize.pinField).toBeUndefined();
    expect(rows.elasticsearch.pinField).toBeUndefined();
    expect(rows.postgres.pinField).toBeUndefined();
  });

  it("leaves the connectors tag unset when the release sets no connectors version", () => {
    expect(rowsOf({ version: "8.9.3" }).connectors.tag).toBeUndefined();
  });

  it("returns undefined for a minor that the table does not hold", () => {
    expect(releaseImages({ version: "8.10.0" }, imagesFixture)).toBeUndefined();
  });

  it("leaves out the images that a release does not run", () => {
    expect(
      (releaseImages({ version: "8.9.3" }, imagesFixture) ?? []).map(
        (row) => row.image,
      ),
    ).toEqual([
      "camunda",
      "optimize",
      "connectors",
      "elasticsearch",
      "postgres",
    ]);
  });
});
