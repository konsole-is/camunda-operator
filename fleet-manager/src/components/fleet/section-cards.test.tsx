// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { OptionView, SectionView } from "@/lib/policy/types";
import { SectionCards } from "./section-cards";

afterEach(cleanup);

// The allowed list of an Environment, with the values that must never show.
const allowedFromEnvironment = [
  {
    name: "sqs",
    label: "SQS access",
    value: "arn:aws:iam::aws:policy/AmazonSQSFullAccess",
  },
  {
    name: "s3",
    label: "S3 read",
    value: "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess",
  },
  {
    name: "sns",
    label: "SNS publish",
    value: {
      role: "roles/pubsub.publisher",
      resource: "projects/acme/topics/{{ .namespace }}-*",
    },
  },
];

const awsRole = (value?: unknown): OptionView => ({
  registration: "aws-connectors-role",
  label: "Role for each cluster",
  isDefault: false,
  parameters: [
    {
      name: "policies",
      label: "Policies",
      schema: { type: "array", items: { type: "string" } },
      claimEditable: true,
      allowed: allowedFromEnvironment,
      value,
    },
    {
      name: "boundaryPolicies",
      label: "Admin policies",
      claimEditable: false,
      summary: "3 policies",
    },
    {
      name: "retentionDays",
      label: "Retention in days",
      schema: { type: "integer", minimum: 1, maximum: 365 },
      claimEditable: true,
      value: 30,
    },
  ],
});

const section = (
  overrides: Partial<SectionView> = {},
  option = awsRole(),
): SectionView => ({
  name: "connectors-permissions",
  label: "Connectors permissions",
  selection: "One",
  emptyOptionLabel: "No cloud permissions",
  required: false,
  options: [{ ...option, isDefault: true }],
  ...overrides,
});

describe("SectionCards", () => {
  it("renders an array parameter with allowed as checkboxes with the labels and submits the names", async () => {
    const onChange = vi.fn();
    const { container } = render(
      <SectionCards section={section()} onChange={onChange} />,
    );
    const group = screen.getByRole("group", { name: "Policies" });
    const boxes = within(group).getAllByRole("checkbox");
    expect(
      boxes.map((b) => b.getAttribute("aria-label") ?? b.textContent),
    ).toEqual(["SQS access", "S3 read", "SNS publish"]);
    expect(boxes.every((b) => b.getAttribute("aria-checked") === "false")).toBe(
      true,
    );

    await userEvent.click(
      within(group).getByRole("checkbox", { name: "S3 read" }),
    );
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "SQS access" }),
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [
        {
          section: "connectors-permissions",
          registration: "aws-connectors-role",
          parameters: { policies: ["sqs", "s3"] },
        },
      ],
      [],
    );

    for (const choice of allowedFromEnvironment) {
      expect(container.innerHTML).not.toContain(
        typeof choice.value === "string" ? choice.value : choice.value.resource,
      );
    }
    expect(container.innerHTML).not.toContain("roles/pubsub.publisher");
  });

  it("preselects only the names that the environment value holds", () => {
    render(<SectionCards section={section({}, awsRole(["s3", "unknown"]))} />);
    const group = screen.getByRole("group", { name: "Policies" });
    const checked = within(group)
      .getAllByRole("checkbox")
      .filter((b) => b.getAttribute("aria-checked") === "true")
      .map((b) => b.getAttribute("aria-label"));
    expect(checked).toEqual(["S3 read"]);
  });

  it("hides the empty option when the section is required", () => {
    const { rerender } = render(<SectionCards section={section()} />);
    expect(
      screen.getByRole("radio", { name: /No cloud permissions/ }),
    ).toBeTruthy();
    rerender(<SectionCards section={section({ required: true })} />);
    expect(
      screen.queryByRole("radio", { name: /No cloud permissions/ }),
    ).toBeNull();
  });

  it("marks the default option and preselects it", () => {
    render(
      <SectionCards
        section={section({
          name: "backup-storage",
          label: "Backup storage",
          emptyOptionLabel: "No backups",
          options: [
            {
              registration: "s3",
              label: "New S3 bucket",
              isDefault: false,
              parameters: [],
            },
            {
              registration: "existing-object-storage",
              label: "Existing bucket",
              isDefault: true,
              parameters: [],
            },
          ],
        })}
      />,
    );
    const option = screen.getByRole("radio", {
      name: /Existing bucket/,
    }) as HTMLInputElement;
    expect(option.checked).toBe(true);
    expect(
      screen.getByRole("radio", { name: /Existing bucket/ }).closest("label")
        ?.textContent,
    ).toContain("Default");
    expect(
      screen.getByRole("radio", { name: /New S3 bucket/ }).closest("label")
        ?.textContent,
    ).not.toContain("Default");
  });

  it("shows the schema error of an editable parameter at its field", async () => {
    const onChange = vi.fn();
    render(<SectionCards section={section()} onChange={onChange} />);
    const input = screen.getByRole("spinbutton", { name: "Retention in days" });
    await userEvent.clear(input);
    await userEvent.type(input, "500");
    expect(input.getAttribute("aria-invalid")).toBe("true");
    const describedBy = input.getAttribute("aria-describedby")!;
    expect(document.getElementById(describedBy)?.textContent).toBe(
      "must be <= 365",
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "aws-connectors-role.retentionDays", message: "must be <= 365" },
    ]);
  });

  it("shows a locked parameter as its summary, set by the admin", () => {
    render(<SectionCards section={section()} />);
    const locked = screen.getByRole("group", { name: "Admin policies" });
    expect(locked.textContent).toContain("3 policies");
    expect(locked.textContent).toContain("Set by your admin");
  });

  it("reports a YAML parameter that does not parse", async () => {
    const onChange = vi.fn();
    const option: OptionView = {
      registration: "tuning",
      label: "Tuning",
      isDefault: true,
      parameters: [
        {
          name: "overrides",
          label: "Overrides",
          schema: { type: "object" },
          claimEditable: true,
        },
      ],
    };
    render(<SectionCards section={section({}, option)} onChange={onChange} />);
    // user-event reads "[" as the start of a key name, so "[[" types one "[".
    await userEvent.type(
      screen.getByRole("textbox", { name: "Overrides" }),
      "a: [[",
    );
    const [, errors] = onChange.mock.lastCall!;
    expect(errors).toEqual([
      { path: "tuning.overrides", message: expect.stringMatching(/^Line 1: /) },
    ]);
  });

  it("takes a new policy default and keeps a value that the user set", async () => {
    const onChange = vi.fn();
    const policy = (retention: number, bucket: string): OptionView => ({
      registration: "aws-s3",
      label: "New bucket",
      isDefault: true,
      parameters: [
        {
          name: "retentionDays",
          label: "Retention in days",
          schema: { type: "integer" },
          claimEditable: true,
          value: retention,
        },
        {
          name: "bucketClass",
          label: "Bucket class",
          schema: { type: "string" },
          claimEditable: true,
          value: bucket,
        },
      ],
    });
    const { rerender } = render(
      <SectionCards
        section={section({}, policy(30, "standard"))}
        onChange={onChange}
      />,
    );
    const bucket = screen.getByRole("textbox", { name: "Bucket class" });
    await userEvent.clear(bucket);
    await userEvent.type(bucket, "cold");
    rerender(
      <SectionCards
        section={section({}, policy(60, "archive"))}
        onChange={onChange}
      />,
    );
    expect(
      (
        screen.getByRole("spinbutton", {
          name: "Retention in days",
        }) as HTMLInputElement
      ).value,
    ).toBe("60");
    expect(onChange).toHaveBeenLastCalledWith(
      [
        {
          section: "connectors-permissions",
          registration: "aws-s3",
          parameters: { bucketClass: "cold" },
        },
      ],
      [],
    );
  });

  it("checks an empty list against the schema of an optional parameter", async () => {
    const onChange = vi.fn();
    const option: OptionView = {
      registration: "tuning",
      label: "Tuning",
      isDefault: true,
      parameters: [
        {
          name: "zones",
          label: "Zones",
          schema: { type: "array", items: { type: "string" }, minItems: 1 },
          claimEditable: true,
          value: ["a"],
        },
      ],
    };
    render(<SectionCards section={section({}, option)} onChange={onChange} />);
    const zones = screen.getByRole("textbox", { name: "Zones" });
    await userEvent.clear(zones);
    // user-event reads "[" as the start of a key name, so "[[" types one "[".
    await userEvent.type(zones, "[[]");
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "tuning.zones", message: "must NOT have fewer than 1 items" },
    ]);
  });

  it("drops a selected option that the new options no longer hold", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SectionCards section={section()} onChange={onChange} />,
    );
    expect(onChange.mock.lastCall![0]).toEqual([
      {
        section: "connectors-permissions",
        registration: "aws-connectors-role",
      },
    ]);
    rerender(
      <SectionCards
        section={section({
          options: [
            {
              registration: "other",
              label: "Other",
              isDefault: false,
              parameters: [],
            },
          ],
        })}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith([], []);
    expect(
      (
        screen.getByRole("radio", {
          name: /No cloud permissions/,
        }) as HTMLInputElement
      ).checked,
    ).toBe(true);
  });

  it("greys an option that cannot be chosen and says why", () => {
    render(
      <SectionCards
        section={section({
          options: [
            {
              registration: "aws-ebs",
              label: "Encrypted volumes",
              isDefault: false,
              disabledReason: "Needs an encryption key",
              parameters: [],
            },
          ],
        })}
      />,
    );
    const option = screen.getByRole("radio", {
      name: /Encrypted volumes/,
    }) as HTMLInputElement;
    expect(option.disabled).toBe(true);
    expect(option.closest("label")?.textContent).toContain(
      "Needs an encryption key",
    );
  });

  it("reports a required section without a selection on its first call, and clears it on a pick", async () => {
    const onChange = vi.fn();
    render(
      <SectionCards
        section={section({
          required: true,
          options: [
            {
              registration: "other",
              label: "Other",
              isDefault: false,
              parameters: [],
            },
          ],
        })}
        onChange={onChange}
      />,
    );
    expect(onChange.mock.calls[0]).toEqual([
      [],
      [{ path: "connectors-permissions", message: "Choose an option" }],
    ]);
    await userEvent.click(screen.getByRole("radio", { name: /Other/ }));
    expect(onChange).toHaveBeenLastCalledWith(
      [{ section: "connectors-permissions", registration: "other" }],
      [],
    );
  });

  it("sets the parameter of a choice from its card", async () => {
    const onChange = vi.fn();
    render(
      <SectionCards
        section={section({
          name: "secondary-storage",
          label: "Secondary storage",
          emptyOptionLabel: undefined,
          required: true,
          options: [
            {
              registration: "elasticsearch",
              label: "New Elasticsearch",
              isDefault: true,
              parameters: [
                {
                  name: "preset",
                  label: "Preset",
                  claimEditable: true,
                  value: "small",
                },
              ],
              choices: {
                parameter: "preset",
                items: [
                  { name: "small", specs: [{ label: "Nodes", value: "1" }] },
                  {
                    name: "large",
                    specs: [
                      { label: "Nodes", value: "3" },
                      { label: "Memory", value: "8 GiB" },
                    ],
                  },
                ],
              },
            },
          ],
        })}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /large/ }));
    expect(onChange).toHaveBeenLastCalledWith(
      [
        {
          section: "secondary-storage",
          registration: "elasticsearch",
          parameters: { preset: "large" },
        },
      ],
      [],
    );
  });
});
