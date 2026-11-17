// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { ContractTab } from "./contract-tab";

afterEach(cleanup);

const spec = {
  baseUrl: "http://production-identity.platform:80",
  issuerUrl: "https://login.example.com/realms/camunda",
  issuerBackendUrl: "http://keycloak.platform:8080/auth/realms/camunda",
  authUrl:
    "https://login.example.com/realms/camunda/protocol/openid-connect/auth",
  tokenUrl:
    "https://login.example.com/realms/camunda/protocol/openid-connect/token",
  jwksUrl:
    "https://login.example.com/realms/camunda/protocol/openid-connect/certs",
  clientId: "optimize",
  audience: "optimize-api",
  clientSecretRef: {
    namespace: "platform",
    name: "production-optimize-client",
    key: "client-secret",
  },
};

const contractOf = (fields: Record<string, unknown> = spec) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "ManagementAuthConfig",
    metadata: { name: "production" },
    spec: fields,
  }) as unknown as Resource<"ManagementAuthConfig">;

const reader = (
  namespace: string,
  name: string,
  cluster: string,
  externalUrl?: string,
) =>
  ({
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaOptimize",
    metadata: { namespace, name },
    spec: {
      clusterRef: { name: cluster },
      managementAuthRef: "production",
      ...(externalUrl && { externalUrl }),
    },
  }) as unknown as Resource<"CamundaOptimize">;

// termValue gives the text of the description after the term of label.
function termValue(label: string): string | null | undefined {
  return screen.getByText(label, { selector: "dt" }).nextElementSibling
    ?.textContent;
}

const noReaders = "No Optimize reads this contract.";

describe("ContractTab", () => {
  it("shows every field of the contract", () => {
    render(
      <ContractTab name="production" contract={contractOf()} readers={[]} />,
    );
    expect(termValue("Name")).toBe("production");
    for (const [label, value] of [
      ["Base URL", spec.baseUrl],
      ["Issuer URL", spec.issuerUrl],
      ["Issuer URL inside the cluster", spec.issuerBackendUrl],
      ["Authorization URL", spec.authUrl],
      ["Token URL", spec.tokenUrl],
      ["JWKS URL", spec.jwksUrl],
      ["Client ID", "optimize"],
      ["Audience", "optimize-api"],
    ]) {
      expect(termValue(label)).toBe(value);
    }
  });

  it("shows the client secret as namespace, name, and key", () => {
    render(
      <ContractTab name="production" contract={contractOf()} readers={[]} />,
    );
    expect(termValue("Client secret namespace")).toBe("platform");
    expect(termValue("Client secret name")).toBe("production-optimize-client");
    expect(termValue("Client secret key")).toBe("client-secret");
  });

  it("says Not set for an unset issuer URL inside the cluster, with the note", () => {
    const rest: Record<string, unknown> = { ...spec };
    delete rest.issuerBackendUrl;
    render(
      <ContractTab
        name="production"
        contract={contractOf(rest)}
        readers={[]}
      />,
    );
    expect(termValue("Issuer URL inside the cluster")).toBe(
      "Not setOptimize uses the issuer URL.",
    );
  });

  it("says Not valid for a field of the wrong type, and does not throw", () => {
    render(
      <ContractTab
        name="production"
        contract={contractOf({ ...spec, clientId: 7, clientSecretRef: null })}
        readers={[]}
      />,
    );
    expect(termValue("Client ID")).toBe("Not valid");
    expect(termValue("Client secret name")).toBe("Not set");
  });

  it("lists the readers with their cluster", () => {
    render(
      <ContractTab
        name="production"
        contract={contractOf()}
        readers={[
          reader(
            "team-orders",
            "orders-optimize",
            "orders",
            "https://orders.example.com/optimize",
          ),
          reader("team-lab", "lab-optimize", "lab"),
        ]}
      />,
    );
    const table = screen.getByRole("table", { name: "Readers" });
    const orders = within(table)
      .getAllByRole("row")
      .find((r) => within(r).queryByText("orders-optimize")) as HTMLElement;
    expect(within(orders).getByText("team-orders")).toBeTruthy();
    expect(
      within(orders).getByRole("link", { name: "orders" }).getAttribute("href"),
    ).toBe("/fleet/clusters/team-orders/orders");
    expect(
      within(orders).getByText("https://orders.example.com/optimize"),
    ).toBeTruthy();
    const lab = within(table)
      .getAllByRole("row")
      .find((r) => within(r).queryByText("lab-optimize")) as HTMLElement;
    expect(within(lab).getByText("Not set")).toBeTruthy();
  });

  it("says when no Optimize reads the contract", () => {
    render(
      <ContractTab name="production" contract={contractOf()} readers={[]} />,
    );
    expect(screen.getByText(noReaders)).toBeTruthy();
  });

  it("does not say that no Optimize reads the contract while the Optimize list is not read", () => {
    render(
      <ContractTab
        name="production"
        contract={contractOf()}
        readers={[]}
        unread={["CamundaOptimize"]}
      />,
    );
    expect(screen.queryByText(noReaders)).toBeNull();
    expect(
      screen.getByText(
        "The fleet manager has no current list of CamundaOptimize. This tab can be incomplete or out of date.",
      ),
    ).toBeTruthy();
  });

  it("says when the plane has no contract name yet", () => {
    render(<ContractTab name={undefined} contract={undefined} readers={[]} />);
    expect(
      screen.getByText("This plane has not written its contract yet."),
    ).toBeTruthy();
  });

  it("says when the named contract does not exist", () => {
    render(<ContractTab name="production" contract={undefined} readers={[]} />);
    expect(
      screen.getByText("The contract production does not exist."),
    ).toBeTruthy();
  });

  it("does not say that the contract does not exist while the contracts or the plane are not read", () => {
    for (const unread of [
      ["ManagementAuthConfig"],
      ["CamundaManagementCluster"],
    ]) {
      render(
        <ContractTab
          name={unread[0] === "ManagementAuthConfig" ? "production" : undefined}
          contract={undefined}
          readers={[]}
          unread={unread}
        />,
      );
      expect(
        screen.queryByText("The contract production does not exist."),
      ).toBeNull();
      expect(
        screen.queryByText("This plane has not written its contract yet."),
      ).toBeNull();
      expect(
        screen.getByText(
          `The fleet manager has no current list of ${unread[0]}. This tab can be incomplete or out of date.`,
        ),
      ).toBeTruthy();
      cleanup();
    }
  });
});
