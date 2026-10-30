import "server-only";
import { kubeConfig, requestJSON } from "@/lib/k8s/client";
import { appNamespace } from "./keys";

/** PermissionRule is one rule of the app in its namespace. An empty list means any. */
export type PermissionRule = {
  apiGroups: string[];
  resources: string[];
  resourceNames: string[];
  verbs: string[];
};

/** AppPermissions are the rules of the app. incomplete means that the API server could not list every rule. */
export type AppPermissions = { rules: PermissionRule[]; incomplete: boolean };

type RulesReview = {
  status?: {
    resourceRules?: {
      apiGroups?: string[];
      resources?: string[];
      resourceNames?: string[];
      verbs?: string[];
    }[];
    incomplete?: boolean;
  };
};

const DISCOVERY_GROUPS = ["authorization.k8s.io", "authentication.k8s.io"];

/**
 * appPermissions returns the resource rules that the app has in
 * FLEET_NAMESPACE, once each, sorted by API group, then by resource. It leaves out the
 * rules that every user has to review itself (the selfsubject resources) and
 * the non-resource rules. It rejects when the API server refuses the review.
 */
export async function appPermissions(): Promise<AppPermissions> {
  const review = await requestJSON<RulesReview>(kubeConfig(), {
    method: "POST",
    path: "/apis/authorization.k8s.io/v1/selfsubjectrulesreviews",
    body: {
      apiVersion: "authorization.k8s.io/v1",
      kind: "SelfSubjectRulesReview",
      spec: { namespace: appNamespace() },
    },
    timeoutMs: 10_000,
  });
  const rules = (review.status?.resourceRules ?? [])
    .map((rule) => ({
      apiGroups: rule.apiGroups ?? [],
      resources: rule.resources ?? [],
      resourceNames: rule.resourceNames ?? [],
      verbs: rule.verbs ?? [],
    }))
    .filter((rule) => !isDiscovery(rule))
    // Two roles can grant the same rule.
    .filter(
      (rule, index, all) =>
        all.findIndex(
          (other) => JSON.stringify(other) === JSON.stringify(rule),
        ) === index,
    )
    .sort(
      (a, b) =>
        a.apiGroups.join(",").localeCompare(b.apiGroups.join(",")) ||
        a.resources.join(",").localeCompare(b.resources.join(",")),
    );
  return { rules, incomplete: review.status?.incomplete ?? false };
}

function isDiscovery(rule: PermissionRule): boolean {
  return (
    rule.apiGroups.length > 0 &&
    rule.apiGroups.every((g) => DISCOVERY_GROUPS.includes(g)) &&
    rule.resources.length > 0 &&
    rule.resources.every((r) => r.startsWith("selfsubject"))
  );
}
