import { describe, expect, it } from "vitest";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { condition, pluginGrant, pluginRegistration } from "./fixtures";
import {
  GRANT_REASON_TEXT,
  type GrantTemplateView,
  grantTemplates,
} from "./grants";

const role = pluginRegistration("aws-s3", {
  grantTemplates: [
    {
      name: "bucket-access",
      description: "Lets the cluster read and write its backup bucket.",
      format: "aws-iam-statement",
      template: {
        Effect: "Allow",
        Action: ["s3:GetObject"],
        Resource: "arn:aws:s3:::{{ .outputs.bucket }}/*",
      },
      requiresInputs: ["principal"],
    },
  ],
});
const grant = (name: string, template: string, conditions: ConditionRow[]) =>
  pluginGrant(
    "team-orders",
    name,
    { template, cluster: { name: "orders" } },
    {
      "plugins.camunda.io/registration": "aws-s3",
      "plugins.camunda.io/request": "orders-backup-storage",
    },
    conditions,
  );
const first = (view: ReturnType<typeof grantTemplates>) =>
  view.templates[0] as GrantTemplateView;

describe("grantTemplates", () => {
  it("counts ready and refused Grants of each template", () => {
    const view = grantTemplates(role, [
      grant("a", "bucket-access", [
        condition("Applied", "True", "Healthy"),
        condition("MatchesTemplate", "True", "Healthy"),
        condition("Ready", "True", "Healthy"),
      ]),
      grant("b", "bucket-access", [
        condition("Applied", "False", "OverCeiling"),
        condition("Ready", "False", "OverCeiling"),
      ]),
      grant("c", "bucket-access", []),
    ]);
    expect(first(view)).toMatchObject({
      name: "bucket-access",
      description: "Lets the cluster read and write its backup bucket.",
      format: "aws-iam-statement",
      ready: 1,
      refused: 1,
      requiresInputs: ["principal"],
    });
    expect(first(view).grants.map((g) => g.state)).toEqual([
      "ready",
      "refused",
      "waiting",
    ]);
    expect(first(view).grants[1]).toMatchObject({
      namespace: "team-orders",
      cluster: "orders",
      request: "orders-backup-storage",
      applied: expect.objectContaining({ reason: "OverCeiling" }),
    });
  });

  it("keeps the Go placeholders of the template as written", () => {
    expect(first(grantTemplates(role, [])).template).toContain(
      '"arn:aws:s3:::{{ .outputs.bucket }}/*"',
    );
  });

  it("keeps a Grant whose template the registration no longer has", () => {
    const view = grantTemplates(role, [
      grant("old", "gone", [condition("Ready", "False", "TemplateChanged")]),
    ]);
    expect(view.withoutTemplate.map((g) => g.name)).toEqual(["old"]);
    expect(first(view).grants).toEqual([]);
  });

  it("gives an error for a grant template that is not valid", () => {
    const view = grantTemplates(
      pluginRegistration("bad", {
        grantTemplates: [
          { name: "x", format: "aws-iam-statement", template: "Allow" },
        ] as never,
      }),
      [],
    );
    expect(view.templates).toEqual([
      { error: "Grant template 1 is not a valid grant template." },
    ]);
  });

  it("gives an error for a Grant whose conditions are not valid", () => {
    const view = grantTemplates(role, [
      grant("a", "bucket-access", [{ type: "Ready", status: 1 } as never]),
    ]);
    expect(first(view).grants[0]).toMatchObject({
      name: "a",
      error: "Condition 1 is not a valid condition.",
    });
  });
});

describe("GRANT_REASON_TEXT", () => {
  it("has a text for each refusal reason of api/plugins/v1alpha1", () => {
    expect(Object.keys(GRANT_REASON_TEXT).sort()).toEqual([
      "OverCeiling",
      "OverSizeLimit",
      "Rejected",
      "TemplateChanged",
    ]);
  });
});
