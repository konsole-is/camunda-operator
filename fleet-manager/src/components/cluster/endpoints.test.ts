import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { endpointRows } from "./endpoints";

type Endpoint = NonNullable<
  NonNullable<Resource<"CamundaCluster">["status"]>["endpoints"]
>[number];

const ENDPOINTS: Endpoint[] = [
  {
    name: "admin",
    pathPrefix: "/admin",
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  },
  {
    name: "connectors",
    pathPrefix: "/inbound",
    port: 8080,
    protocol: "HTTP",
    service: "orders-connectors",
  },
  {
    name: "default",
    pathPrefix: "/",
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  },
  { name: "grpc", port: 26500, protocol: "GRPC", service: "orders-gateway" },
  {
    name: "operate",
    pathPrefix: "/operate",
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  },
  {
    name: "rest",
    pathPrefix: "/v2",
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  },
  {
    name: "tasklist",
    pathPrefix: "/tasklist",
    port: 8080,
    protocol: "HTTP",
    service: "orders-gateway",
  },
];

function clusterWith(
  externalUrl: string | undefined,
  endpoints: Endpoint[] | null = ENDPOINTS,
): Resource<"CamundaCluster"> {
  return {
    kind: "CamundaCluster",
    metadata: { namespace: "team-a", name: "orders" },
    spec: { presetRef: "medium", externalUrl },
    status: endpoints === null ? undefined : { endpoints },
  } as unknown as Resource<"CamundaCluster">;
}

const cluster = clusterWith("https://orders.example.com");
const withExternalUrl = (url: string | undefined) => clusterWith(url);
const withEndpoint = (e: Endpoint) =>
  clusterWith("https://orders.example.com", [...ENDPOINTS, e]);
const withoutStatus = () => clusterWith("https://orders.example.com", null);

function optimize(
  clusterRef: string,
  externalUrl: string,
  pathPrefix: string,
): Resource<"CamundaOptimize"> {
  return {
    kind: "CamundaOptimize",
    metadata: { namespace: "team-a", name: `${clusterRef}-optimize` },
    spec: { clusterRef: { name: clusterRef }, externalUrl },
    status: {
      endpoints: [
        {
          name: "optimize",
          service: "orders-optimize",
          port: 8090,
          pathPrefix,
          protocol: "HTTP",
        },
      ],
    },
  } as unknown as Resource<"CamundaOptimize">;
}

const row = (rows: ReturnType<typeof endpointRows>, name: string) =>
  rows.find((r) => r.name === name);

describe("endpointRows", () => {
  it("serves an embedded Operate from the gateway", () => {
    const rows = endpointRows({ cluster, spec: {}, optimizes: [] });
    expect(row(rows, "operate")).toMatchObject({
      owner: "CamundaCluster",
      url: "https://orders.example.com/operate",
      service: "orders-gateway",
      port: 8080,
      protocol: "HTTP",
      servedBy: { label: "Gateway", embedded: true },
    });
  });

  it("serves a standalone Operate by itself", () => {
    const rows = endpointRows({
      cluster,
      spec: { operate: { mode: "Standalone" } },
      optimizes: [],
    });
    expect(row(rows, "operate")?.servedBy).toEqual({
      label: "Operate",
      embedded: false,
    });
  });

  it("serves the client routes from the brokers when the gateway is embedded", () => {
    const rows = endpointRows({
      cluster,
      spec: { gateway: { mode: "Embedded" } },
      optimizes: [],
    });
    for (const name of ["default", "rest", "grpc"]) {
      expect(row(rows, name)?.servedBy).toEqual({
        label: "Brokers",
        embedded: true,
      });
    }
  });

  it("serves the client routes from a standalone gateway", () => {
    const rows = endpointRows({ cluster, spec: {}, optimizes: [] });
    expect(row(rows, "rest")?.servedBy).toEqual({
      label: "Gateway",
      embedded: false,
    });
  });

  it("serves the connectors route from Connectors, also without a box", () => {
    const rows = endpointRows({ cluster, spec: {}, optimizes: [] });
    expect(row(rows, "connectors")?.servedBy).toEqual({
      label: "Connectors",
      embedded: false,
    });
  });

  it("joins the external URL and the path with one slash", () => {
    const rows = endpointRows({
      cluster: withExternalUrl("https://orders.example.com/"),
      spec: {},
      optimizes: [],
    });
    expect(row(rows, "default")?.url).toBe("https://orders.example.com/");
    expect(row(rows, "rest")?.url).toBe("https://orders.example.com/v2");
  });

  it("gives gRPC the host of the access request and no URL without it", () => {
    expect(
      row(
        endpointRows({
          cluster,
          spec: {},
          optimizes: [],
          accessOutputs: { grpcHost: "grpc.orders.example.com:443" },
        }),
        "grpc",
      )?.url,
    ).toBe("grpc.orders.example.com:443");
    expect(
      row(endpointRows({ cluster, spec: {}, optimizes: [] }), "grpc")?.url,
    ).toBeUndefined();
  });

  it("gives no URL to an HTTP route of a cluster without an external URL", () => {
    expect(
      row(
        endpointRows({
          cluster: withExternalUrl(undefined),
          spec: {},
          optimizes: [],
        }),
        "operate",
      )?.url,
    ).toBeUndefined();
  });

  it("gives no URL to an HTTP route of a cluster with an empty external URL", () => {
    expect(
      row(
        endpointRows({
          cluster: withExternalUrl(""),
          spec: {},
          optimizes: [],
        }),
        "operate",
      )?.url,
    ).toBeUndefined();
  });

  it("keeps the order of the cluster routes and gives each a unique key", () => {
    const rows = endpointRows({ cluster, spec: {}, optimizes: [] });
    expect(rows.map((r) => r.name)).toEqual(ENDPOINTS.map((e) => e.name));
    expect(new Set(rows.map((r) => r.key)).size).toBe(rows.length);
  });

  it("adds the routes of the Optimize of the cluster after the cluster routes", () => {
    const rows = endpointRows({
      cluster,
      spec: {},
      optimizes: [optimize("orders", "https://optimize.example.com", "/")],
    });
    expect(rows.at(-1)).toMatchObject({
      owner: "CamundaOptimize",
      name: "optimize",
      url: "https://optimize.example.com/",
      service: "orders-optimize",
      port: 8090,
      servedBy: { label: "Optimize", embedded: false },
    });
  });

  it("gives an Optimize URL with a path once", () => {
    const rows = endpointRows({
      cluster,
      spec: {},
      optimizes: [
        optimize("orders", "https://x.example.com/optimize", "/optimize"),
      ],
    });
    expect(rows.at(-1)?.url).toBe("https://x.example.com/optimize");
  });

  it("gives no URL to an Optimize whose external URL does not parse", () => {
    const rows = endpointRows({
      cluster,
      spec: {},
      optimizes: [optimize("orders", "not a url", "/")],
    });
    expect(rows.at(-1)?.url).toBeUndefined();
  });

  it("leaves out the Optimize of another cluster", () => {
    const rows = endpointRows({
      cluster,
      spec: {},
      optimizes: [optimize("payments", "https://p.example.com", "/")],
    });
    expect(rows.every((r) => r.owner === "CamundaCluster")).toBe(true);
  });

  it("shows an unknown route without Served by", () => {
    const rows = endpointRows({
      cluster: withEndpoint({
        name: "future",
        service: "s",
        port: 1,
        protocol: "HTTP",
        pathPrefix: "/f",
      }),
      spec: {},
      optimizes: [],
    });
    expect(row(rows, "future")).toMatchObject({
      url: "https://orders.example.com/f",
    });
    expect(row(rows, "future")?.servedBy).toBeUndefined();
  });

  it("gives no rows to a cluster without endpoints", () => {
    expect(
      endpointRows({ cluster: withoutStatus(), spec: {}, optimizes: [] }),
    ).toEqual([]);
  });
});
