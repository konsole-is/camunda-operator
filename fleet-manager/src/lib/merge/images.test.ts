import { describe, expect, it } from "vitest";
import { loadBundledData, parseImagesTable } from "@/startup/bundled-data";
import {
  IMAGE_FIELDS,
  defaultImages,
  defaultRepositories,
  minorOf,
  type ImageField,
} from "./images";

const table = parseImagesTable({
  schemaVersion: 1,
  minors: {
    "8.9": {
      camunda: { repository: "camunda/camunda", version: "8.9.23" },
      identity: { repository: "camunda/identity", version: "8.9.10" },
      console: { repository: "camunda/console", version: "8.9.122" },
      keycloak: {
        repository: "camunda/keycloak",
        tagPrefix: "quay-optimized-",
        version: "26.6.4",
      },
      elasticsearch: { version: "9.2.8" },
    },
  },
});

describe("defaultImages", () => {
  it("returns each image of the minor of a version, on its own version line", () => {
    expect(defaultImages(table, "8.9.4")).toEqual({
      camunda: { repository: "camunda/camunda", tag: "8.9.23" },
      identity: { repository: "camunda/identity", tag: "8.9.10" },
      console: { repository: "camunda/console", tag: "8.9.122" },
      keycloak: {
        repository: "camunda/keycloak",
        tag: "quay-optimized-26.6.4",
      },
      elasticsearch: { tag: "9.2.8" },
    });
  });

  it("returns undefined for a minor that the table does not hold", () => {
    expect(defaultImages(table, "8.10.0")).toBeUndefined();
  });

  it("returns undefined for a version that has no minor", () => {
    expect(defaultImages(table, "8")).toBeUndefined();
    expect(defaultImages(table, "")).toBeUndefined();
    expect(defaultImages(table, "latest")).toBeUndefined();
  });

  it("reads the table of the checkout for Camunda 8.9", async () => {
    const { imagesTable } = await loadBundledData({});

    const images = defaultImages(imagesTable, "8.9.0");

    expect(images?.identity?.repository).toBe("camunda/identity");
    expect(images?.console?.repository).toBe("camunda/console");
    expect(images?.keycloak?.tag).toMatch(/^quay-optimized-\d/);
  });
});

const twoMinors = parseImagesTable({
  schemaVersion: 1,
  minors: {
    "8.9": {
      camunda: { repository: "camunda/camunda", version: "8.9.28" },
      "web-modeler-restapi": {
        repository: "camunda/web-modeler-restapi",
        version: "8.9.15",
      },
      postgres: {
        repository: "ghcr.io/cloudnative-pg/postgresql",
        version: "17",
      },
      elasticsearch: { version: "9.2.8" },
    },
    "8.10": {
      camunda: { repository: "camunda/camunda", version: "8.10.2" },
      "web-modeler-restapi": { repository: "camunda/hub", version: "8.10.1" },
      postgres: {
        repository: "ghcr.io/cloudnative-pg/postgresql",
        version: "17",
      },
    },
  },
});

describe("minorOf", () => {
  it("returns the major and minor of a version", () => {
    expect(minorOf("8.10.2")).toBe("8.10");
    expect(minorOf("8.9")).toBe("8.9");
  });

  it("returns undefined for a version without a minor", () => {
    expect(minorOf("8")).toBeUndefined();
    expect(minorOf("latest")).toBeUndefined();
    expect(minorOf("")).toBeUndefined();
  });
});

describe("defaultRepositories", () => {
  it("gives one default without a range for one minor", () => {
    const d = defaultRepositories(twoMinors, ["8.9"]);

    expect(d.minors).toEqual(["8.9"]);
    expect(d.fromUsers).toBe(true);
    expect(d.fields.webModelerRestapi).toEqual([
      { repository: "camunda/web-modeler-restapi", from: "8.9", to: "8.9" },
    ]);
  });

  it("groups the minors of one repository and splits where the repository changes", () => {
    const d = defaultRepositories(twoMinors, ["8.10", "8.9", "8.10"]);

    expect(d.minors).toEqual(["8.9", "8.10"]);
    expect(d.fields.camunda).toEqual([
      { repository: "camunda/camunda", from: "8.9", to: "8.10" },
    ]);
    expect(d.fields.webModelerRestapi).toEqual([
      { repository: "camunda/web-modeler-restapi", from: "8.9", to: "8.9" },
      { repository: "camunda/hub", from: "8.10", to: "8.10" },
    ]);
  });

  it("lists the minors that the table does not hold", () => {
    const d = defaultRepositories(twoMinors, ["8.11", "8.8", "8.9"]);

    expect(d.missing).toEqual(["8.8", "8.11"]);
    expect(d.minors).toEqual(["8.9"]);
    expect(d.fields.camunda).toEqual([
      { repository: "camunda/camunda", from: "8.9", to: "8.9" },
    ]);
  });

  it("shows the newest minor of the table when no user runs a minor", () => {
    const d = defaultRepositories(twoMinors, []);

    expect(d).toMatchObject({
      minors: ["8.10"],
      fromUsers: false,
      missing: [],
    });
  });

  it("shows no minor when every minor of the users is missing from the table", () => {
    const d = defaultRepositories(twoMinors, ["8.8"]);

    expect(d).toMatchObject({ minors: [], fromUsers: true, missing: ["8.8"] });
  });

  it("ends a group at a minor whose entry lacks the image", () => {
    const gap = parseImagesTable({
      schemaVersion: 1,
      minors: {
        "8.9": { console: { repository: "camunda/console", version: "8.9.1" } },
        "8.10": {},
        "8.11": {
          console: { repository: "camunda/console", version: "8.11.1" },
        },
      },
    });

    expect(
      defaultRepositories(gap, ["8.9", "8.10", "8.11"]).fields.console,
    ).toEqual([
      { repository: "camunda/console", from: "8.9", to: "8.9" },
      { repository: "camunda/console", from: "8.11", to: "8.11" },
    ]);
  });

  it("does not join minors across a requested minor that the table does not hold", () => {
    const hole = parseImagesTable({
      schemaVersion: 1,
      minors: {
        "8.9": { console: { repository: "camunda/console", version: "8.9.1" } },
        "8.11": {
          console: { repository: "camunda/console", version: "8.11.1" },
        },
      },
    });
    const d = defaultRepositories(hole, ["8.9", "8.10", "8.11"]);

    expect(d.missing).toEqual(["8.10"]);
    expect(d.minors).toEqual(["8.9", "8.11"]);
    expect(d.fields.console).toEqual([
      { repository: "camunda/console", from: "8.9", to: "8.9" },
      { repository: "camunda/console", from: "8.11", to: "8.11" },
    ]);
  });

  it("gives no default for a field whose image the table does not hold", () => {
    expect(defaultRepositories(twoMinors, ["8.9"]).fields.console).toEqual([]);
  });

  it("gives no minor and no default for an empty table", () => {
    const d = defaultRepositories({ schemaVersion: 1, minors: {} }, []);

    expect(d.minors).toEqual([]);
    expect(Object.values(d.fields).every((f) => f.length === 0)).toBe(true);
  });

  it("maps every image field of the platform config and never elasticsearch", () => {
    expect(Object.values(IMAGE_FIELDS)).not.toContain("elasticsearch");
    expect(Object.keys(IMAGE_FIELDS)).toEqual([
      "camunda",
      "connectors",
      "optimize",
      "identity",
      "console",
      "webModelerRestapi",
      "webModelerWebsockets",
      "keycloak",
      "postgres",
    ]);
  });

  it("reads a repository for every image field from the table of the checkout", async () => {
    const { imagesTable } = await loadBundledData({});

    const d = defaultRepositories(imagesTable, ["8.9"]);

    for (const field of Object.keys(IMAGE_FIELDS) as ImageField[])
      expect(d.fields[field], field).toHaveLength(1);
  });
});
