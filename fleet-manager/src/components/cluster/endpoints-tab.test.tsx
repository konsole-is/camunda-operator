// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { EndpointRow } from "./endpoints";
import { EndpointsTab } from "./endpoints-tab";

afterEach(cleanup);

const operate: EndpointRow = {
  key: "CamundaCluster/operate",
  owner: "CamundaCluster",
  name: "operate",
  url: "https://orders.example.com/operate",
  service: "orders-gateway",
  port: 8080,
  protocol: "HTTP",
  servedBy: { label: "Gateway", embedded: true },
};
const grpc: EndpointRow = {
  key: "CamundaCluster/grpc",
  owner: "CamundaCluster",
  name: "grpc",
  url: "grpc.orders.example.com:443",
  service: "orders-gateway",
  port: 26500,
  protocol: "GRPC",
  servedBy: { label: "Gateway", embedded: false },
};

const rowOf = (name: string) =>
  within(screen.getByRole("table", { name: "Endpoints" })).getByRole("row", {
    name: new RegExp(`^${name}`),
  });

describe("EndpointsTab", () => {
  it("links an HTTP URL and shows a gRPC host as text", () => {
    render(<EndpointsTab rows={[operate, grpc]} />);
    expect(
      screen
        .getByRole("link", { name: "https://orders.example.com/operate" })
        .getAttribute("href"),
    ).toBe("https://orders.example.com/operate");
    expect(
      within(rowOf("grpc")).getByText("grpc.orders.example.com:443"),
    ).toBeTruthy();
    expect(
      screen.queryByRole("link", { name: "grpc.orders.example.com:443" }),
    ).toBeNull();
  });

  it("shows the Service with its port and the protocol", () => {
    render(<EndpointsTab rows={[grpc]} />);
    expect(
      within(rowOf("grpc")).getByText("orders-gateway:26500"),
    ).toBeTruthy();
    expect(within(rowOf("grpc")).getByText("GRPC")).toBeTruthy();
  });

  it("marks a route of an embedded app", () => {
    render(<EndpointsTab rows={[operate, grpc]} />);
    expect(within(rowOf("operate")).getByText("Gateway")).toBeTruthy();
    expect(within(rowOf("operate")).getByText("(embedded)")).toBeTruthy();
    expect(within(rowOf("grpc")).queryByText("(embedded)")).toBeNull();
  });

  it("says when a route has no public URL", () => {
    render(<EndpointsTab rows={[{ ...operate, url: undefined }]} />);
    expect(within(rowOf("operate")).getByText("No external URL")).toBeTruthy();
  });

  it("shows a dash for a route that no known process serves", () => {
    render(
      <EndpointsTab
        rows={[{ ...operate, name: "future", servedBy: undefined }]}
      />,
    );
    expect(within(rowOf("future")).getByText("-")).toBeTruthy();
  });

  it("says when the cluster reports no endpoints", () => {
    render(<EndpointsTab rows={[]} />);
    expect(
      screen.getByText("This cluster reports no endpoints yet."),
    ).toBeTruthy();
    expect(screen.queryByRole("table")).toBeNull();
  });
});
