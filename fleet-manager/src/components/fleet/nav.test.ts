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
    ["/dev/watch-probe", undefined],
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

  it("puts each create page beside the lists, where no object name can take its path", () => {
    expect(
      SETUP_ITEMS.flatMap((item) => (item.create ? [item.create] : [])),
    ).toEqual([
      { href: "/setup/new-environment", label: "New environment" },
      { href: "/setup/new-preset", label: "New preset" },
      { href: "/setup/new-release", label: "New release" },
    ]);
  });
});

describe("setupItemOf", () => {
  it.each([
    ["/setup/presets", "Presets"],
    ["/setup/presets/gold", "Presets"],
    ["/setup/new-environment", "Environments"],
    ["/setup/environments/new", "Environments"],
    ["/setup/new-release", "Releases"],
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

  it("names the object new on the page of an Environment named new", () => {
    expect(setupBreadcrumb("/setup/environments/new")).toEqual({
      item: SETUP_ITEMS[0],
      name: "new",
    });
  });

  it("names the create page by its label", () => {
    expect(setupBreadcrumb("/setup/new-environment")).toEqual({
      item: SETUP_ITEMS[0],
      name: "New environment",
    });
  });

  it("keeps a name with bad percent-encoding as it is", () => {
    expect(setupBreadcrumb("/setup/presets/%E0%A4%A")?.name).toBe("%E0%A4%A");
  });

  it("names the preset, not its kind, on a preset detail page", () => {
    expect(setupBreadcrumb("/setup/presets/elasticsearch/standard")).toEqual({
      item: SETUP_ITEMS[1],
      name: "standard",
    });
  });

  it("decodes the preset name and keeps a bad segment as it is", () => {
    expect(setupBreadcrumb("/setup/presets/cluster/8.9%20lts")?.name).toBe(
      "8.9 lts",
    );
    expect(setupBreadcrumb("/setup/presets/cluster/%E0%A4%A")?.name).toBe(
      "%E0%A4%A",
    );
  });

  it("names the namespace and the object on a namespaced detail page", () => {
    expect(setupBreadcrumb("/setup/connections/kind/team%20a/orders")).toEqual({
      item: SETUP_ITEMS[4],
      name: "team a/orders",
    });
  });

  it("names a contract by its namespace and name, and a Management Identity contract by its name", () => {
    expect(
      setupBreadcrumb("/setup/connections/object-storage/team-a/backups"),
    ).toEqual({ item: SETUP_ITEMS[4], name: "team-a/backups" });
    expect(
      setupBreadcrumb(
        "/setup/connections/management-identity/plane-a-optimize",
      ),
    ).toEqual({ item: SETUP_ITEMS[4], name: "plane-a-optimize" });
  });

  it.each([
    "/setup",
    "/setup/presets",
    "/setup/presets/",
    "/setup/presets/cluster/",
    "/setup/connections/kind/team-a/",
    "/setup/unknown/x",
    "/fleet/presets/gold",
  ])("has no breadcrumb on %s", (pathname) => {
    expect(setupBreadcrumb(pathname)).toBeUndefined();
  });
});
