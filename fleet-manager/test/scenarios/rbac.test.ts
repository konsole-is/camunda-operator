import fs from "node:fs";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { CHART_DIR, chartRules } from "./rbac";

function rulesOf(file: string) {
  return chartRules(fs.readFileSync(file, "utf8"), file);
}

describe("chartRules", () => {
  it("reads the rules of the chart ClusterRole", () => {
    const rules = rulesOf(path.join(CHART_DIR, "clusterrole.yaml"));
    expect(rules).toContainEqual({
      apiGroups: [""],
      resources: ["pods/log"],
      verbs: ["get"],
    });
    expect(
      rules.find((r) => r.resources?.includes("camundaclusterclaims"))?.verbs,
    ).toContain("delete");
  });

  it("reads the rules of the chart Role", () => {
    const rules = rulesOf(path.join(CHART_DIR, "role.yaml"));
    expect(rules).toEqual([
      { apiGroups: [""], resources: ["configmaps"], verbs: ["create"] },
      {
        apiGroups: [""],
        resources: ["configmaps"],
        resourceNames: ["fleet-manager-settings"],
        verbs: ["get", "update", "patch"],
      },
    ]);
  });

  it("refuses a rules block with a template", () => {
    const file = path.resolve(
      "test/fixtures/scenarios/chart-templated/role.yaml",
    );
    expect(() => rulesOf(file)).toThrow(
      `The rules of ${file} hold a template. Read them with helm template instead`,
    );
  });

  it("refuses a template directive inside the rules block", () => {
    const text = [
      "{{- if .Values.fleetManager.enabled }}",
      "kind: Role",
      "rules:",
      "  {{- if .Values.allowDelete }}",
      '  - apiGroups: [""]',
      "    resources: [configmaps]",
      "    verbs: [delete]",
      "  {{- end }}",
      "{{- end }}",
      "",
    ].join("\n");
    expect(() => chartRules(text, "role.yaml")).toThrow(
      "The rules of role.yaml hold a template",
    );
  });

  it("refuses a file without a rules block", () => {
    expect(() => chartRules("kind: Role\n", "role.yaml")).toThrow(
      /role.yaml has no rules/,
    );
  });
});
