import { describe, expect, it } from "vitest";
import type { ContractRef } from "@/lib/usage";
import {
  CONNECTION_TABS,
  connectionTabOf,
  contractHref,
  contractRefOf,
} from "./tabs";

describe("CONNECTION_TABS", () => {
  it("lists the five tabs in the order of the spec", () => {
    expect(CONNECTION_TABS.map((t) => [t.label, t.kind])).toEqual([
      ["Object storage", "ObjectStorageConfig"],
      ["Secondary storage", "SecondaryStorageConfig"],
      ["Database servers", "DatabaseServerConfig"],
      ["Databases", "DatabaseConfig"],
      ["Management Identity", "ManagementAuthConfig"],
    ]);
  });
});

describe("connectionTabOf", () => {
  it("reads a known tab", () => {
    expect(connectionTabOf("databases")).toBe("databases");
  });

  it("reads the first item of a repeated value", () => {
    expect(connectionTabOf(["databases", "object-storage"])).toBe("databases");
  });

  it.each([undefined, "", "buckets", ["buckets", "databases"]])(
    "falls back to Object storage for %j",
    (value) => {
      expect(connectionTabOf(value)).toBe("object-storage");
    },
  );
});

describe("contractRefOf", () => {
  it("reads a namespaced contract from two segments and Management Identity from one", () => {
    expect(contractRefOf("object-storage", ["team-a", "backups"])).toEqual({
      kind: "ObjectStorageConfig",
      namespace: "team-a",
      name: "backups",
    });
    expect(contractRefOf("management-identity", ["plane-a"])).toEqual({
      kind: "ManagementAuthConfig",
      name: "plane-a",
    });
  });

  it.each<[string, string[]]>([
    ["object-storage", ["backups"]],
    ["object-storage", ["a", "b", "c"]],
    ["management-identity", ["a", "b"]],
    ["management-identity", []],
    ["buckets", ["a", "b"]],
  ])("refuses the tab %j with the segments %j", (tab, segments) => {
    expect(contractRefOf(tab, segments)).toBeUndefined();
  });
});

describe("contractHref", () => {
  it.each<ContractRef>([
    { kind: "ObjectStorageConfig", namespace: "team-a", name: "backups" },
    { kind: "SecondaryStorageConfig", namespace: "team-a", name: "orders-es" },
    { kind: "DatabaseServerConfig", namespace: "platform", name: "postgres" },
    { kind: "DatabaseConfig", namespace: "platform", name: "identity-db" },
    { kind: "ManagementAuthConfig", name: "plane-a-optimize" },
  ])("builds the href that contractRefOf reads back for %j", (ref) => {
    const [, setup, connections, tab, ...segments] =
      contractHref(ref).split("/");
    expect([setup, connections]).toEqual(["setup", "connections"]);
    expect(
      contractRefOf(
        tab,
        segments.map((s) => decodeURIComponent(s)),
      ),
    ).toEqual(ref);
  });

  it("gives the paths of the routes", () => {
    expect(
      contractHref({
        kind: "ObjectStorageConfig",
        namespace: "team-a",
        name: "backups",
      }),
    ).toBe("/setup/connections/object-storage/team-a/backups");
    expect(
      contractHref({ kind: "ManagementAuthConfig", name: "plane-a" }),
    ).toBe("/setup/connections/management-identity/plane-a");
  });
});
