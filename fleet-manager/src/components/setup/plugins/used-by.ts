import type { ConditionRow } from "@/components/fleet/conditions-table";
import type { Grant } from "@/generated/crds/grant";
import type { PluginRequest } from "@/generated/crds/pluginrequest";
import { REQUEST_LABEL } from "@/lib/labels";
import { byCodeUnit } from "@/lib/policy/json";
import { environmentOf } from "@/lib/policy/environment";
import { type GrantView, grantView } from "./grants";
import { NO_ENVIRONMENT, type SourceReads, unreadKinds } from "./groups";
import type { RegistrationSources } from "./sources";
import {
  type Invalid,
  conditionsOf,
  isRecord,
  isText,
  itemsOf,
} from "./values";

/** ResourceRef is one resource that a plugin reports for a request. */
export type ResourceRef = NonNullable<
  NonNullable<PluginRequest["status"]>["resources"]
>[number];

/** SummaryItem is one fact that a plugin reports about a request. */
export type SummaryItem = NonNullable<
  NonNullable<PluginRequest["status"]>["summary"]
>[number];

/** UsedByRow is one request of a registration. */
export type UsedByRow = {
  namespace: string;
  name: string;
  /** claim and target are unset, with refError, when the request names no valid claim or target. */
  claim?: {
    kind: "CamundaClusterClaim" | "CamundaManagementClusterClaim";
    name: string;
  };
  target?: { kind: string; name: string };
  refError?: string;
  /**
   * environment is the display name, else the name, of the Environment of
   * the namespace, or "No environment". It is unset while Namespace or
   * Environment is not read.
   */
  environment?: string;
  /** healthUnknown is true while PluginRequest is not read. outputsReady and ready are then unset. */
  healthUnknown?: true;
  outputsReady?: ConditionRow;
  ready?: ConditionRow;
  /** error says why the conditions cannot be read. */
  error?: string;
  /**
   * grantsNotReady names the Grants of the request whose Ready is False,
   * while the Ready of the request has the reason GrantRejected. It is unset
   * while Grant is not read.
   */
  grantsNotReady?: string[];
  /** headline is the first item of status.summary. */
  headline?: SummaryItem | Invalid;
  resources: (ResourceRef | Invalid)[];
  /** grants is unset while Grant is not read. */
  grants?: GrantView[];
  /** deletionPolicy is unset when the request has a value that is not Delete or Orphan. */
  deletionPolicy?: "Delete" | "Orphan";
};

/**
 * usedByRows gives one row for each request of the registration name, by
 * namespace and name. The Grants of a request are the Grants of grants in
 * its namespace whose REQUEST_LABEL is its name. A value of the status that
 * is not valid is an error of its row.
 */
export function usedByRows(
  name: string,
  sources: RegistrationSources & SourceReads,
  grants: readonly Grant[],
): UsedByRow[] {
  const unread = unreadKinds(sources);
  const healthUnknown = unread.includes("PluginRequest");
  const grantsUnread = unread.includes("Grant");
  const environmentsUnread =
    unread.includes("Namespace") || unread.includes("Environment");
  return sources.requests
    .filter((r) => r.spec.registration === name)
    .sort(
      (a, b) =>
        byCodeUnit(a.metadata.namespace ?? "", b.metadata.namespace ?? "") ||
        byCodeUnit(a.metadata.name ?? "", b.metadata.name ?? ""),
    )
    .map((request) => {
      const namespace = request.metadata.namespace ?? "";
      const requestName = request.metadata.name ?? "";
      const read = conditionsOf(request.status?.conditions);
      const conditions = "conditions" in read ? read.conditions : [];
      const ready = conditions.find((c) => c.type === "Ready");
      const outputsReady = conditions.find((c) => c.type === "OutputsReady");
      const own = grants
        .filter(
          (g) =>
            g.metadata.namespace === namespace &&
            g.metadata.labels?.[REQUEST_LABEL] === requestName,
        )
        .map(grantView);
      const [headline] = itemsOf(
        request.status?.summary,
        isSummaryItem,
        "Summary item",
        "summary items",
      );
      const claimRef: unknown = request.spec.claimRef;
      const target: unknown = request.spec.target;
      return {
        namespace,
        name: requestName,
        ...(isRef(claimRef) && isRef(target)
          ? {
              claim: claimRef as NonNullable<UsedByRow["claim"]>,
              target: { kind: target.kind, name: target.name },
            }
          : {
              refError: "The claim or the target of the request is not valid.",
            }),
        ...(!environmentsUnread && {
          environment: environmentName(namespace, sources),
        }),
        ...(healthUnknown && { healthUnknown: true as const }),
        ...(!healthUnknown && outputsReady && { outputsReady }),
        ...(!healthUnknown && ready && { ready }),
        ...("error" in read && { error: read.error }),
        ...(!grantsUnread && {
          grantsNotReady:
            ready?.reason === "GrantRejected"
              ? own
                  .filter((g) => g.ready?.status === "False")
                  .map((g) => g.name)
              : [],
          grants: own,
        }),
        ...(headline && { headline }),
        resources: itemsOf(
          request.status?.resources,
          isResource,
          "Resource",
          "resources",
        ),
        ...((request.spec.deletionPolicy === "Delete" ||
          request.spec.deletionPolicy === "Orphan") && {
          deletionPolicy: request.spec.deletionPolicy,
        }),
      };
    });
}

function environmentName(
  namespace: string,
  sources: RegistrationSources,
): string {
  const object = sources.namespaces.find((n) => n.metadata?.name === namespace);
  const environment = object
    ? environmentOf(object, sources.environments).environment
    : undefined;
  if (!environment) return NO_ENVIRONMENT;
  const label: unknown = environment.spec.displayName;
  return isText(label) && label
    ? (label as string)
    : (environment.metadata.name ?? "");
}

const SEVERITIES: readonly unknown[] = [undefined, "Info", "Warning", "Error"];

function isRef(item: unknown): item is { kind: string; name: string } {
  return isRecord(item) && isText(item.kind) && isText(item.name);
}

function isSummaryItem(item: unknown): item is SummaryItem {
  return (
    isRecord(item) &&
    isText(item.label) &&
    isText(item.value) &&
    SEVERITIES.includes(item.severity)
  );
}

function isResource(item: unknown): item is ResourceRef {
  return (
    isRecord(item) &&
    isText(item.kind) &&
    isText(item.name) &&
    (item.onOrphan === "Kept" || item.onOrphan === "Deleted") &&
    isText(item.id, true) &&
    isText(item.location, true) &&
    isText(item.role, true)
  );
}
