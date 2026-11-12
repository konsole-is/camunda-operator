import { describe, expect, it } from "vitest";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { EditableParameter, OptionView, SectionView } from "@/lib/policy";
import {
  type StorageDraft,
  draftErrors,
  goneSections,
  picksOf,
  resolvedErrors,
  valueErrors,
  valueProblem,
} from "./checks";

const view = (over: Partial<SectionView> = {}): SectionView => ({
  name: "storage",
  label: "Storage",
  selection: "One",
  required: true,
  options: [
    {
      registration: "store",
      label: "Store",
      isDefault: true,
      parameters: [
        {
          name: "name",
          label: "Name",
          claimEditable: true,
          allowed: [{ name: "a", label: "a" }],
        },
      ],
    },
  ],
  ...over,
});

const store = {
  apiVersion: "plugins.camunda.io/v1alpha1",
  kind: "PluginRegistration",
  metadata: { name: "store" },
  spec: {
    sections: ["storage"],
    mode: "Optional",
    label: "Store",
    deletionPolicies: ["Delete"],
    serviceAccount: { name: "store", namespace: "plugins" },
    parameters: {
      type: "object",
      required: ["owner"],
      properties: { owner: { type: "string" } },
    },
  },
} as PluginRegistration;

const draftOf = (picks: StorageDraft["picks"]): StorageDraft => ({
  kind: "CamundaClusterClaim",
  picks,
  namespace: "team-a",
  environment: undefined,
  sections: [
    {
      apiVersion: "plugins.camunda.io/v1alpha1",
      kind: "PluginSection",
      metadata: { name: "storage" },
      spec: {
        appliesTo: ["CamundaClusterClaim"],
        label: "Storage",
        selection: "One",
      },
    },
  ],
  registrations: [store],
});

describe("picksOf", () => {
  it("counts the enabled default of a section that picks does not hold", () => {
    expect(picksOf({}, view())).toEqual([
      { section: "storage", registration: "store" },
    ]);
  });

  it("keeps a picked empty option", () => {
    expect(picksOf({ storage: [] }, view())).toEqual([]);
  });
});

describe("goneSections", () => {
  it("names a section with a pick that the sections do not offer to the kind", () => {
    const draft = draftOf({
      storage: [{ section: "storage", registration: "store" }],
      old: [{ section: "old", registration: "x" }],
      empty: [],
    });
    expect(goneSections(draft)).toEqual(["old"]);
    expect(
      goneSections({ ...draft, kind: "CamundaManagementClusterClaim" }),
    ).toEqual(["storage", "old"]);
  });
});

describe("draftErrors", () => {
  it("leaves a missing pick of an asked section to the flow", () => {
    const draft = draftOf({ storage: [] });
    expect(draftErrors(draft, [view()], {})).toEqual([
      "Choose an option for Storage.",
    ]);
    expect(draftErrors(draft, [view()], {}, ["storage"])).toEqual([]);
  });

  it("does not count the card errors of a section that views do not hold", () => {
    const errors = { gone: [{ path: "x/name", message: "required" }] };
    expect(draftErrors(draftOf({}), [view()], errors)).toEqual([]);
  });
});

describe("valueErrors", () => {
  it("names a gone value of a disabled option, and asks for nothing else of it", () => {
    const disabled = view({
      options: [
        {
          ...view().options[0],
          disabledReason: "No Store in team-a",
          parameters: [
            { ...view().options[0].parameters[0], required: true },
            {
              name: "size",
              label: "Size",
              claimEditable: true,
              required: true,
            },
          ],
        },
      ],
    });
    expect(
      valueErrors(
        draftOf({
          storage: [
            {
              section: "storage",
              registration: "store",
              parameters: { name: "b" },
            },
          ],
        }),
        [disabled],
      ),
    ).toEqual(["b is not available for Name of Store. Choose another."]);
  });

  const tiered = (value?: unknown) =>
    view({
      options: [
        {
          registration: "store",
          label: "Store",
          isDefault: true,
          parameters: [
            {
              name: "tier",
              label: "Tier",
              claimEditable: true,
              ...(value !== undefined && { value }),
              allowed: [{ name: "gold", label: "Gold" }],
            },
          ],
        },
      ],
    });
  const setting = (parameters?: Record<string, unknown>) =>
    draftOf({
      storage: [
        {
          section: "storage",
          registration: "store",
          ...(parameters && { parameters }),
        },
      ],
    });

  it("does not check a policy value that the claim does not set against the names", () => {
    expect(valueErrors(setting(), [tiered("silver")])).toEqual([]);
  });

  it("checks a value that the claim sets against the names", () => {
    expect(valueErrors(setting({ tier: "silver" }), [tiered()])).toEqual([
      "silver is not available for Tier of Store. Choose another.",
    ]);
  });

  it("asks to fix a value that the claim sets and that is no name", () => {
    expect(valueErrors(setting({ tier: { level: 1 } }), [tiered()])).toEqual([
      "Fix the Tier of Store.",
    ]);
  });

  it("takes a list of names as names", () => {
    expect(
      valueErrors(setting({ tier: ["gold", "bronze"] }), [tiered()]),
    ).toEqual(["bronze is not available for Tier of Store. Choose another."]);
  });
});

describe("valueProblem", () => {
  const option = (choices?: string[]): OptionView => ({
    registration: "store",
    label: "Store",
    isDefault: true,
    parameters: [],
    ...(choices && {
      choices: {
        parameter: "preset",
        items: choices.map((name) => ({ name })),
      },
    }),
  });
  const preset: EditableParameter = {
    name: "preset",
    label: "Preset",
    claimEditable: true,
    schema: { type: "string", minLength: 2 },
  };

  it("checks a value of the choices against its schema too, because the name is the value", () => {
    expect(valueProblem(option(["a", "small"]), preset, "a", true)).toEqual({
      kind: "schema",
      errors: [expect.objectContaining({ message: expect.any(String) })],
    });
  });

  it("refuses a name that the choices do not hold", () => {
    expect(valueProblem(option(["small"]), preset, "large", true)).toEqual({
      kind: "unknownName",
      name: "large",
    });
  });

  it("asks for a required value", () => {
    expect(
      valueProblem(option(), { ...preset, required: true }, "", false),
    ).toEqual({ kind: "required" });
  });
});

describe("valueErrors and an empty list of names", () => {
  const zones = (minItems?: number) =>
    view({
      options: [
        {
          registration: "store",
          label: "Store",
          isDefault: true,
          parameters: [
            {
              name: "zones",
              label: "Zones",
              claimEditable: true,
              allowed: [
                { name: "a", label: "a" },
                { name: "b", label: "b" },
              ],
              schema: {
                type: "array",
                items: { type: "string" },
                ...(minItems !== undefined && { minItems }),
              },
            },
          ],
        },
      ],
    });
  const draft = draftOf({
    storage: [
      { section: "storage", registration: "store", parameters: { zones: [] } },
    ],
  });

  it("refuses it when the schema asks for more items", () => {
    expect(valueErrors(draft, [zones(1)])).toEqual(["Fix the Zones of Store."]);
  });

  it("passes it when the schema allows it", () => {
    expect(valueErrors(draft, [zones()])).toEqual([]);
  });
});

describe("resolvedErrors", () => {
  const selections = [{ section: "storage", registration: "store" }];

  it("checks a request with the parameters that the claim controller adds", () => {
    const draft = draftOf({ storage: selections });
    expect(resolvedErrors(draft, [view()], selections)).toEqual([
      expect.stringMatching(/^The policy gives Store a value that its schema/),
    ]);
    expect(
      resolvedErrors(draft, [view()], selections, () => ({ owner: "me" })),
    ).toEqual([]);
  });

  it("leaves out a parameter whose value the draft does not know yet", () => {
    const draft = draftOf({ storage: selections });
    expect(
      resolvedErrors(draft, [view()], selections, () => ({
        owner: undefined,
      })),
    ).toEqual([]);
  });

  it("leaves out the policy value of such a parameter too", () => {
    const strict = {
      ...store,
      spec: {
        ...store.spec,
        parameterValues: { owner: "from-the-policy" },
        parameters: {
          type: "object",
          additionalProperties: false,
          required: ["owner"],
          properties: { owner: { type: "string" } },
        },
      },
    } as PluginRegistration;
    const draft = {
      ...draftOf({ storage: selections }),
      registrations: [strict],
    };
    expect(
      resolvedErrors(draft, [view()], selections, () => ({
        owner: undefined,
      })),
    ).toEqual([]);
  });

  describe("with a composed rule in the schema", () => {
    const composed = (rule: Record<string, unknown>) =>
      ({
        ...store,
        spec: {
          ...store.spec,
          parameters: {
            type: "object",
            properties: {
              owner: { type: "string" },
              size: { type: "integer" },
            },
            ...rule,
          },
        },
      }) as PluginRegistration;
    const errorsOf = (
      rule: Record<string, unknown>,
      owner: string | undefined,
    ) =>
      resolvedErrors(
        {
          ...draftOf({ storage: selections }),
          registrations: [composed(rule)],
        },
        [view()],
        selections,
        () => ({ owner }),
      );

    it.each([
      ["minProperties", { minProperties: 2 }],
      ["allOf", { allOf: [{ required: ["owner", "size"] }] }],
    ])(
      "skips the check of %s while the controller parameter is not known, and checks it in full once it is",
      (_, rule) => {
        expect(errorsOf(rule, undefined)).toEqual([]);
        expect(errorsOf(rule, "me")).toEqual([
          expect.stringMatching(
            /^The policy gives Store a value that its schema/,
          ),
        ]);
      },
    );
  });
});
