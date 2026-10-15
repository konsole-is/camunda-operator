import { describe, expect, it } from "vitest";
import { loadBundledData, parseImagesTable } from "@/startup/bundled-data";
import { defaultImages } from "./images";

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
