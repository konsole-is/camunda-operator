// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { OptimizeTab } from "./optimize-tab";
import type { OptimizeRow } from "./plane-optimize";

afterEach(cleanup);

const row: OptimizeRow = {
  namespace: "team-orders",
  name: "orders-optimize",
  externalUrl: "https://orders.example.com/optimize",
  callbackUrl:
    "https://orders.example.com/optimize/api/authentication/callback",
};
const row2: OptimizeRow = {
  namespace: "team-billing",
  name: "billing-optimize",
  externalUrl: "https://billing.example.com/optimize",
  callbackUrl:
    "https://billing.example.com/optimize/api/authentication/callback",
};
const healthy = {
  type: "OptimizeCallbacksReady",
  status: "True",
  reason: "Healthy",
};
const empty =
  "No Optimize with an external URL reads the contract of this plane.";
const oidcSentence = "Add the callbacks to your identity provider.";

const table = () => screen.getByRole("table", { name: "Optimize" });

describe("OptimizeTab", () => {
  it("shows namespace, name, external URL, and callback URL", () => {
    render(<OptimizeTab rows={[row]} callbacks={healthy} oidc={false} />);
    for (const column of [
      "Namespace",
      "Name",
      "External URL",
      "Callback URL",
    ]) {
      expect(
        within(table()).getByRole("columnheader", { name: column }),
      ).toBeTruthy();
    }
    expect(within(table()).getByText("team-orders")).toBeTruthy();
    expect(within(table()).getByText("orders-optimize")).toBeTruthy();
    expect(
      within(table())
        .getByRole("link", { name: "https://orders.example.com/optimize" })
        .getAttribute("href"),
    ).toBe("https://orders.example.com/optimize");
    expect(
      within(table()).getByText(
        "https://orders.example.com/optimize/api/authentication/callback",
      ),
    ).toBeTruthy();
  });

  it("shows the state of OptimizeCallbacksReady once, above the table, and no state in a row", () => {
    render(
      <OptimizeTab
        rows={[row, row2]}
        callbacks={{
          type: "OptimizeCallbacksReady",
          status: "False",
          reason: "ConnectionFailed",
          message: "Keycloak did not answer",
        }}
        oidc={false}
      />,
    );
    const states = screen.getAllByText(/ConnectionFailed/);
    expect(states).toHaveLength(1);
    expect(screen.getByText("Keycloak did not answer")).toBeTruthy();
    expect(within(table()).queryByText(/Ready|ConnectionFailed/)).toBeNull();
    expect(
      states[0].compareDocumentPosition(table()) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("says in OIDC mode to add the callbacks to the identity provider", () => {
    render(
      <OptimizeTab
        rows={[row]}
        callbacks={{ ...healthy, reason: "Disabled" }}
        oidc
      />,
    );
    expect(screen.getByText(oidcSentence)).toBeTruthy();
  });

  it("does not say it in a Keycloak mode", () => {
    render(<OptimizeTab rows={[row]} callbacks={healthy} oidc={false} />);
    expect(screen.queryByText(oidcSentence)).toBeNull();
  });

  it("says when no Optimize with an external URL reads the contract", () => {
    render(
      <OptimizeTab
        rows={[]}
        callbacks={{ ...healthy, reason: "NoCallbacks" }}
        oidc={false}
      />,
    );
    expect(screen.getByText(empty)).toBeTruthy();
    expect(screen.queryByRole("table")).toBeNull();
  });

  it("does not say there is no Optimize while a source is not read, and names the kind", () => {
    render(
      <OptimizeTab
        rows={[]}
        callbacks={undefined}
        oidc
        unread={["CamundaOptimize"]}
      />,
    );
    expect(screen.queryByText(empty)).toBeNull();
    expect(
      screen.getByText(
        "The page could not read CamundaOptimize, so this tab can be incomplete or out of date.",
      ),
    ).toBeTruthy();
  });

  it("says that an external URL that is not a string is not valid", () => {
    render(
      <OptimizeTab
        rows={[{ namespace: "team-odd", name: "odd-optimize" }]}
        callbacks={healthy}
        oidc
      />,
    );
    expect(within(table()).getAllByText("Not a valid URL")).toHaveLength(2);
  });
});
