import { describe, expect, it } from "vitest";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { SectionView } from "@/lib/policy";
import {
  type StorageDraft,
  draftErrors,
  goneSections,
  picksOf,
  resolvedErrors,
  valueErrors,
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
});
