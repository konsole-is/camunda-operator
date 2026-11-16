import { describe, expect, it } from "vitest";
import { ENVIRONMENT_LABEL } from "@/lib/labels";
import { sectionFixtures } from "./fixtures";
import {
  emptyPreviewText,
  previewClaims,
  previewSection,
  templateErrors,
} from "./preview";

const sources = sectionFixtures();
const registration = (name: string) =>
  sources.registrations.find((r) => r.metadata.name === name)!;

const orders = {
  kind: "CamundaClusterClaim" as const,
  namespace: "team-a",
  name: "orders",
  plugins: [],
};

describe("previewClaims", () => {
  it("returns the claims of the namespaces of the environment that a section of the registration applies to, by namespace and name", () => {
    expect(
      previewClaims(sources, "production", "aws-connectors-role").map(
        (c) => `${c.namespace}/${c.name}`,
      ),
    ).toEqual(["team-a/billing", "team-a/orders"]);
  });

  it("leaves out a claim of a kind that no section of the registration applies to", () => {
    const s = sectionFixtures();
    s.namespaces.find(
      (n) => n.metadata?.name === "platform",
    )!.metadata!.labels = {
      [ENVIRONMENT_LABEL]: "production",
    };
    expect(
      previewClaims(s, "production", "aws-connectors-role").map((c) => c.name),
    ).toEqual(["billing", "orders"]);
    expect(
      previewClaims(s, "production", "gateway-public").map((c) => c.name),
    ).toEqual(["plane-a", "billing", "orders"]);
  });

  it("offers the example claim in the first namespace without a claim", () => {
    expect(previewClaims(sources, "staging", "aws-connectors-role")).toEqual([
      {
        kind: "CamundaClusterClaim",
        namespace: "team-c",
        name: "example",
        plugins: [],
        example: true,
      },
    ]);
  });

  it("offers no claim without a namespace", () => {
    expect(previewClaims(sources, "lab", "aws-connectors-role")).toEqual([]);
    expect(previewClaims(sources, "", "aws-connectors-role")).toEqual([]);
  });
});

describe("emptyPreviewText", () => {
  it("asks for a namespace when the environment has none", () => {
    expect(emptyPreviewText(sources, "lab")).toBe(
      "Add a namespace first. The preview renders the template for a claim of the environment.",
    );
  });

  it("says that no section applies when the environment has namespaces", () => {
    const s = sectionFixtures();
    s.registrations = s.registrations.map((r) =>
      r.metadata.name === "aws-connectors-role"
        ? { ...r, spec: { ...r.spec, sections: ["missing"] } }
        : r,
    );
    expect(previewClaims(s, "production", "aws-connectors-role")).toEqual([]);
    expect(emptyPreviewText(s, "production")).toBe(
      "No section of this option applies to a claim, so the preview has no claim to render for.",
    );
  });
});

describe("templateErrors", () => {
  it("names each string of an object that does not parse, by its path", () => {
    expect(
      templateErrors({
        Statement: [
          { Resource: "{{ .claimName }}", Action: "{{ .outputs.x }}" },
        ],
        Version: "2012-10-17",
      }),
    ).toEqual([
      {
        path: "Statement[0].Resource",
        message: "unknown placeholder .claimName",
      },
      {
        path: "Statement[0].Action",
        message:
          "the placeholder .outputs.x is not allowed: a template parameter cannot read the outputs",
      },
    ]);
  });

  it("names the error of a string value with an empty path", () => {
    expect(templateErrors("{{ .pods }}")).toEqual([
      {
        path: "",
        message: "the placeholder .pods is allowed only in a metric query",
      },
    ]);
  });

  it("has no error for a template that parses", () => {
    expect(
      templateErrors({ a: "{{ .namespace }}-{{ .parameters.x }}" }),
    ).toEqual([]);
  });
});

describe("previewSection", () => {
  const twoSections = {
    ...registration("aws-connectors-role"),
    spec: {
      ...registration("aws-connectors-role").spec,
      sections: ["encryption-key", "connectors-permissions"],
    },
  };
  const claim = { ...orders, plugins: [] };

  it("takes the section of the selection of the claim", () => {
    expect(
      previewSection(
        {
          ...claim,
          plugins: [
            {
              section: "connectors-permissions",
              registration: "aws-connectors-role",
            },
          ],
        },
        twoSections,
        sources.sections,
        {
          sections: { "encryption-key": { allowed: ["aws-connectors-role"] } },
        },
      )?.metadata.name,
    ).toBe("connectors-permissions");
  });

  it("takes a section that allows the option when the claim does not select it", () => {
    expect(
      previewSection(claim, twoSections, sources.sections, {
        sections: {
          "connectors-permissions": { allowed: ["aws-connectors-role"] },
        },
      })?.metadata.name,
    ).toBe("connectors-permissions");
  });

  it("gives no section when no section of the option applies to the claim", () => {
    expect(
      previewSection(
        { ...claim, kind: "CamundaManagementClusterClaim" },
        twoSections,
        sources.sections,
        {},
      ),
    ).toBeUndefined();
  });
});
