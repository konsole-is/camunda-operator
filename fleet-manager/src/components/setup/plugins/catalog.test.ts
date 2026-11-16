import { describe, expect, it } from "vitest";
import type { CatalogChart } from "@/startup/bundled-data";
import { missingCharts } from "./catalog";
import { pluginRegistration, registrationSources } from "./fixtures";

const chart = (name: string): CatalogChart => ({
  name,
  description: `The ${name} chart.`,
  sections: ["encryption-key"],
  chart: `oci://example/${name}`,
  version: "0.1.0",
  repository: "https://example.com/charts",
  installCommand: `helm install ${name} oci://example/${name} --version 0.1.0`,
});

describe("missingCharts", () => {
  it("leaves out a chart whose name is the controller name of a registration", () => {
    const charts = [chart("aws-kms"), chart("rustfs")];
    const registrations = [
      pluginRegistration("aws-kms", {
        controller: { name: "aws-kms", version: "0.1.0" },
      }),
    ];
    expect(missingCharts(charts, registrations).map((c) => c.name)).toEqual([
      "rustfs",
    ]);
  });

  it("counts a chart as not installed when its controller runs under another name", () => {
    const registrations = [
      pluginRegistration("aws-kms", {
        controller: { name: "kms-eu", version: "0.1.0" },
      }),
    ];
    expect(
      missingCharts([chart("aws-kms")], registrations).map((c) => c.name),
    ).toEqual(["aws-kms"]);
  });

  it("does not count a registration named after the chart without that controller", () => {
    expect(
      missingCharts([chart("rustfs")], [pluginRegistration("rustfs", {})]).map(
        (c) => c.name,
      ),
    ).toEqual(["rustfs"]);
  });

  it("keeps the order of the catalog", () => {
    expect(
      missingCharts(
        [chart("rustfs"), chart("aws-kms"), chart("gateway-access")],
        [],
      ).map((c) => c.name),
    ).toEqual(["rustfs", "aws-kms", "gateway-access"]);
  });

  it("is empty for an empty catalog", () => {
    expect(missingCharts([], registrationSources().registrations)).toEqual([]);
  });
});
