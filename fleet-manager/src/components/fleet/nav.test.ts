import { describe, expect, it } from "vitest";
import { SETUP_ITEMS, sectionOf, setupBreadcrumb, setupItemOf } from "./nav";

describe("sectionOf", () => {
  it.each([
    ["/fleet", "fleet"],
    ["/fleet/", "fleet"],
    ["/fleet/clusters/team-orders/orders", "fleet"],
    ["/fleet/management", "fleet"],
    ["/fleet/management/new", "fleet"],
    ["/fleet/management/plane-a", "fleet"],
    ["/setup", "setup"],
    ["/setup/presets/gold", "setup"],
    ["/fleetwide", undefined],
    ["/setup-old", undefined],
    ["/dev/gallery", undefined],
    ["/", undefined],
  ])("puts %s in %s", (pathname, section) => {
    expect(sectionOf(pathname)).toBe(section);
  });
});

describe("SETUP_ITEMS", () => {
  it("lists the Setup pages in the order of the contract", () => {
    expect(SETUP_ITEMS.map((item) => item.label)).toEqual([
      "Environments",
      "Presets",
      "Releases",
      "Platform",
      "Connections",
      "Plugins",
      "Settings",
    ]);
    expect(SETUP_ITEMS.map((item) => item.href)).toEqual(
      SETUP_ITEMS.map((item) => `/setup/${item.slug}`),
    );
  });
});

describe("setupItemOf", () => {
  it.each([
    ["/setup/presets", "Presets"],
    ["/setup/presets/gold", "Presets"],
    ["/setup", undefined],
    ["/setup/unknown", undefined],
    ["/fleet/presets", undefined],
  ])("finds the Setup page of %s", (pathname, label) => {
    expect(setupItemOf(pathname)?.label).toBe(label);
  });
});

describe("setupBreadcrumb", () => {
  it("names the list and the object of a detail page", () => {
    expect(setupBreadcrumb("/setup/presets/gold")).toEqual({
      item: SETUP_ITEMS[1],
      name: "gold",
    });
  });

  it("decodes the name", () => {
    expect(setupBreadcrumb("/setup/releases/8.9%20lts")?.name).toBe("8.9 lts");
  });

  it("keeps a name with bad percent-encoding as it is", () => {
    expect(setupBreadcrumb("/setup/presets/%E0%A4%A")?.name).toBe("%E0%A4%A");
  });

  it.each([
    "/setup",
    "/setup/presets",
    "/setup/presets/",
    "/setup/unknown/x",
    "/fleet/presets/gold",
  ])("has no breadcrumb on %s", (pathname) => {
    expect(setupBreadcrumb(pathname)).toBeUndefined();
  });
});
