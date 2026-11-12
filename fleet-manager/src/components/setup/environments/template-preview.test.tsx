// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { renderJSON, type TemplateData } from "@/lib/gotemplate";
import type { PreviewClaim } from "./preview";
import { TemplatePreview } from "./template-preview";

afterEach(cleanup);

const claims: PreviewClaim[] = [
  {
    kind: "CamundaClusterClaim",
    namespace: "team-a",
    name: "orders",
    plugins: [],
  },
  {
    kind: "CamundaClusterClaim",
    namespace: "team-b",
    name: "billing",
    plugins: [],
  },
];

const dataOf = (claim: PreviewClaim): TemplateData => ({
  namespace: claim.namespace,
  claim: claim.name,
  request: `${claim.name}-connectors-permissions`,
  serviceAccount: `${claim.name}-connectors`,
  parameters: {},
  outputs: {},
});

describe("TemplatePreview", () => {
  it("renders {{ .namespace }} and {{ .serviceAccount }} for the chosen cluster", () => {
    render(
      <TemplatePreview
        claims={claims}
        render={(claim) =>
          renderJSON(
            "{{ .namespace }}/{{ .serviceAccount }}",
            dataOf(claim),
            "parameter",
          )
        }
      />,
    );
    expect(screen.getByText("team-a/orders-connectors")).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Preview for"), {
      target: { value: "1" },
    });
    expect(screen.getByText("team-b/billing-connectors")).toBeTruthy();
  });

  it("renders an object parameter as an object", () => {
    const { container } = render(
      <TemplatePreview
        claims={claims}
        render={(claim) =>
          renderJSON(
            {
              Statement: [
                { Resource: "arn:aws:sqs:*:*:{{ .namespace }}-*", Max: 3 },
              ],
            },
            dataOf(claim),
            "parameter",
          )
        }
      />,
    );
    const output = container.querySelector("pre")?.textContent ?? "";
    expect(output).toContain("Resource: arn:aws:sqs:*:*:team-a-*");
    expect(output).toContain("Max: 3");
    expect(output).not.toContain("{");
  });

  it("shows the error of a string with its path", () => {
    render(
      <TemplatePreview
        claims={claims}
        render={(claim) =>
          renderJSON(
            { a: ["{{ .parameters.region }}"] },
            dataOf(claim),
            "parameter",
          )
        }
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe(
      'a[0]: parameter "region" has no value',
    );
  });

  it("names an example claim as an example", () => {
    render(
      <TemplatePreview
        claims={[{ ...claims[0], name: "example", example: true }]}
        render={() => ({ value: "x" })}
      />,
    );
    expect(
      screen.getByText(
        "No claim exists yet, so the preview uses the example claim example in team-a.",
      ),
    ).toBeTruthy();
  });

  it("shows Add a namespace first without a namespace", () => {
    render(<TemplatePreview claims={[]} render={() => ({ value: "x" })} />);
    expect(
      screen.getByText(
        "Add a namespace first. The preview renders the template for a claim of the environment.",
      ),
    ).toBeTruthy();
  });
});
