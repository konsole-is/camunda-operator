// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { GrantPreview } from "@/lib/claims/templates";
import { GrantPreviewList } from "./grant-preview";

afterEach(cleanup);

const picked: GrantPreview = {
  section: "backup-storage",
  sectionLabel: "Backup storage",
  registration: "s3",
  optionLabel: "S3 bucket",
  template: "read",
  description: "Read the backups",
  result: {
    value: {
      Effect: "Allow",
      Resource: "<bucket ARN from Backup storage>",
      Policy: "<policy: SQS access>",
    },
  },
};

const failing: GrantPreview = {
  ...picked,
  template: "tags",
  description: undefined,
  result: {
    error:
      'parameter "tags" is a list: a template prints only a string, a number, or a boolean',
    field: "parameters.tags",
    path: "Tags",
  },
};

describe("GrantPreviewList", () => {
  it("shows a picked choice by its label and never an ARN", () => {
    render(<GrantPreviewList previews={[picked]} />);
    const list = screen.getByRole("region", {
      name: "Permissions the options ask for",
    });
    expect(list.textContent).toContain("Backup storage: S3 bucket");
    expect(list.textContent).toContain("read");
    expect(list.textContent).toContain("Read the backups");
    expect(list.textContent).toContain("<policy: SQS access>");
    expect(list.textContent).toContain("<bucket ARN from Backup storage>");
    expect(list.textContent).not.toContain("arn:");
  });

  it("shows the error of a template that cannot render, with its path", () => {
    render(<GrantPreviewList previews={[picked, failing]} />);
    expect(screen.getByRole("alert").textContent).toBe(
      'Tags: parameter "tags" is a list: a template prints only a string, a number, or a boolean',
    );
    expect(
      screen.getByText("<policy: SQS access>", { exact: false }),
    ).toBeTruthy();
  });

  it("says which inputs a template needs before the plugin writes it", () => {
    render(
      <GrantPreviewList
        previews={[
          picked,
          { ...picked, template: "key-use", onlyWhen: ["keyArn"] },
          { ...picked, template: "both", onlyWhen: ["keyArn", "queueArn"] },
        ]}
      />,
    );
    const items = screen.getAllByRole("listitem");
    expect(items[0].textContent).not.toContain("Only when");
    expect(items[1].textContent).toContain(
      "Only when the input keyArn has a value",
    );
    expect(items[2].textContent).toContain(
      "Only when the inputs keyArn, queueArn have a value",
    );
  });

  it("renders nothing without previews", () => {
    const { container } = render(<GrantPreviewList previews={[]} />);
    expect(container.textContent).toBe("");
  });

  it("names the kinds that could not be read and shows no previews", () => {
    render(
      <GrantPreviewList previews={[picked]} unread={["PluginRegistration"]} />,
    );
    const list = screen.getByRole("region", {
      name: "Permissions the options ask for",
    });
    expect(within(list).getByRole("alert").textContent).toBe(
      "The fleet manager could not read the PluginRegistration objects, so the permissions are not known.",
    );
    expect(list.textContent).not.toContain("SQS access");
  });
});
