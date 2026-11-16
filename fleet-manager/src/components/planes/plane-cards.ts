// This module runs in the browser and on the server.
import {
  type CardNumber,
  type ComponentCardModel,
  highestUse,
  podsReady,
} from "@/components/cluster/cards";
import type { CamundaPlatformConfig } from "@/generated/crds/camundaplatformconfig";
import {
  conditionsAt,
  MANAGEMENT_CLUSTER_PLACEMENT,
  type PlaneComponent,
} from "@/lib/conditions";
import type { PlaneField } from "@/lib/fields";
import type { Resource } from "@/lib/k8s/kinds";
import { COMPONENT_LABEL, PLANE_COMPONENTS } from "@/lib/labels";
import type { PodUsage, Unavailable } from "@/lib/observe/types";
import {
  IDENTITY_MODE_LABEL,
  type IdentityModeId,
  planeIssuer,
} from "@/lib/planes/identity";

/** PlaneCardId names one component card of a management plane. */
export type PlaneCardId =
  | "keycloak"
  | "identityProvider"
  | "identity"
  | "console"
  | "webModelerRestapi"
  | "webModelerWebsockets";

/**
 * PLANE_CARD_CONDITION is the place of the condition of each card in
 * MANAGEMENT_CLUSTER_PLACEMENT. The identity provider runs outside the
 * plane and has none.
 */
export const PLANE_CARD_CONDITION: Readonly<
  Record<PlaneCardId, PlaneComponent | undefined>
> = {
  keycloak: "keycloak",
  identityProvider: undefined,
  identity: "identity",
  console: "console",
  webModelerRestapi: "webModeler",
  webModelerWebsockets: "webModeler",
};

const LABEL: Record<PlaneCardId, string> = {
  keycloak: "Keycloak",
  identityProvider: "Identity provider",
  identity: "Management Identity",
  console: "Console",
  webModelerRestapi: "Web Modeler REST API",
  webModelerWebsockets: "Web Modeler websockets",
};

const COMPONENT: Record<
  Exclude<PlaneCardId, "identityProvider">,
  string
> = PLANE_COMPONENTS;

/**
 * PlaneCardInput is what the cards are made from. config is the platform
 * config that the plane names, for the OIDC issuer. An undefined usage is
 * not loaded yet. unread holds the kinds that the page could not read or
 * that are out of date: CamundaManagementCluster, Pod, and
 * CamundaPlatformConfig change what a card says.
 */
export type PlaneCardInput = {
  plane: Resource<"CamundaManagementCluster">;
  pods: readonly Resource<"Pod">[];
  config?: Resource<"CamundaPlatformConfig">;
  usage?: PodUsage[] | Unavailable;
  unread?: readonly string[];
};

type Condition = NonNullable<ComponentCardModel["condition"]>;

/**
 * planeCards returns the cards of the components that run, in this order:
 * Keycloak (keycloak mode), the identity provider (externalKeycloak or oidc
 * mode), Management Identity, Console, Web Modeler REST API, Web Modeler
 * websockets. A card with pods shows the pods ready and the highest use. The
 * identity provider card shows the mode and the issuer URL, counts no pods,
 * and has no condition. While the plane is unread, every other card is
 * unknown. While the pods are unread, a card says so in place of its pods.
 * While the platform config is unread, an OIDC issuer says so.
 */
export function planeCards(
  input: PlaneCardInput,
): ComponentCardModel<PlaneCardId>[] {
  const { plane, unread = [] } = input;
  const mode = modeOf(plane);
  const ids: PlaneCardId[] = [
    ...(mode === "keycloak" ? (["keycloak"] as const) : []),
    ...(mode === "externalKeycloak" || mode === "oidc"
      ? (["identityProvider"] as const)
      : []),
    "identity",
    ...(plane.spec.console ? (["console"] as const) : []),
    ...(plane.spec.webModeler
      ? (["webModelerRestapi", "webModelerWebsockets"] as const)
      : []),
  ];
  const sorted = [...input.pods].sort((a, b) =>
    (a.metadata?.name ?? "").localeCompare(b.metadata?.name ?? ""),
  );
  const podsUnread = unread.includes("Pod");
  const planeUnread = unread.includes("CamundaManagementCluster");
  return ids.map((id): ComponentCardModel<PlaneCardId> => {
    if (id === "identityProvider") {
      return {
        id,
        label: LABEL[id],
        used: true,
        embedded: [],
        pods: [],
        numbers: [
          { label: "Mode", value: mode ? IDENTITY_MODE_LABEL[mode] : "-" },
          issuerOf(plane, mode, input.config, unread),
        ],
      };
    }
    const names = sorted
      .filter((p) => p.metadata?.labels?.[COMPONENT_LABEL] === COMPONENT[id])
      .map((p) => p.metadata?.name ?? "");
    const pods = sorted.filter((p) => names.includes(p.metadata?.name ?? ""));
    const condition = conditionsAt(
      MANAGEMENT_CLUSTER_PLACEMENT,
      plane.status?.conditions as Condition[] | undefined,
      PLANE_CARD_CONDITION[id]!,
    )[0];
    return {
      id,
      label: LABEL[id],
      used: true,
      ...(condition && { condition }),
      ...(planeUnread && { unknown: true as const }),
      embedded: [],
      pods: names,
      numbers: podsUnread
        ? [
            {
              label: "Pods ready",
              value: "Could not read Pod",
              severity: "Warning",
            },
            { label: "Highest use", value: "Not known" },
          ]
        : [podsReady(pods), highestUse(input.usage, names)],
    };
  });
}

function modeOf(
  plane: Resource<"CamundaManagementCluster">,
): IdentityModeId | undefined {
  const provider = plane.spec.identityProvider ?? {};
  if (provider.keycloak) return "keycloak";
  if (provider.externalKeycloak) return "externalKeycloak";
  if (provider.oidc) return "oidc";
  return undefined;
}

function issuerOf(
  plane: Resource<"CamundaManagementCluster">,
  mode: IdentityModeId | undefined,
  config: Resource<"CamundaPlatformConfig"> | undefined,
  unread: readonly string[],
): CardNumber {
  // Only the OIDC issuer comes from the platform config.
  if (mode === "oidc" && unread.includes("CamundaPlatformConfig")) {
    return {
      label: "Issuer",
      value: "Could not read CamundaPlatformConfig",
      severity: "Warning",
    };
  }
  const external = plane.spec.identityProvider?.externalKeycloak;
  const issuer = planeIssuer(
    mode,
    { url: external?.url ?? "", realm: external?.realm ?? "", adminSecret: "" },
    config as unknown as CamundaPlatformConfig | undefined,
  );
  return {
    label: "Issuer",
    value: issuer.kind === "known" ? issuer.url : "Unknown",
  };
}

/**
 * planeCardHref is the URL of the Overview tab with the detail of id open,
 * or with the cards for null. The identity provider has no detail, so it
 * gets no URL.
 */
export function planeCardHref(id: PlaneCardId | null): string | undefined {
  if (id === "identityProvider") return undefined;
  return id === null
    ? "?tab=overview"
    : `?${new URLSearchParams({ tab: "overview", component: id })}`;
}

/**
 * planeCardOf returns the card that the component query parameter names
 * when it is one of cards and has a detail, else undefined.
 */
export function planeCardOf(
  value: string | null,
  cards: readonly ComponentCardModel<PlaneCardId>[],
): Exclude<PlaneCardId, "identityProvider"> | undefined {
  const card = cards.find((c) => c.id === value);
  return card && card.id !== "identityProvider" ? card.id : undefined;
}

/**
 * planeCardSelector returns the pod selector of the card id: podSelector,
 * the selector of the pods of the plane, with the component value of id.
 */
export function planeCardSelector(
  podSelector: string,
  id: Exclude<PlaneCardId, "identityProvider">,
): string {
  return `${podSelector},${COMPONENT_LABEL}=${COMPONENT[id]}`;
}

const WEB_MODELER_FIELDS = [
  "webModeler",
  "webModeler.mail",
  "versions.webModeler",
  "url.webModeler",
  "url.webModelerWebsockets",
];
const CARD_FIELDS: Record<PlaneCardId, (key: string) => boolean> = {
  keycloak: (key) => ["versions.keycloak", "url.keycloak"].includes(key),
  identityProvider: () => false,
  identity: (key) =>
    key === "identityProvider" ||
    key.startsWith("admin.") ||
    key === "versions.identity" ||
    key === "url.identity",
  console: (key) =>
    ["console", "versions.console", "url.console"].includes(key),
  webModelerRestapi: (key) => WEB_MODELER_FIELDS.includes(key),
  webModelerWebsockets: (key) => WEB_MODELER_FIELDS.includes(key),
};

/**
 * planeCardFields returns the fields of planeFields that the Settings tab
 * of the card id shows, in the order of fields.
 */
export function planeCardFields(
  id: PlaneCardId,
  fields: readonly PlaneField[],
): PlaneField[] {
  return fields.filter((f) => CARD_FIELDS[id](f.key));
}
