// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { condition, pluginGrant, pluginRegistration } from "./fixtures";
import { GRANT_REASON_TEXT } from "./grants";
import { GrantsTab } from "./grants-tab";

afterEach(cleanup);

const NOW = "2026-11-13T09:00:00Z";
const role = pluginRegistration("aws-s3", {
  grantTemplates: [
    {
      name: "bucket-access",
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
const tab = (grants: ReturnType<typeof grant>[], unread: string[] = []) =>
  render(
    <GrantsTab registration={role} grants={grants} unread={unread} now={NOW} />,
  );

describe("GrantsTab", () => {
  it("says that an option without grant templates writes no Grant", () => {
    render(
      <GrantsTab
        registration={pluginRegistration("rustfs", {})}
        grants={[]}
        unread={[]}
        now={NOW}
      />,
    );
    expect(
      screen.getByText(
        "This option owns the policies of its role and writes no Grant.",
      ),
    ).toBeTruthy();
  });

  it("shows each template with its format, its statement, its inputs, and its counts", () => {
    tab([
      grant("a", "bucket-access", [condition("Ready", "True", "Healthy")]),
      grant("b", "bucket-access", [condition("Ready", "False", "Rejected")]),
    ]);
    const panel = screen.getByRole("region", { name: "bucket-access" });
    expect(within(panel).getByText("aws-iam-statement")).toBeTruthy();
    expect(within(panel).getByText(/\{\{ \.outputs\.bucket \}\}/)).toBeTruthy();
    expect(within(panel).getByText("principal")).toBeTruthy();
    expect(within(panel).getByText("1 ready, 1 refused")).toBeTruthy();
  });

  it("shows Applied, MatchesTemplate, and Ready of each Grant in that order", () => {
    tab([
      grant("a", "bucket-access", [
        condition("Ready", "True", "Healthy"),
        condition("Applied", "True", "Healthy"),
        condition("MatchesTemplate", "True", "Healthy"),
      ]),
    ]);
    const list = screen.getByRole("list", { name: "Conditions of a" });
    expect(
      within(list)
        .getAllByRole("listitem")
        .map((li) => li.querySelector(".font-mono")?.textContent),
    ).toEqual(["Applied", "MatchesTemplate", "Ready"]);
  });

  it.each(["OverCeiling", "OverSizeLimit", "Rejected"])(
    "shows the Applied reason %s of a refused Grant",
    (reason) => {
      tab([
        grant("b", "bucket-access", [
          condition("Applied", "False", reason),
          condition("MatchesTemplate", "True", "Healthy"),
          condition("Ready", "False", reason),
        ]),
      ]);
      const list = screen.getByRole("list", { name: "Conditions of b" });
      expect(within(list).getAllByText(reason).length).toBe(2);
      expect(
        screen.getByText(
          GRANT_REASON_TEXT[reason as keyof typeof GRANT_REASON_TEXT],
        ),
      ).toBeTruthy();
    },
  );

  it("shows TemplateChanged of MatchesTemplate", () => {
    tab([
      grant("c", "bucket-access", [
        condition("Applied", "True", "Healthy"),
        condition("MatchesTemplate", "False", "TemplateChanged"),
        condition("Ready", "False", "TemplateChanged"),
      ]),
    ]);
    expect(screen.getByText(GRANT_REASON_TEXT.TemplateChanged)).toBeTruthy();
  });

  it("lists a Grant of a template that the registration no longer has", () => {
    tab([
      grant("old", "gone", [condition("Ready", "False", "TemplateChanged")]),
    ]);
    const panel = screen.getByRole("region", {
      name: "Grants of a template the registration no longer has",
    });
    expect(within(panel).getByText("team-orders/old")).toBeTruthy();
  });

  it("shows no counts and no empty list while Grant is not read", () => {
    tab([], ["Grant"]);
    const panel = screen.getByRole("region", { name: "bucket-access" });
    expect(within(panel).queryByText(/ready, .* refused/)).toBeNull();
    expect(
      within(panel).queryByText("No Grant of this template yet."),
    ).toBeNull();
    expect(
      within(panel).getByText("Not known: Grant is not read."),
    ).toBeTruthy();
  });

  it("does not say that an option writes no Grant while Grant is not read", () => {
    render(
      <GrantsTab
        registration={pluginRegistration("rustfs", {})}
        grants={[]}
        unread={["Grant"]}
        now={NOW}
      />,
    );
    expect(
      screen.queryByText(
        "This option owns the policies of its role and writes no Grant.",
      ),
    ).toBeNull();
    expect(screen.getByText("This option has no grant template.")).toBeTruthy();
    expect(screen.getByText("Not known: Grant is not read.")).toBeTruthy();
  });

  it("shows the error of a Grant whose conditions are not valid", () => {
    tab([grant("a", "bucket-access", [{ type: "Ready", status: 1 } as never])]);
    expect(screen.getByRole("alert").textContent).toBe(
      "Condition 1 is not a valid condition.",
    );
  });
});
