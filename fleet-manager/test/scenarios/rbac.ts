// The page scenarios run the app as a user that holds only the RBAC of the
// chart. CI has no Helm, so this module reads the rules from the templates.
import fs from "node:fs/promises";
import path from "node:path";
import type { KubeConfig } from "@kubernetes/client-node";
import { parse } from "yaml";
import { KubeError, kubeRequest } from "./kube";

/**
 * CHART_DIR holds the chart templates of the fleet manager. It resolves
 * from the working directory, which is fleet-manager for Vitest and
 * Playwright.
 */
export const CHART_DIR = path.resolve("deploy/chart");

/** PolicyRule is one rule of a Role or a ClusterRole. */
export interface PolicyRule {
  apiGroups?: string[];
  resources?: string[];
  resourceNames?: string[];
  verbs: string[];
}

/**
 * chartRules returns the rules of the Role or ClusterRole template text. It
 * throws when the rules block holds a template, so that a test never binds
 * other rules than the chart, and when the text has no rules block. file
 * names the template in the error.
 */
export function chartRules(text: string, file: string): PolicyRule[] {
  const lines = text.split("\n");
  const start = lines.findIndex((line) => line.startsWith("rules:"));
  if (start < 0) throw new Error(`The template ${file} has no rules block`);
  const rules = lines.slice(start);
  // The templates close their outer if after the rules.
  while (
    rules.length > 0 &&
    /^(\{\{-?\s*end\s*-?\}\})?\s*$/.test(rules.at(-1)!)
  ) {
    rules.pop();
  }
  const block = rules.join("\n");
  if (block.includes("{{")) {
    throw new Error(
      `The rules of ${file} hold a template. Read them with helm template instead`,
    );
  }
  return (parse(block) as { rules: PolicyRule[] }).rules;
}

const RBAC = "/apis/rbac.authorization.k8s.io/v1";

/**
 * bindChartRBAC gives user the rules of clusterrole.yaml in every namespace
 * and the rules of role.yaml in namespace, through roles and bindings named
 * fleet-scenarios-<user>, which must be a valid object name. admin is a
 * kubeconfig that may create them. A second call for the same user keeps
 * the objects of the first.
 */
export async function bindChartRBAC(
  admin: KubeConfig,
  user: string,
  namespace: string,
  chartDir: string = CHART_DIR,
): Promise<void> {
  // The prefix keeps the name apart from the built-in roles of the API
  // server, such as edit and view.
  const name = `fleet-scenarios-${user}`;
  const read = async (file: string) => {
    const full = path.join(chartDir, file);
    return chartRules(await fs.readFile(full, "utf8"), full);
  };
  const subjects = [
    { apiGroup: "rbac.authorization.k8s.io", kind: "User", name: user },
  ];
  const binding = (kind: "ClusterRole" | "Role") => ({
    metadata: { name },
    roleRef: { apiGroup: "rbac.authorization.k8s.io", kind, name },
    subjects,
  });
  const scoped = `${RBAC}/namespaces/${namespace}`;

  await create(admin, `${RBAC}/clusterroles`, {
    kind: "ClusterRole",
    metadata: { name },
    rules: await read("clusterrole.yaml"),
  });
  await create(admin, `${RBAC}/clusterrolebindings`, {
    kind: "ClusterRoleBinding",
    ...binding("ClusterRole"),
  });
  await create(admin, `${scoped}/roles`, {
    kind: "Role",
    metadata: { name },
    rules: await read("role.yaml"),
  });
  await create(admin, `${scoped}/rolebindings`, {
    kind: "RoleBinding",
    ...binding("Role"),
  });
}

async function create(
  kc: KubeConfig,
  collection: string,
  body: { kind: string; [key: string]: unknown },
): Promise<void> {
  try {
    await kubeRequest(kc, "POST", collection, {
      apiVersion: "rbac.authorization.k8s.io/v1",
      ...body,
    });
  } catch (error) {
    if (error instanceof KubeError && error.status === 409) return;
    throw error;
  }
}
