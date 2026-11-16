import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { planeEndpointRows } from "./plane-endpoints";

const route = (name: string, pathPrefix: unknown, service: unknown) => ({
  name,
  pathPrefix,
  port: 80,
  protocol: "HTTP",
  service,
});

function planeOf(
  spec: Record<string, unknown>,
  endpoints: unknown,
): Resource<"CamundaManagementCluster"> {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace: "platform", name: "production" },
    spec,
    status: { endpoints },
  } as unknown as Resource<"CamundaManagementCluster">;
}

const host = "https://camunda.example.com";
const keycloakSpec = {
  identityProvider: { keycloak: { externalUrl: `${host}/auth` } },
  identity: { externalUrl: `${host}/identity` },
  console: { externalUrl: `${host}/console` },
  webModeler: {
    externalUrl: `${host}/modeler`,
    websocketsExternalUrl: `${host}/modeler-ws`,
  },
};

describe("planeEndpointRows", () => {
  it("gives each route of status.endpoints with the URL of its app and the path of the route", () => {
    const plane = planeOf(keycloakSpec, [
      route("console", "/console", "production-console"),
      route("identity", "/identity", "production-identity"),
      route("keycloak", "/auth", "production-keycloak"),
      route("web-modeler", "/modeler", "production-web-modeler"),
      route(
        "web-modeler-websockets",
        "/modeler-ws",
        "production-web-modeler-websockets",
      ),
    ]);
    const row = (
      name: string,
      path: string,
      service: string,
      label: string,
    ) => ({
      key: `CamundaManagementCluster/${name}`,
      owner: "CamundaManagementCluster",
      name,
      path,
      url: `${host}${path}`,
      service,
      port: 80,
      protocol: "HTTP",
      servedBy: { label, embedded: false },
    });
    expect(planeEndpointRows(plane)).toEqual([
      row("console", "/console", "production-console", "Console"),
      row(
        "identity",
        "/identity",
        "production-identity",
        "Management Identity",
      ),
      row("keycloak", "/auth", "production-keycloak", "Keycloak"),
      row("web-modeler", "/modeler", "production-web-modeler", "Web Modeler"),
      row(
        "web-modeler-websockets",
        "/modeler-ws",
        "production-web-modeler-websockets",
        "Web Modeler websockets",
      ),
    ]);
  });

  it("drops the trailing slash of an app URL", () => {
    const plane = planeOf({ identity: { externalUrl: `${host}/identity/` } }, [
      route("identity", "/identity", "production-identity"),
    ]);
    expect(planeEndpointRows(plane)[0].url).toBe(`${host}/identity`);
  });

  it("gives a route of an app on its own host the URL of that host", () => {
    const plane = planeOf(
      { console: { externalUrl: "https://console.example.com" } },
      [route("console", "/", "production-console")],
    );
    expect(planeEndpointRows(plane)[0].url).toBe(
      "https://console.example.com/",
    );
  });

  it("gives an unknown route no URL and no servedBy", () => {
    const [row] = planeEndpointRows(
      planeOf(keycloakSpec, [route("metrics", "/metrics", "production-x")]),
    );
    expect(row.url).toBeUndefined();
    expect(row.servedBy).toBeUndefined();
    expect(row.urlInvalid).toBeUndefined();
  });

  it("gives no URL when the app has no URL on the spec", () => {
    const [row] = planeEndpointRows(
      planeOf({}, [route("console", "/console", "production-console")]),
    );
    expect(row.url).toBeUndefined();
    expect(row.urlInvalid).toBeUndefined();
    expect(row.servedBy).toEqual({ label: "Console", embedded: false });
  });

  it("marks the URL as not valid when the app URL does not parse", () => {
    const [row] = planeEndpointRows(
      planeOf({ console: { externalUrl: "console" } }, [
        route("console", "/console", "production-console"),
      ]),
    );
    expect(row.url).toBeUndefined();
    expect(row.urlInvalid).toBe(true);
  });

  it("gives no rows without a plane or without status.endpoints", () => {
    expect(planeEndpointRows(undefined)).toEqual([]);
    expect(planeEndpointRows(planeOf(keycloakSpec, undefined))).toEqual([]);
    expect(planeEndpointRows(planeOf(keycloakSpec, "routes"))).toEqual([]);
  });

  it("does not throw on an entry of the wrong type", () => {
    const rows = planeEndpointRows(
      planeOf(keycloakSpec, [
        null,
        { name: 4, pathPrefix: 5, service: null, port: "80" },
      ]),
    );
    expect(rows).toEqual([
      {
        key: "CamundaManagementCluster/1",
        owner: "CamundaManagementCluster",
        name: "",
        service: "",
        port: undefined,
        protocol: "HTTP",
      },
    ]);
  });
});
