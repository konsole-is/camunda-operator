import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { ACCESS_SECTION, launchLinks } from "./launch";

describe("launchLinks", () => {
  it("builds the three links from externalUrl", () => {
    expect(launchLinks("https://c.example.com", undefined)).toEqual([
      { label: "Operate", href: "https://c.example.com/operate" },
      { label: "Tasklist", href: "https://c.example.com/tasklist" },
      { label: "Admin", href: "https://c.example.com/admin" },
    ]);
  });

  it("drops a trailing slash", () => {
    expect(launchLinks("https://c.example.com/", undefined)[0].href).toBe(
      "https://c.example.com/operate",
    );
  });

  it("keeps a path of externalUrl", () => {
    expect(
      launchLinks("https://h.example.com/x/", undefined).map((l) => l.href),
    ).toEqual([
      "https://h.example.com/x/operate",
      "https://h.example.com/x/tasklist",
      "https://h.example.com/x/admin",
    ]);
  });

  it("uses the operate, tasklist, and admin links of the access request without externalUrl", () => {
    expect(
      launchLinks(undefined, [
        { name: "admin", url: "https://a.example.com" },
        { name: "grpc", url: "grpc.example.com:443" },
        { name: "operate", url: "https://o.example.com" },
      ]),
    ).toEqual([
      { label: "Operate", href: "https://o.example.com" },
      { label: "Admin", href: "https://a.example.com" },
    ]);
  });

  it.each([
    ["no URL and no links", undefined, undefined],
    ["an empty URL and no links", "", []],
  ])("has no links with %s", (_, url, links) => {
    expect(launchLinks(url, links)).toEqual([]);
  });

  it("prefers externalUrl over the access links", () => {
    expect(
      launchLinks("https://c.example.com", [
        { name: "operate", url: "https://o.example.com" },
      ])[0].href,
    ).toBe("https://c.example.com/operate");
  });
});

describe("ACCESS_SECTION", () => {
  it("matches SectionAccess of internal/pluginstartup/names.go", () => {
    const source = readFileSync(
      new URL("../../../../internal/pluginstartup/names.go", import.meta.url),
      "utf8",
    );
    expect(ACCESS_SECTION).toBe(
      /\bSectionAccess\s*=\s*"([^"]*)"/.exec(source)?.[1],
    );
  });
});
