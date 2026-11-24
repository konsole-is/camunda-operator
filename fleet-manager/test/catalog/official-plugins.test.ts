import { readFileSync } from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { parseCatalog } from "@/startup/bundled-data";

// The release workflow of konsole-is/camunda-operator-plugins rewrites the
// version and the install command of each chart with jq.
const file = path.resolve(
  import.meta.dirname,
  "../../catalog/official-plugins.json",
);
const text = readFileSync(file, "utf8");
const catalog = parseCatalog(JSON.parse(text));

const CHART_REGISTRY =
  "oci://ghcr.io/konsole-is/camunda-operator-plugins/charts";
const REPOSITORY = "https://github.com/konsole-is/camunda-operator-plugins";
// The chart aws turns on none of its controllers by default, and refuses
// to install with none on.
const SWITCHES: Record<string, string> = {
  aws: " --set identity.enabled=true --set connectorsRole.enabled=true --set kms.enabled=true --set ebs.enabled=true --set s3.enabled=true",
};

describe("catalog/official-plugins.json", () => {
  it("lists each planned official chart with the section that it serves", () => {
    expect(
      Object.fromEntries(catalog.charts.map((c) => [c.name, c.sections])),
    ).toEqual({
      "gateway-access": ["access"],
      rustfs: ["backup-storage"],
      aws: [
        "cloud-identity",
        "encryption-key",
        "volume-encryption",
        "backup-storage",
        "connectors-permissions",
      ],
    });
  });

  it("names the controllers whose registrations each chart publishes", () => {
    expect(
      Object.fromEntries(catalog.charts.map((c) => [c.name, c.controllers])),
    ).toEqual({
      "gateway-access": ["gateway-access"],
      rustfs: ["rustfs"],
      aws: [
        "aws-identity",
        "aws-connectors-role",
        "aws-kms",
        "aws-ebs",
        "aws-s3",
      ],
    });
  });

  it("points each chart at its OCI reference and at the plugins repository", () => {
    for (const c of catalog.charts) {
      expect(c.chart).toBe(`${CHART_REGISTRY}/${c.name}`);
      expect(c.repository).toBe(REPOSITORY);
    }
  });

  it("gives every chart the one SemVer version of the plugins repository", () => {
    const versions = new Set(catalog.charts.map((c) => c.version));

    expect(versions.size).toBe(1);
    expect([...versions][0]).toMatch(/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/);
  });

  it("installs the chart at its version, with the controllers on, with the install command", () => {
    for (const c of catalog.charts) {
      expect(c.installCommand).toBe(
        `helm install ${c.name} ${c.chart} --version ${c.version} --namespace camunda-operator-plugins --create-namespace${SWITCHES[c.name] ?? ""}`,
      );
    }
  });

  it("keeps the layout that jq writes, so a release changes only the versions", () => {
    expect(text).toBe(`${JSON.stringify(JSON.parse(text), null, 2)}\n`);
  });
});
