import { describe, expect, it } from "vitest";
import type { PluginRegistration } from "@/generated/crds/pluginregistration";
import type { PluginSection } from "@/generated/crds/pluginsection";
import type { ClusterClaimObject } from "./draft";
import { createdObjects } from "./review";

const section = (
  name: string,
  label: string,
  selection: "One" | "Many",
): PluginSection =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginSection",
    metadata: { name },
    spec: { appliesTo: ["CamundaClusterClaim"], label, selection },
  }) as PluginSection;

const registration = (name: string, label?: string): PluginRegistration =>
  ({
    apiVersion: "plugins.camunda.io/v1alpha1",
    kind: "PluginRegistration",
    metadata: { name },
    spec: {
      sections: [],
      mode: "Optional",
      deletionPolicies: ["Delete"],
      serviceAccount: { name, namespace: "plugins" },
      ...(label && { label }),
    },
  }) as PluginRegistration;

const sections = [
  section("access", "Access", "One"),
  section("connectors-permissions", "Connector permissions", "Many"),
];
const registrations = [
  registration("gateway", "Gateway"),
  registration("sqs-role"),
];

const claim = (
  spec: Partial<ClusterClaimObject["spec"]>,
): ClusterClaimObject => ({
  apiVersion: "fleet.camunda.io/v1alpha1",
  kind: "CamundaClusterClaim",
  metadata: { name: "orders", namespace: "team-new" },
  spec: { presetRef: "small", ...spec },
});

describe("createdObjects", () => {
  it("lists the namespace, the claim, each request, the cluster, and the attachments", () => {
    expect(
      createdObjects({
        claim: claim({
          plugins: [
            { section: "access", registration: "gateway" },
            { section: "connectors-permissions", registration: "sqs-role" },
          ],
          attachments: {
            optimize: { managementAuthConfigRef: "central" },
            backupSchedule: { schedule: "0 2 * * *" },
          },
        }),
        newNamespace: true,
        sections,
        registrations,
      }),
    ).toEqual([
      "Creates the namespace team-new.",
      "Creates the claim orders.",
      "Requests Access: Gateway as orders-access.",
      "Requests Connector permissions: sqs-role as orders-connectors-permissions-sqs-role.",
      "Creates the CamundaCluster orders.",
      "Creates the CamundaOptimize orders-optimize.",
      "Creates the BackupSchedule orders-backups.",
    ]);
  });

  it("lists no namespace, Optimize, or schedule for an existing namespace without attachments", () => {
    expect(
      createdObjects({
        claim: claim({}),
        newNamespace: false,
        sections,
        registrations,
      }),
    ).toEqual([
      "Creates the claim orders.",
      "Creates the CamundaCluster orders.",
    ]);
  });
});
