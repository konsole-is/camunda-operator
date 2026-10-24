import { describe, expect, it } from "vitest";
import type { PluginSection } from "@/generated/crds/pluginsection";
import { checkAllowed } from "./allowed";

describe("checkAllowed with a registration named after a key of Object", () => {
  it("refuses a parameter of a registration that the snapshot has no parameters for", () => {
    const got = checkAllowed({
      policy: {
        sections: { dns: { allowed: ["constructor"] } },
        parameters: { other: {} },
      },
      selections: [
        {
          section: "dns",
          registration: "constructor",
          parameters: { name: "zone" },
        },
      ],
    });

    expect(got).toEqual({
      refusal: {
        reason: "NotAllowed",
        message:
          'parameter "name" of registration "constructor" is not claim-editable',
      },
    });
  });
});

describe("checkAllowed with sections", () => {
  it("needs the claim kind when it gets the sections", () => {
    const sections = [
      {
        metadata: { name: "dns" },
        spec: {
          label: "DNS",
          selection: "One",
          appliesTo: ["CamundaClusterClaim"],
        },
      } as PluginSection,
    ];

    // @ts-expect-error: sections without kind
    checkAllowed({ policy: {}, sections });

    expect(
      checkAllowed({
        kind: "CamundaManagementClusterClaim",
        policy: { sections: { dns: { allowed: ["route53"] } } },
        sections,
        selections: [{ section: "dns", registration: "route53" }],
      }),
    ).toEqual({
      refusal: {
        reason: "NotAllowed",
        message:
          'section "dns" does not apply to a CamundaManagementClusterClaim',
      },
    });
  });
});
