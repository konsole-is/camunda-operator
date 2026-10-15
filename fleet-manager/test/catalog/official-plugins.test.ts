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

describe("catalog/official-plugins.json", () => {
  it("lists each planned official chart with the section that it serves", () => {
    expect(
      Object.fromEntries(catalog.charts.map((c) => [c.name, c.sections])),
    ).toEqual({
      "gateway-access": ["access"],
      rustfs: ["backup-storage"],
      "aws-identity": ["cloud-identity"],
      "aws-kms": ["encryption-key"],
      "aws-ebs": ["volume-encryption"],
      "aws-s3": ["backup-storage"],
      "aws-connectors-role": ["connectors-permissions"],
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

  it("installs the chart at its version with the install command", () => {
    for (const c of catalog.charts) {
      expect(c.installCommand).toBe(
        `helm install ${c.name} ${c.chart} --version ${c.version} --namespace camunda-operator-plugins --create-namespace`,
      );
    }
  });

  it("keeps the layout that jq writes, so a release changes only the versions", () => {
    expect(text).toBe(`${JSON.stringify(JSON.parse(text), null, 2)}\n`);
  });
});
