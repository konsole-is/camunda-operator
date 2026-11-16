// This module runs in the browser and on the server.
import type { EndpointRow } from "@/components/cluster/endpoints";
import type { Resource } from "@/lib/k8s/kinds";
import {
  PLANE_URL_LABEL,
  type PlaneUrlId,
  planeExternalUrls,
} from "@/lib/planes/urls";

// The route names of status.endpoints of a plane, with the URL of the app
// that serves each.
const ROUTE_URL: Record<string, PlaneUrlId> = {
  keycloak: "keycloak",
  identity: "identity",
  console: "console",
  "web-modeler": "webModeler",
  "web-modeler-websockets": "webModelerWebsockets",
};

/**
 * planeEndpointRows returns the routes of status.endpoints of plane in
 * their order. The URL of a route is the origin of the URL of its app on
 * the spec with the pathPrefix of the route. servedBy names the app. A
 * route name that no app has gets no URL and no servedBy. urlInvalid is
 * true when the URL of the app does not parse. An entry that is not an
 * object is left out.
 */
export function planeEndpointRows(
  plane: Resource<"CamundaManagementCluster"> | undefined,
): EndpointRow[] {
  const endpoints: unknown = plane?.status?.endpoints;
  if (!Array.isArray(endpoints)) return [];
  const urls = planeExternalUrls(plane);
  return endpoints.flatMap((entry: unknown, index): EndpointRow[] => {
    if (typeof entry !== "object" || entry === null) return [];
    const e = entry as Record<string, unknown>;
    const name = typeof e.name === "string" ? e.name : "";
    const path = typeof e.pathPrefix === "string" ? e.pathPrefix : undefined;
    const app = Object.hasOwn(ROUTE_URL, name) ? ROUTE_URL[name] : undefined;
    const appUrl = app === undefined ? undefined : urls[app];
    const origin =
      appUrl !== undefined && URL.canParse(appUrl)
        ? new URL(appUrl).origin
        : undefined;
    return [
      {
        key: `CamundaManagementCluster/${name || index}`,
        owner: "CamundaManagementCluster",
        name,
        ...(path !== undefined && { path }),
        ...(origin !== undefined && origin !== "null"
          ? { url: `${origin}${path ?? ""}` }
          : appUrl !== undefined && { urlInvalid: true as const }),
        service: typeof e.service === "string" ? e.service : "",
        port: typeof e.port === "number" ? e.port : undefined,
        protocol: e.protocol === "GRPC" ? "GRPC" : "HTTP",
        ...(app !== undefined && {
          servedBy: { label: PLANE_URL_LABEL[app], embedded: false },
        }),
      },
    ];
  });
}
