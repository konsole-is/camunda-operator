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
    const names = boxes.map(
      (b) => b.getAttribute("aria-label") ?? b.textContent,
    );
    expect(names).toEqual(["SQS access", "S3 read", "SNS publish"]);
    expect(boxes.every((b) => b.getAttribute("aria-checked") === "false")).toBe(
      true,
    );

    await userEvent.click(boxes[names.indexOf("S3 read")]);
    await userEvent.click(boxes[names.indexOf("SQS access")]);
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
      { path: "aws-connectors-role/retentionDays", message: "must be <= 365" },
    ]);
  });

  it("shows a locked parameter as its summary, set by the admin", () => {
    render(<SectionCards section={section()} />);
    const locked = screen.getByRole("group", { name: "Admin policies" });
    expect(locked.textContent).toContain("3 policies");
    expect(locked.textContent).toContain("Set by your admin");
  });

  it("forgets a YAML error when new options remove its option", async () => {
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
    const { rerender } = render(
      <SectionCards section={section({}, option)} onChange={onChange} />,
    );
    // user-event reads "[" as the start of a key name, so "[[" types one "[".
    await userEvent.type(
      screen.getByRole("textbox", { name: "Overrides" }),
      "a: [[",
    );
    rerender(
      <SectionCards section={section({ options: [] })} onChange={onChange} />,
    );
    rerender(
      <SectionCards section={section({}, option)} onChange={onChange} />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /Tuning/ }));
    expect(onChange.mock.lastCall![1]).toEqual([]);
  });

  it("forgets a YAML error when its option is chosen again", async () => {
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
    await userEvent.click(
      screen.getByRole("radio", { name: /No cloud permissions/ }),
    );
    await userEvent.click(screen.getByRole("radio", { name: /Tuning/ }));
    expect(onChange.mock.lastCall![1]).toEqual([]);
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
      { path: "tuning/overrides", message: expect.stringMatching(/^Line 1: /) },
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

  it("shows a new policy default in a YAML field that nobody edited", () => {
    const policy = (value: unknown): OptionView => ({
      registration: "tuning",
      label: "Tuning",
      isDefault: true,
      parameters: [
        {
          name: "overrides",
          label: "Overrides",
          schema: { type: "object" },
          claimEditable: true,
          value,
        },
      ],
    });
    const { rerender } = render(
      <SectionCards section={section({}, policy({ a: 1 }))} />,
    );
    rerender(<SectionCards section={section({}, policy({ a: 2 }))} />);
    expect(
      (
        screen.getByRole("textbox", {
          name: "Overrides",
        }) as HTMLTextAreaElement
      ).value,
    ).toBe("a: 2\n");
  });

  it("shows the required error of a cleared YAML parameter at its field", async () => {
    const option: OptionView = {
      registration: "tuning",
      label: "Tuning",
      isDefault: true,
      parameters: [
        {
          name: "overrides",
          label: "Overrides",
          schema: { type: "object" },
          required: true,
          claimEditable: true,
          value: { a: 1 },
        },
      ],
    };
    render(<SectionCards section={section({}, option)} />);
    const box = screen.getByRole("textbox", { name: "Overrides" });
    await userEvent.clear(box);
    expect(box.getAttribute("aria-invalid")).toBe("true");
    expect(
      document.getElementById(box.getAttribute("aria-describedby")!)
        ?.textContent,
    ).toBe("is required");
  });

  it.each([
    [
      "more than maxItems",
      { maxItems: 1 },
      ["S3 read", "SQS access"],
      "Choose at most 1",
    ],
    ["fewer than minItems", { minItems: 1 }, [], "Choose at least 1"],
  ])("reports an allowed list with %s", async (_, bounds, clicks, message) => {
    const onChange = vi.fn();
    const option = awsRole(["s3"]);
    option.parameters = option.parameters.map((rule) =>
      rule.name === "policies"
        ? {
            ...rule,
            schema: { type: "array", items: { type: "string" }, ...bounds },
          }
        : rule,
    );
    render(<SectionCards section={section({}, option)} onChange={onChange} />);
    const group = screen.getByRole("group", { name: "Policies" });
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "S3 read" }),
    );
    for (const name of clicks)
      await userEvent.click(within(group).getByRole("checkbox", { name }));
    expect(onChange.mock.lastCall![1]).toEqual([
      { path: "aws-connectors-role/policies", message },
    ]);
  });

  it("describes an allowed list by its error when a choice is named error", async () => {
    const option = awsRole(["s3"]);
    option.parameters = option.parameters.map((rule) =>
      rule.name === "policies"
        ? {
            ...rule,
            schema: { type: "array", items: { type: "string" }, minItems: 1 },
            allowed: [
              { name: "error", label: "Error log" },
              { name: "s3", label: "S3 read" },
            ],
          }
        : rule,
    );
    render(<SectionCards section={section({}, option)} />);
    const group = screen.getByRole("group", { name: "Policies" });
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "S3 read" }),
    );
    expect(
      document.getElementById(group.getAttribute("aria-describedby")!)
        ?.textContent,
    ).toBe("Choose at least 1");
  });

  describe("an empty string", () => {
    const option = (
      schema: Record<string, unknown>,
      value: string,
    ): OptionView => ({
      registration: "naming",
      label: "Naming",
      isDefault: true,
      parameters: [
        { name: "prefix", label: "Prefix", schema, claimEditable: true, value },
      ],
    });

    it("keeps a cleared inherited string as an empty override", async () => {
      const onChange = vi.fn();
      render(
        <SectionCards
          section={section({}, option({ type: "string" }, "team-"))}
          onChange={onChange}
        />,
      );
      await userEvent.clear(screen.getByRole("textbox", { name: "Prefix" }));
      expect(onChange).toHaveBeenLastCalledWith(
        [
          {
            section: "connectors-permissions",
            registration: "naming",
            parameters: { prefix: "" },
          },
        ],
        [],
      );
    });

    it("checks an empty string against the schema", async () => {
      const onChange = vi.fn();
      render(
        <SectionCards
          section={section(
            {},
            option({ type: "string", minLength: 1 }, "team-"),
          )}
          onChange={onChange}
        />,
      );
      await userEvent.clear(screen.getByRole("textbox", { name: "Prefix" }));
      expect(onChange.mock.lastCall![1]).toEqual([
        {
          path: "naming/prefix",
          message: "must NOT have fewer than 1 characters",
        },
      ]);
    });

    it("lets a select pick an empty enum value", async () => {
      const onChange = vi.fn();
      render(
        <SectionCards
          section={section(
            {},
            option({ type: "string", enum: ["", "enabled"] }, "enabled"),
          )}
          onChange={onChange}
        />,
      );
      await userEvent.selectOptions(
        screen.getByRole("combobox", { name: "Prefix" }),
        "Empty",
      );
      expect(onChange.mock.lastCall![0]).toEqual([
        {
          section: "connectors-permissions",
          registration: "naming",
          parameters: { prefix: "" },
        },
      ]);
    });
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
      { path: "tuning/zones", message: "must NOT have fewer than 1 items" },
    ]);
  });

  it("reports a picked name that the new allowed list no longer holds", async () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SectionCards section={section()} onChange={onChange} />,
    );
    const group = screen.getByRole("group", { name: "Policies" });
    await userEvent.click(
      within(group).getByRole("checkbox", { name: "S3 read" }),
    );
    const narrowed = awsRole();
    narrowed.parameters = narrowed.parameters.map((rule) =>
      rule.name === "policies" && rule.claimEditable
        ? {
            ...rule,
            allowed: allowedFromEnvironment.filter((a) => a.name !== "s3"),
          }
        : rule,
    );
    rerender(
      <SectionCards section={section({}, narrowed)} onChange={onChange} />,
    );
    expect(onChange.mock.lastCall![1]).toEqual([
      {
        path: "aws-connectors-role/policies",
        message: "s3 is not an allowed choice",
      },
    ]);
  });

  it("drops a selected option that new options disable", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <SectionCards
        section={section({ required: true })}
        onChange={onChange}
      />,
    );
    rerender(
      <SectionCards
        section={section(
          { required: true },
          { ...awsRole(), disabledReason: "Needs an IAM role" },
        )}
        onChange={onChange}
      />,
    );
    expect(onChange).toHaveBeenLastCalledWith(
      [],
      [{ path: "connectors-permissions", message: "Choose an option" }],
    );
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

  it("reports a picked card that a refresh removed", async () => {
    const onChange = vi.fn();
    const storage = (names: string[]): SectionView =>
      section({
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
              items: names.map((name) => ({ name })),
            },
          },
        ],
      });
    const { rerender } = render(
      <SectionCards
        section={storage(["small", "large"])}
        onChange={onChange}
      />,
    );
    await userEvent.click(screen.getByRole("radio", { name: /large/ }));
    rerender(<SectionCards section={storage(["small"])} onChange={onChange} />);
    expect(onChange.mock.lastCall![1]).toEqual([
      {
        path: "elasticsearch/preset",
        message: "large is not an available choice",
      },
    ]);
    const cards = screen.getByRole("group", { name: "Preset" });
    expect(
      document.getElementById(cards.getAttribute("aria-describedby")!)
        ?.textContent,
    ).toBe("large is not an available choice");
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

  describe("registrations whose names start the same", () => {
    // A number parameter of one registration, invalid above 365.
    const daysOption = (
      registration: string,
      label: string,
      value: unknown = 30,
      name = "b",
    ): OptionView => ({
      registration,
      label: `Option ${registration}`,
      isDefault: false,
      parameters: [
        {
          name,
          label,
          schema: { type: "integer", minimum: 1, maximum: 365 },
          claimEditable: true,
          value,
        },
      ],
    });
    const prefixedOptions = (...options: OptionView[]) =>
      section({ selection: "Many", options });
    const picked = (...registrations: string[]) =>
      registrations.map((registration) => ({
        section: "connectors-permissions",
        registration,
      }));

    it("shows the error of a parameter only under its own registration", async () => {
      const onChange = vi.fn();
      render(
        <SectionCards
          section={prefixedOptions(
            daysOption("a", "Days of a"),
            daysOption("a.b", "Days of a.b"),
          )}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      const inAB = screen.getByRole("spinbutton", { name: "Days of a.b" });
      await userEvent.clear(inAB);
      await userEvent.type(inAB, "500");
      expect(inAB.getAttribute("aria-invalid")).toBe("true");
      expect(
        screen
          .getByRole("spinbutton", { name: "Days of a" })
          .getAttribute("aria-invalid"),
      ).toBeNull();
      expect(onChange.mock.lastCall![1]).toEqual([
        { path: "a.b/b", message: "must be <= 365" },
      ]);
    });

    it("leaves a.b untouched when a parameter of a is edited", async () => {
      const onChange = vi.fn();
      const options = (defaultOfAB: number) =>
        prefixedOptions(
          daysOption("a", "Days of a"),
          daysOption("a.b", "Days of a.b", defaultOfAB),
        );
      const { rerender } = render(
        <SectionCards
          section={options(500)}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      const inA = screen.getByRole("spinbutton", { name: "Days of a" });
      await userEvent.clear(inA);
      await userEvent.type(inA, "40");
      // The page gets the error of the policy value, but nobody edited the field.
      expect(onChange.mock.lastCall![1]).toEqual([
        { path: "a.b/b", message: "must be <= 365" },
      ]);
      expect(
        screen
          .getByRole("spinbutton", { name: "Days of a.b" })
          .getAttribute("aria-invalid"),
      ).toBeNull();
      rerender(
        <SectionCards
          section={options(200)}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      expect(
        (
          screen.getByRole("spinbutton", {
            name: "Days of a.b",
          }) as HTMLInputElement
        ).value,
      ).toBe("200");
      expect(
        (
          screen.getByRole("spinbutton", {
            name: "Days of a",
          }) as HTMLInputElement
        ).value,
      ).toBe("40");
    });

    it("follows a new policy default in a parameter whose registration and name join like an edited one", async () => {
      const onChange = vi.fn();
      const options = (start: number) =>
        prefixedOptions(
          daysOption("a", "Days of a", start, "b.c"),
          daysOption("a.b", "Days of a.b", start, "c"),
        );
      const { rerender } = render(
        <SectionCards
          section={options(30)}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      const inAB = screen.getByRole("spinbutton", { name: "Days of a.b" });
      await userEvent.clear(inAB);
      await userEvent.type(inAB, "40");
      rerender(
        <SectionCards
          section={options(60)}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      expect(
        (
          screen.getByRole("spinbutton", {
            name: "Days of a",
          }) as HTMLInputElement
        ).value,
      ).toBe("60");
      expect(
        (
          screen.getByRole("spinbutton", {
            name: "Days of a.b",
          }) as HTMLInputElement
        ).value,
      ).toBe("40");
    });

    it("forgets the YAML error of a.b when a.b leaves the selection and a stays", async () => {
      const onChange = vi.fn();
      const yamlOption = (registration: string): OptionView => ({
        registration,
        label: `Option ${registration}`,
        isDefault: false,
        parameters: [
          {
            name: "overrides",
            label: `Overrides of ${registration}`,
            schema: { type: "object" },
            claimEditable: true,
          },
        ],
      });
      render(
        <SectionCards
          section={prefixedOptions(yamlOption("a"), yamlOption("a.b"))}
          defaultValue={picked("a", "a.b")}
          onChange={onChange}
        />,
      );
      // user-event reads "[" as the start of a key name, so "[[" types one "[".
      await userEvent.type(
        screen.getByRole("textbox", { name: "Overrides of a.b" }),
        "a: [[",
      );
      await userEvent.click(
        screen.getByRole("checkbox", { name: "Option a.b" }),
      );
      await userEvent.click(
        screen.getByRole("checkbox", { name: "Option a.b" }),
      );
      expect(onChange.mock.lastCall![1]).toEqual([]);
    });

    it("reports a schema error inside an object parameter under the registration and the parameter", async () => {
      const onChange = vi.fn();
      const option: OptionView = {
        registration: "a.b",
        label: "Option a.b",
        isDefault: false,
        parameters: [
          {
            name: "overrides",
            label: "Overrides",
            schema: {
              type: "object",
              required: ["days"],
              properties: { days: { type: "integer" } },
            },
            claimEditable: true,
          },
        ],
      };
      render(
        <SectionCards
          section={prefixedOptions(option)}
          defaultValue={picked("a.b")}
          onChange={onChange}
        />,
      );
      await userEvent.type(
        screen.getByRole("textbox", { name: "Overrides" }),
        "x: 1",
      );
      expect(onChange.mock.lastCall![1]).toEqual([
        { path: "a.b/overrides.days", message: "is required" },
      ]);
    });

    it("keeps a parameter named constructor working after a sibling parameter is edited", async () => {
      const onChange = vi.fn();
      const option: OptionView = {
        ...daysOption("a", "Days of a"),
        parameters: [
          ...daysOption("a", "Days of a").parameters,
          ...daysOption("a", "Constructor of a", 30, "constructor").parameters,
        ],
      };
      render(
        <SectionCards
          section={prefixedOptions(option)}
          defaultValue={picked("a")}
          onChange={onChange}
        />,
      );
      const inA = screen.getByRole("spinbutton", { name: "Days of a" });
      await userEvent.clear(inA);
      await userEvent.type(inA, "40");
      expect(onChange.mock.lastCall![1]).toEqual([]);
    });

    it("keeps a registration named constructor working", () => {
      const onChange = vi.fn();
      render(
        <SectionCards
          section={prefixedOptions(
            daysOption("constructor", "Keys of constructor", 30, "keys"),
          )}
          defaultValue={picked("constructor")}
          onChange={onChange}
        />,
      );
      expect(onChange.mock.lastCall![1]).toEqual([]);
    });

    it("names each field by its own label when the registration and parameter names join with a dash", () => {
      render(
        <SectionCards
          section={prefixedOptions(
            daysOption("a", "Days of a", 30, "b-x"),
            daysOption("a-b", "Days of a-b", 30, "x"),
          )}
          defaultValue={picked("a", "a-b")}
        />,
      );
      const inA = screen.getByRole("spinbutton", { name: "Days of a" });
      const inAB = screen.getByRole("spinbutton", { name: "Days of a-b" });
      expect(inA).not.toBe(inAB);
    });
  });

  describe("rules of the schema", () => {
    const ruled: OptionView = {
      registration: "team-bucket",
      label: "Team bucket",
      isDefault: false,
      schema: {
        type: "object",
        "x-kubernetes-validations": [
          {
            rule: "self.min <= self.max",
            message: "min must not be above max",
          },
        ],
      },
      parameters: [
        {
          name: "name",
          label: "Bucket name",
          schema: {
            type: "string",
            "x-kubernetes-validations": [
              {
                rule: "self.startsWith('team-')",
                message: "name must start with team-",
              },
            ],
          },
          claimEditable: true,
          value: "team-a",
        },
        {
          name: "limits",
          label: "Limits",
          schema: {
            type: "object",
            properties: {
              cpu: {
                type: "integer",
                "x-kubernetes-validations": [{ rule: "self > 0" }],
              },
            },
          },
          claimEditable: true,
        },
        {
          name: "bucket",
          label: "Shared bucket",
          schema: {
            type: "string",
            "x-kubernetes-validations": [
              { rule: "self != ''", message: "bucket must not be empty" },
            ],
          },
          claimEditable: false,
          summary: "shared",
        },
      ],
    };

    // The text of each element that describes the field.
    const descriptionOf = (field: HTMLElement) =>
      (field.getAttribute("aria-describedby") ?? "")
        .split(" ")
        .map((id) => document.getElementById(id)?.textContent ?? "")
        .join("\n");

    it("shows the rule of a parameter under its field, checked after the save", () => {
      render(<SectionCards section={section({}, ruled)} />);

      const description = descriptionOf(
        screen.getByRole("textbox", { name: "Bucket name" }),
      );

      expect(description).toContain("name must start with team-");
      expect(description).toContain("Checked after you save");
    });

    it("shows the rule text when a rule has no message, with its path inside the parameter", () => {
      render(<SectionCards section={section({}, ruled)} />);

      const description = descriptionOf(
        screen.getByRole("textbox", { name: "Limits" }),
      );

      expect(description).toContain("cpu");
      expect(description).toContain("self > 0");
    });

    it("shows the rules of the registration that span parameters under the option", async () => {
      render(
        <SectionCards
          section={section({ options: [{ ...ruled, isDefault: false }] })}
        />,
      );
      expect(screen.queryByText(/min must not be above max/)).toBeNull();

      await userEvent.click(screen.getByRole("radio", { name: /Team bucket/ }));

      expect(screen.getByText(/min must not be above max/)).toBeTruthy();
    });

    it("shows no rule for a locked parameter", () => {
      render(<SectionCards section={section({}, ruled)} />);

      const locked = screen.getByRole("group", { name: "Shared bucket" });

      expect(locked.textContent).not.toContain("Checked after you save");
      expect(screen.queryByText(/bucket must not be empty/)).toBeNull();
    });

    it("reports no error for a rule, because the app does not check it", async () => {
      const onChange = vi.fn();
      render(<SectionCards section={section({}, ruled)} onChange={onChange} />);
      const input = screen.getByRole("textbox", { name: "Bucket name" });

      await userEvent.clear(input);
      await userEvent.type(input, "a");

      expect(onChange.mock.lastCall![1]).toEqual([]);
      expect(input.getAttribute("aria-invalid")).toBeNull();
    });
  });
});
